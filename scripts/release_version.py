#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import subprocess
from collections.abc import Iterable

SEMVER = re.compile(
    r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
    r"(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$"
)
STABLE = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$")


def normalize_version(value: str) -> str:
    version = value.strip()
    if version.startswith("v"):
        version = version[1:]
    if not SEMVER.fullmatch(version):
        raise ValueError(f"invalid semantic version: {value!r}")
    return version


def version_from_tag(tag: str) -> str:
    if not tag.startswith("v"):
        raise ValueError(f"release tag must start with 'v': {tag!r}")
    return normalize_version(tag)


def stable_base_version(tags: Iterable[str]) -> str:
    candidates = [
        parsed
        for tag in tags
        if (parsed := stable_tuple(tag)) is not None
    ]
    if not candidates:
        raise ValueError("no stable release tag found")
    return ".".join(str(part) for part in max(candidates))


def development_version(base: str, sha: str) -> str:
    version = normalize_version(base)
    if stable_tuple(version) is None:
        raise ValueError("development base version must be a stable semantic version")
    short = sha.strip().lower()[:8]
    if not re.fullmatch(r"[0-9a-f]{7,8}", short):
        raise ValueError(f"invalid git SHA: {sha!r}")
    return f"{version}-dev.{short}"


def stable_tuple(value: str) -> tuple[int, int, int] | None:
    version = value[1:] if value.startswith("v") else value
    match = STABLE.fullmatch(version)
    if not match:
        return None
    return tuple(int(part) for part in match.groups())


def next_version(
    tags: Iterable[str],
    bump: str,
    exact: str | None = None,
) -> str:
    if exact and exact.strip():
        return normalize_version(exact)

    if bump not in {"patch", "minor", "major"}:
        raise ValueError(f"invalid bump: {bump!r}")

    try:
        major, minor, patch = (
            int(part) for part in stable_base_version(tags).split(".")
        )
    except ValueError as error:
        raise ValueError("no stable release tag found; provide --exact") from error

    if bump == "major":
        return f"{major + 1}.0.0"
    if bump == "minor":
        return f"{major}.{minor + 1}.0"
    return f"{major}.{minor}.{patch + 1}"


def repository_tags() -> list[str]:
    output = subprocess.check_output(
        ["git", "tag", "--list", "v*"],
        text=True,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def reachable_repository_tags() -> list[str]:
    output = subprocess.check_output(
        ["git", "tag", "--merged", "HEAD", "--list", "v*"],
        text=True,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def main() -> None:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    from_tag = subparsers.add_parser("from-tag")
    from_tag.add_argument("tag")

    development = subparsers.add_parser("dev")
    development.add_argument("--sha", required=True)

    next_release = subparsers.add_parser("next")
    next_release.add_argument("--bump", choices=("patch", "minor", "major"), required=True)
    next_release.add_argument("--exact", default="")

    args = parser.parse_args()

    try:
        if args.command == "from-tag":
            print(version_from_tag(args.tag))
        elif args.command == "dev":
            print(
                development_version(
                    stable_base_version(reachable_repository_tags()),
                    args.sha,
                )
            )
        else:
            print(
                next_version(
                    repository_tags(),
                    args.bump,
                    args.exact,
                )
            )
    except ValueError as error:
        raise SystemExit(str(error)) from error


if __name__ == "__main__":
    main()
