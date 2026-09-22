# FPBCraft FTB Quests

`quests/` is the current working quest pack.

`current/quests/` is the unpacked, unchanged pre-overhaul baseline supplied at the beginning of the project. `current/BASELINE.md` records hashes for its logical SNBT contents.

## Current state

- Phase 1 — structural cleanup — complete.
- Phase 2 — source restoration — complete.
- Phase 3A — Heavy Industry and Power — complete.
- Phase 3B — Aeronautics and Transportation — complete.
- Phase 4A + 4B — Farming/Food and Fishing/Sea Exploration — complete.
- Phase 5 — Building/Decoration, Adventure/Exploration, Combat/Equipment — complete.
- Phase 6 — RPG Series — complete.
- Phase 7 — Bosses and progression links — complete.
- Phase 8 — book-wide polish and static QA — in progress.

## Validation

Run:

```bash
python scripts/validate_ftbquests.py
```

The validator checks duplicate object/quest IDs, dangling dependencies, unknown chapter groups, locale-file synchronization, orphan localization IDs, and known structural regressions. GitHub Actions runs the same validation automatically for quest-pack changes.

See:
- [Phase 1 result](../docs/quest-overhaul/07-phase-1-result.md)
- [Phase 2 result](../docs/quest-overhaul/08-phase-2-result.md)
- [Phase 3A result](../docs/quest-overhaul/09-phase-3a-result.md)
- [Phase 3B result](../docs/quest-overhaul/10-phase-3b-result.md)
- [Phase 4A + 4B result](../docs/quest-overhaul/11-phase-4ab-result.md)
- [Phase 5 result](../docs/quest-overhaul/12-phase-5-result.md)
- [Phase 6 result](../docs/quest-overhaul/13-phase-6-result.md)
- [Phase 7 result](../docs/quest-overhaul/14-phase-7-result.md)
