#!/usr/bin/env python3
"""Static validation for the FPBCraft FTB Quests pack.

The validator intentionally uses lightweight text parsing instead of a full SNBT
parser so it can run in CI without additional dependencies.
"""

from __future__ import annotations

import argparse
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

HEX_ID = r"[0-9A-F]{16}"
ID_RE = re.compile(rf'\bid:\s*"({HEX_ID})"')
CHAPTER_ID_RE = re.compile(rf'^\tid:\s*"({HEX_ID})"\s*$', re.MULTILINE)
GROUP_RE = re.compile(rf'^\\tgroup:\\s*"({HEX_ID}|)"\\s*
FILENAME_RE = re.compile(r'^\tfilename:\s*"([^"]+)"\s*$', re.MULTILINE)
DEPENDENCIES_RE = re.compile(r'dependencies:\s*\[([^\]]*)\]')
QUOTED_ID_RE = re.compile(rf'"({HEX_ID})"')
GROUP_ID_RE = re.compile(rf'\{{\s*id:\s*"({HEX_ID})"\s*\}}')
LOCALE_KEY_RE = re.compile(
    rf'^\t((?:chapter|quest|chapter_group)\.({HEX_ID})\.(?:title|quest_desc)|'
    rf'chapter_group\.({HEX_ID})\.title):',
    re.MULTILINE,
)
COORD_RE = re.compile(r'^\s*([xy]):\s*(-?[0-9.]+)d\s*$', re.MULTILINE)
TASK_TYPE_RE = re.compile(r'^\s*type:\s*"([^"]+)"\s*$', re.MULTILINE)


class Report:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []
        self.info: list[str] = []

    def error(self, message: str) -> None:
        self.errors.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)

    def note(self, message: str) -> None:
        self.info.append(message)


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def extract_quest_blocks(text: str) -> list[str]:
    marker = "quests: ["
    start = text.find(marker)
    if start < 0:
        return []

    blocks: list[str] = []
    depth = 0
    block_start: int | None = None
    in_string = False
    escaped = False

    for index in range(start + len(marker), len(text)):
        char = text[index]

        if in_string:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                in_string = False
            continue

        if char == '"':
            in_string = True
        elif char == "{":
            if depth == 0:
                block_start = index
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0 and block_start is not None:
                blocks.append(text[block_start : index + 1])
                block_start = None
        elif char == "]" and depth == 0:
            break

    return blocks


def first_object_id(block: str) -> str | None:
    match = ID_RE.search(block)
    return match.group(1) if match else None


def direct_field(block: str, field: str) -> str | None:
    # Quest-level fields are indented one level inside the quest block. This
    # deliberately avoids matching task/icon fields nested deeper in the block.
    match = re.search(rf'^\s{{1,4}}{re.escape(field)}:\s*"([^"]+)"\s*$',
                      block, re.MULTILINE)
    return match.group(1) if match else None


def has_inline_title(block: str) -> bool:
    return bool(re.search(r'^\s{1,4}title:\s*', block, re.MULTILINE))


def quest_coordinates(block: str) -> tuple[float, float] | None:
    values: dict[str, float] = {}
    for axis, raw in COORD_RE.findall(block):
        values[axis] = float(raw)
    if "x" in values and "y" in values:
        return values["x"], values["y"]
    return None


def detect_cycles(graph: dict[str, set[str]]) -> list[list[str]]:
    cycles: list[list[str]] = []
    visiting: set[str] = set()
    visited: set[str] = set()
    stack: list[str] = []

    def visit(node: str) -> None:
        if node in visited:
            return
        if node in visiting:
            try:
                index = stack.index(node)
            except ValueError:
                index = 0
            cycle = stack[index:] + [node]
            if cycle not in cycles:
                cycles.append(cycle)
            return

        visiting.add(node)
        stack.append(node)
        for dependency in graph.get(node, set()):
            visit(dependency)
        stack.pop()
        visiting.remove(node)
        visited.add(node)

    for node in graph:
        visit(node)
    return cycles


def validate(root: Path) -> Report:
    report = Report()
    quest_root = root / "ftbquests" / "quests"
    chapters_dir = quest_root / "chapters"
    groups_file = quest_root / "chapter_groups.snbt"
    locale_upper = quest_root / "lang" / "en_US.snbt"
    locale_lower = quest_root / "lang" / "en_us.snbt"

    required = [chapters_dir, groups_file, locale_upper, locale_lower]
    for path in required:
        if not path.exists():
            report.error(f"Missing required path: {path}")
    if report.errors:
        return report

    chapter_files = sorted(chapters_dir.glob("*.snbt"))
    if not chapter_files:
        report.error("No chapter files found")
        return report

    groups_text = read(groups_file)
    group_ids = set(GROUP_ID_RE.findall(groups_text))
    if not group_ids:
        report.error("No chapter groups found")

    all_ids: dict[str, list[str]] = defaultdict(list)
    chapter_ids: dict[str, str] = {}
    quest_ids: dict[str, str] = {}
    quest_blocks: dict[str, str] = {}
    quest_optional: dict[str, bool] = {}
    dependencies: list[tuple[str, str, str]] = []
    graph: dict[str, set[str]] = defaultdict(set)
    task_types: Counter[str] = Counter()
    checkmark_quests: list[tuple[str, str]] = []

    for path in chapter_files:
        text = read(path)
        rel = path.relative_to(root).as_posix()

        filename_match = FILENAME_RE.search(text)
        if not filename_match:
            report.error(f"{rel}: missing filename field")
        elif filename_match.group(1) != path.stem:
            report.error(
                f"{rel}: filename field {filename_match.group(1)!r} does not "
                f"match file stem {path.stem!r}"
            )

        chapter_match = CHAPTER_ID_RE.search(text)
        if not chapter_match:
            report.error(f"{rel}: missing top-level chapter id")
            continue
        chapter_id = chapter_match.group(1)
        if chapter_id in chapter_ids:
            report.error(
                f"Duplicate chapter id {chapter_id}: "
                f"{chapter_ids[chapter_id]} and {rel}"
            )
        chapter_ids[chapter_id] = rel

        group_match = GROUP_RE.search(text)
        if not group_match:
            report.error(f"{rel}: missing chapter group field")
        elif group_match.group(1) and group_match.group(1) not in group_ids:
            report.error(
                f"{rel}: references unknown chapter group {group_match.group(1)}"
            )

        for value in ID_RE.findall(text):
            all_ids[value].append(rel)

        coordinate_owners: dict[tuple[float, float], list[str]] = defaultdict(list)
        for block in extract_quest_blocks(text):
            qid = first_object_id(block)
            if not qid:
                report.error(f"{rel}: quest block without a 16-character id")
                continue

            if qid in quest_ids:
                report.error(f"Duplicate quest id {qid}: {quest_ids[qid]} and {rel}")
            quest_ids[qid] = rel
            quest_blocks[qid] = block
            quest_optional[qid] = bool(
                re.search(r'^\s*optional:\s*true\s*$', block, re.MULTILINE)
            )

            coordinates = quest_coordinates(block)
            if coordinates is None:
                report.warn(f"{rel}: quest {qid} has no x/y coordinates")
            else:
                coordinate_owners[coordinates].append(qid)

            task_section = re.search(r'tasks:\s*\[(.*?)\]\s*(?:\n\s*[a-z_]+:|\n\s*\})',
                                     block, re.DOTALL)
            if not task_section:
                report.error(f"{rel}: quest {qid} has no tasks section")
            else:
                types = TASK_TYPE_RE.findall(task_section.group(1))
                if not types:
                    report.error(f"{rel}: quest {qid} has an empty tasks section")
                for task_type in types:
                    task_types[task_type] += 1
                    if task_type == "checkmark":
                        checkmark_quests.append((qid, rel))

            for match in DEPENDENCIES_RE.finditer(block):
                for dep in QUOTED_ID_RE.findall(match.group(1)):
                    dependencies.append((qid, dep, rel))
                    graph[qid].add(dep)

        for coordinates, owners in coordinate_owners.items():
            if len(owners) > 1:
                report.warn(
                    f"{rel}: overlapping quest coordinates {coordinates}: "
                    + ", ".join(owners)
                )

        if "\\n" in text:
            report.warn(f"{rel}: contains a literal \\n sequence; verify formatting")

    duplicates = {key: value for key, value in all_ids.items() if len(value) > 1}
    for value, paths in sorted(duplicates.items()):
        report.error(
            f"Duplicate object id {value}: {', '.join(sorted(set(paths)))}"
        )

    for owner, dep, rel in dependencies:
        if dep not in quest_ids:
            report.error(f"{rel}: quest {owner} depends on missing quest {dep}")

    for cycle in detect_cycles(graph):
        report.error("Quest dependency cycle: " + " -> ".join(cycle))

    for owner, dep, rel in dependencies:
        if quest_optional.get(dep, False) and not quest_optional.get(owner, False):
            report.warn(
                f"{rel}: non-optional quest {owner} directly depends on optional "
                f"quest {dep}; verify that branch cannot block progression"
            )

    upper = read(locale_upper)
    lower = read(locale_lower)
    if upper != lower:
        report.error("en_US.snbt and en_us.snbt are not byte-identical")

    locale_ids: Counter[str] = Counter()
    locale_title_ids: set[str] = set()
    for match in LOCALE_KEY_RE.finditer(lower):
        locale_id = match.group(2) or match.group(3)
        if locale_id:
            locale_ids[locale_id] += 1
            if ".title:" in match.group(0):
                locale_title_ids.add(locale_id)

    for chapter_id, rel in sorted(chapter_ids.items()):
        text = read(root / rel)
        has_inline = bool(re.search(r'^\ttitle:\s*', text, re.MULTILINE))
        if chapter_id not in locale_title_ids and not has_inline:
            report.warn(f"{rel}: chapter {chapter_id} has no localized or inline title")

    for qid, rel in sorted(quest_ids.items()):
        if qid not in locale_title_ids and not has_inline_title(quest_blocks[qid]):
            report.warn(f"{rel}: quest {qid} has no localized or inline title")

    group_title_ids = {
        match.group(1)
        for match in re.finditer(
            rf'^\tchapter_group\.({HEX_ID})\.title:', lower, re.MULTILINE
        )
    }
    for group_id in sorted(group_ids):
        if group_id not in group_title_ids:
            report.warn(f"Chapter group {group_id} has no localized title")

    known_ids = set(chapter_ids) | set(quest_ids) | group_ids
    orphan_locale_ids = sorted(key for key in locale_ids if key not in known_ids)
    if orphan_locale_ids:
        sample = ", ".join(orphan_locale_ids[:20])
        suffix = (
            ""
            if len(orphan_locale_ids) <= 20
            else f" (+{len(orphan_locale_ids) - 20} more)"
        )
        report.warn(
            f"{len(orphan_locale_ids)} localization IDs do not map to a current "
            f"chapter/quest/group: {sample}{suffix}"
        )
        report.note("ORPHAN_LOCALE_IDS=" + ",".join(orphan_locale_ids))

    stale_boss_group = "B055E50000000001"
    for path in root.glob("docs/quest-overhaul/*.md"):
        if stale_boss_group in read(path):
            report.warn(
                f"{path.relative_to(root)} still references stale Bosses group "
                f"{stale_boss_group}; current id is 5055E50000000001"
            )

    report.note(
        f"Scanned {len(chapter_files)} chapters, {len(quest_ids)} quests, "
        f"{sum(task_types.values())} tasks"
    )
    report.note(
        "Task types: "
        + ", ".join(f"{name}={count}" for name, count in sorted(task_types.items()))
    )
    report.note(f"Manual checkmark quests: {len(checkmark_quests)}")
    for qid, rel in checkmark_quests:
        report.note(f"CHECKMARK: {rel}: {qid}")

    return report


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "root",
        nargs="?",
        default=".",
        help="repository root (default: current directory)",
    )
    args = parser.parse_args()

    root = Path(args.root).resolve()
    report = validate(root)

    print(
        f"FTB Quests validation: {len(report.errors)} error(s), "
        f"{len(report.warnings)} warning(s)"
    )

    for message in report.errors:
        print(f"ERROR: {message}")
    for message in report.warnings:
        print(f"WARN: {message}")
    for message in report.info:
        print(f"INFO: {message}")

    return 1 if report.errors else 0


if __name__ == "__main__":
    sys.exit(main())
, re.MULTILINE)
FILENAME_RE = re.compile(r'^\tfilename:\s*"([^"]+)"\s*$', re.MULTILINE)
DEPENDENCIES_RE = re.compile(r'dependencies:\s*\[([^\]]*)\]')
QUOTED_ID_RE = re.compile(rf'"({HEX_ID})"')
GROUP_ID_RE = re.compile(rf'\{{\s*id:\s*"({HEX_ID})"\s*\}}')
LOCALE_KEY_RE = re.compile(
    rf'^\t((?:chapter|quest|chapter_group)\.({HEX_ID})\.(?:title|quest_desc)|'
    rf'chapter_group\.({HEX_ID})\.title):',
    re.MULTILINE,
)
COORD_RE = re.compile(r'^\s*([xy]):\s*(-?[0-9.]+)d\s*$', re.MULTILINE)
TASK_TYPE_RE = re.compile(r'^\s*type:\s*"([^"]+)"\s*$', re.MULTILINE)


class Report:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []
        self.info: list[str] = []

    def error(self, message: str) -> None:
        self.errors.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)

    def note(self, message: str) -> None:
        self.info.append(message)


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def extract_quest_blocks(text: str) -> list[str]:
    marker = "quests: ["
    start = text.find(marker)
    if start < 0:
        return []

    blocks: list[str] = []
    depth = 0
    block_start: int | None = None
    in_string = False
    escaped = False

    for index in range(start + len(marker), len(text)):
        char = text[index]

        if in_string:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                in_string = False
            continue

        if char == '"':
            in_string = True
        elif char == "{":
            if depth == 0:
                block_start = index
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0 and block_start is not None:
                blocks.append(text[block_start : index + 1])
                block_start = None
        elif char == "]" and depth == 0:
            break

    return blocks


def first_object_id(block: str) -> str | None:
    match = ID_RE.search(block)
    return match.group(1) if match else None


def direct_field(block: str, field: str) -> str | None:
    # Quest-level fields are indented one level inside the quest block. This
    # deliberately avoids matching task/icon fields nested deeper in the block.
    match = re.search(rf'^\s{{1,4}}{re.escape(field)}:\s*"([^"]+)"\s*$',
                      block, re.MULTILINE)
    return match.group(1) if match else None


def has_inline_title(block: str) -> bool:
    return bool(re.search(r'^\s{1,4}title:\s*', block, re.MULTILINE))


def quest_coordinates(block: str) -> tuple[float, float] | None:
    values: dict[str, float] = {}
    for axis, raw in COORD_RE.findall(block):
        values[axis] = float(raw)
    if "x" in values and "y" in values:
        return values["x"], values["y"]
    return None


def detect_cycles(graph: dict[str, set[str]]) -> list[list[str]]:
    cycles: list[list[str]] = []
    visiting: set[str] = set()
    visited: set[str] = set()
    stack: list[str] = []

    def visit(node: str) -> None:
        if node in visited:
            return
        if node in visiting:
            try:
                index = stack.index(node)
            except ValueError:
                index = 0
            cycle = stack[index:] + [node]
            if cycle not in cycles:
                cycles.append(cycle)
            return

        visiting.add(node)
        stack.append(node)
        for dependency in graph.get(node, set()):
            visit(dependency)
        stack.pop()
        visiting.remove(node)
        visited.add(node)

    for node in graph:
        visit(node)
    return cycles


def validate(root: Path) -> Report:
    report = Report()
    quest_root = root / "ftbquests" / "quests"
    chapters_dir = quest_root / "chapters"
    groups_file = quest_root / "chapter_groups.snbt"
    locale_upper = quest_root / "lang" / "en_US.snbt"
    locale_lower = quest_root / "lang" / "en_us.snbt"

    required = [chapters_dir, groups_file, locale_upper, locale_lower]
    for path in required:
        if not path.exists():
            report.error(f"Missing required path: {path}")
    if report.errors:
        return report

    chapter_files = sorted(chapters_dir.glob("*.snbt"))
    if not chapter_files:
        report.error("No chapter files found")
        return report

    groups_text = read(groups_file)
    group_ids = set(GROUP_ID_RE.findall(groups_text))
    if not group_ids:
        report.error("No chapter groups found")

    all_ids: dict[str, list[str]] = defaultdict(list)
    chapter_ids: dict[str, str] = {}
    quest_ids: dict[str, str] = {}
    quest_blocks: dict[str, str] = {}
    quest_optional: dict[str, bool] = {}
    dependencies: list[tuple[str, str, str]] = []
    graph: dict[str, set[str]] = defaultdict(set)
    task_types: Counter[str] = Counter()
    checkmark_quests: list[tuple[str, str]] = []

    for path in chapter_files:
        text = read(path)
        rel = path.relative_to(root).as_posix()

        filename_match = FILENAME_RE.search(text)
        if not filename_match:
            report.error(f"{rel}: missing filename field")
        elif filename_match.group(1) != path.stem:
            report.error(
                f"{rel}: filename field {filename_match.group(1)!r} does not "
                f"match file stem {path.stem!r}"
            )

        chapter_match = CHAPTER_ID_RE.search(text)
        if not chapter_match:
            report.error(f"{rel}: missing top-level chapter id")
            continue
        chapter_id = chapter_match.group(1)
        if chapter_id in chapter_ids:
            report.error(
                f"Duplicate chapter id {chapter_id}: "
                f"{chapter_ids[chapter_id]} and {rel}"
            )
        chapter_ids[chapter_id] = rel

        group_match = GROUP_RE.search(text)
        if not group_match:
            report.error(f"{rel}: missing chapter group")
        elif group_match.group(1) not in group_ids:
            report.error(
                f"{rel}: references unknown chapter group {group_match.group(1)}"
            )

        for value in ID_RE.findall(text):
            all_ids[value].append(rel)

        coordinate_owners: dict[tuple[float, float], list[str]] = defaultdict(list)
        for block in extract_quest_blocks(text):
            qid = first_object_id(block)
            if not qid:
                report.error(f"{rel}: quest block without a 16-character id")
                continue

            if qid in quest_ids:
                report.error(f"Duplicate quest id {qid}: {quest_ids[qid]} and {rel}")
            quest_ids[qid] = rel
            quest_blocks[qid] = block
            quest_optional[qid] = bool(
                re.search(r'^\s*optional:\s*true\s*$', block, re.MULTILINE)
            )

            coordinates = quest_coordinates(block)
            if coordinates is None:
                report.warn(f"{rel}: quest {qid} has no x/y coordinates")
            else:
                coordinate_owners[coordinates].append(qid)

            task_section = re.search(r'tasks:\s*\[(.*?)\]\s*(?:\n\s*[a-z_]+:|\n\s*\})',
                                     block, re.DOTALL)
            if not task_section:
                report.error(f"{rel}: quest {qid} has no tasks section")
            else:
                types = TASK_TYPE_RE.findall(task_section.group(1))
                if not types:
                    report.error(f"{rel}: quest {qid} has an empty tasks section")
                for task_type in types:
                    task_types[task_type] += 1
                    if task_type == "checkmark":
                        checkmark_quests.append((qid, rel))

            for match in DEPENDENCIES_RE.finditer(block):
                for dep in QUOTED_ID_RE.findall(match.group(1)):
                    dependencies.append((qid, dep, rel))
                    graph[qid].add(dep)

        for coordinates, owners in coordinate_owners.items():
            if len(owners) > 1:
                report.warn(
                    f"{rel}: overlapping quest coordinates {coordinates}: "
                    + ", ".join(owners)
                )

        if "\\n" in text:
            report.warn(f"{rel}: contains a literal \\n sequence; verify formatting")

    duplicates = {key: value for key, value in all_ids.items() if len(value) > 1}
    for value, paths in sorted(duplicates.items()):
        report.error(
            f"Duplicate object id {value}: {', '.join(sorted(set(paths)))}"
        )

    for owner, dep, rel in dependencies:
        if dep not in quest_ids:
            report.error(f"{rel}: quest {owner} depends on missing quest {dep}")

    for cycle in detect_cycles(graph):
        report.error("Quest dependency cycle: " + " -> ".join(cycle))

    for owner, dep, rel in dependencies:
        if quest_optional.get(dep, False) and not quest_optional.get(owner, False):
            report.warn(
                f"{rel}: non-optional quest {owner} directly depends on optional "
                f"quest {dep}; verify that branch cannot block progression"
            )

    upper = read(locale_upper)
    lower = read(locale_lower)
    if upper != lower:
        report.error("en_US.snbt and en_us.snbt are not byte-identical")

    locale_ids: Counter[str] = Counter()
    locale_title_ids: set[str] = set()
    for match in LOCALE_KEY_RE.finditer(lower):
        locale_id = match.group(2) or match.group(3)
        if locale_id:
            locale_ids[locale_id] += 1
            if ".title:" in match.group(0):
                locale_title_ids.add(locale_id)

    for chapter_id, rel in sorted(chapter_ids.items()):
        text = read(root / rel)
        has_inline = bool(re.search(r'^\ttitle:\s*', text, re.MULTILINE))
        if chapter_id not in locale_title_ids and not has_inline:
            report.warn(f"{rel}: chapter {chapter_id} has no localized or inline title")

    for qid, rel in sorted(quest_ids.items()):
        if qid not in locale_title_ids and not has_inline_title(quest_blocks[qid]):
            report.warn(f"{rel}: quest {qid} has no localized or inline title")

    group_title_ids = {
        match.group(1)
        for match in re.finditer(
            rf'^\tchapter_group\.({HEX_ID})\.title:', lower, re.MULTILINE
        )
    }
    for group_id in sorted(group_ids):
        if group_id not in group_title_ids:
            report.warn(f"Chapter group {group_id} has no localized title")

    known_ids = set(chapter_ids) | set(quest_ids) | group_ids
    orphan_locale_ids = sorted(key for key in locale_ids if key not in known_ids)
    if orphan_locale_ids:
        sample = ", ".join(orphan_locale_ids[:20])
        suffix = (
            ""
            if len(orphan_locale_ids) <= 20
            else f" (+{len(orphan_locale_ids) - 20} more)"
        )
        report.warn(
            f"{len(orphan_locale_ids)} localization IDs do not map to a current "
            f"chapter/quest/group: {sample}{suffix}"
        )

    stale_boss_group = "B055E50000000001"
    for path in root.glob("docs/quest-overhaul/*.md"):
        if stale_boss_group in read(path):
            report.warn(
                f"{path.relative_to(root)} still references stale Bosses group "
                f"{stale_boss_group}; current id is 5055E50000000001"
            )

    report.note(
        f"Scanned {len(chapter_files)} chapters, {len(quest_ids)} quests, "
        f"{sum(task_types.values())} tasks"
    )
    report.note(
        "Task types: "
        + ", ".join(f"{name}={count}" for name, count in sorted(task_types.items()))
    )
    report.note(f"Manual checkmark quests: {len(checkmark_quests)}")
    for qid, rel in checkmark_quests:
        report.note(f"CHECKMARK: {rel}: {qid}")

    return report


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "root",
        nargs="?",
        default=".",
        help="repository root (default: current directory)",
    )
    args = parser.parse_args()

    root = Path(args.root).resolve()
    report = validate(root)

    print(
        f"FTB Quests validation: {len(report.errors)} error(s), "
        f"{len(report.warnings)} warning(s)"
    )

    for message in report.errors:
        print(f"ERROR: {message}")
    for message in report.warnings:
        print(f"WARN: {message}")
    for message in report.info:
        print(f"INFO: {message}")

    return 1 if report.errors else 0


if __name__ == "__main__":
    sys.exit(main())
