#!/usr/bin/env python3
"""Static validation for the FPBCraft FTB Quests pack.

This intentionally uses lightweight text parsing instead of a full SNBT parser so
it can run in CI without additional dependencies.
"""

from __future__ import annotations

import argparse
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

ID_RE = re.compile(r'\bid:\s*"([0-9A-F]{16})"')
CHAPTER_ID_RE = re.compile(r'^\tid:\s*"([0-9A-F]{16})"\s*$', re.MULTILINE)
QUEST_ID_RE = re.compile(r'^\t\t\tid:\s*"([0-9A-F]{16})"\s*$', re.MULTILINE)
GROUP_RE = re.compile(r'^\tgroup:\s*"([0-9A-F]{16})"\s*$', re.MULTILINE)
DEPENDENCIES_RE = re.compile(r'dependencies:\s*\[([^\]]*)\]')
QUOTED_ID_RE = re.compile(r'"([0-9A-F]{16})"')
GROUP_ID_RE = re.compile(r'\{\s*id:\s*"([0-9A-F]{16})"\s*\}')
LOCALE_KEY_RE = re.compile(r'^\t((?:chapter|quest|chapter_group)\.([0-9A-F]{16})\.(?:title|quest_desc)|chapter_group\.([0-9A-F]{16})\.title):', re.MULTILINE)


class Report:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []

    def error(self, message: str) -> None:
        self.errors.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


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

    group_ids = set(GROUP_ID_RE.findall(read(groups_file)))
    if not group_ids:
        report.error("No chapter groups found")

    all_ids: dict[str, list[str]] = defaultdict(list)
    chapter_ids: dict[str, str] = {}
    quest_ids: dict[str, str] = {}
    dependencies: list[tuple[str, str, str]] = []

    for path in chapter_files:
        text = read(path)
        rel = path.relative_to(root).as_posix()

        chapter_match = CHAPTER_ID_RE.search(text)
        if not chapter_match:
            report.error(f"{rel}: missing top-level chapter id")
            continue
        chapter_id = chapter_match.group(1)
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

        file_quest_ids = QUEST_ID_RE.findall(text)
        for qid in file_quest_ids:
            if qid in quest_ids:
                report.error(
                    f"Duplicate quest id {qid}: {quest_ids[qid]} and {rel}"
                )
            quest_ids[qid] = rel

        for match in DEPENDENCIES_RE.finditer(text):
            owner_prefix = text[: match.start()]
            owners = QUEST_ID_RE.findall(owner_prefix)
            owner = owners[-1] if owners else chapter_id
            for dep in QUOTED_ID_RE.findall(match.group(1)):
                dependencies.append((owner, dep, rel))

        if "\\n" in text:
            report.warn(f"{rel}: contains a literal \\n sequence; verify formatting")

    duplicates = {k: v for k, v in all_ids.items() if len(v) > 1}
    for value, paths in sorted(duplicates.items()):
        unique_paths = sorted(set(paths))
        # Reuse inside the same file is also invalid for FTB object IDs.
        report.error(f"Duplicate object id {value}: {', '.join(unique_paths)}")

    for owner, dep, rel in dependencies:
        if dep not in quest_ids:
            report.error(f"{rel}: quest {owner} depends on missing quest {dep}")

    upper = read(locale_upper)
    lower = read(locale_lower)
    if upper != lower:
        report.error("en_US.snbt and en_us.snbt are not byte-identical")

    locale_ids: Counter[str] = Counter()
    for match in LOCALE_KEY_RE.finditer(lower):
        locale_id = match.group(2) or match.group(3)
        if locale_id:
            locale_ids[locale_id] += 1

    known_ids = set(chapter_ids) | set(quest_ids) | group_ids
    orphan_locale_ids = sorted(k for k in locale_ids if k not in known_ids)
    if orphan_locale_ids:
        sample = ", ".join(orphan_locale_ids[:20])
        suffix = "" if len(orphan_locale_ids) <= 20 else f" (+{len(orphan_locale_ids)-20} more)"
        report.warn(
            f"{len(orphan_locale_ids)} localization IDs do not map to a current "
            f"chapter/quest/group: {sample}{suffix}"
        )

    # Targeted regressions found during the overhaul.
    stale_boss_group = "B055E50000000001"
    for path in root.glob("docs/quest-overhaul/*.md"):
        if stale_boss_group in read(path):
            report.warn(
                f"{path.relative_to(root)} still references stale Bosses group "
                f"{stale_boss_group}; current id is 5055E50000000001"
            )

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

    print(f"FTB Quests validation: {len(report.errors)} error(s), "
          f"{len(report.warnings)} warning(s)")

    for message in report.errors:
        print(f"ERROR: {message}")
    for message in report.warnings:
        print(f"WARN: {message}")

    return 1 if report.errors else 0


if __name__ == "__main__":
    sys.exit(main())
