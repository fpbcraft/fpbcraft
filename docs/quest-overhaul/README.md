# FPBCraft FTB Quests overhaul

Status: **implementation in progress**. Phases 1–6 are complete; Phase 7 Bosses / progression links is next.

This directory documents the planned overhaul of the FPBCraft quest book. The starting point is the current FPBCraft quest pack plus the quest packs from **All of Create Aeronautics** and **Create Chronicles**. The source packs are intentionally treated as reusable source material: good layouts, wording, dependency trees, icons and quest ideas should be retained where they still match FPBCraft.

## Goals

1. Remove the overloaded `Getting Started!` catch-all and move mod-specific content to coherent chapters.
2. Restore useful source quests that were removed when their mods were temporarily absent.
3. Eliminate the isolated `FPBCRAFT Additions` group by integrating those chapters into the existing book structure.
4. Add substantially deeper guidance for the mods called out in the overhaul request.
5. Build a first-class RPG Series section with a tree for every class.
6. Build boss progression that complements, rather than merely duplicates, Boss Checklist.
7. Keep the book approachable: quests should explain mechanics and progression, not become an exhaustive item checklist unless collecting items is itself the point.

## Documents

- [Current-state audit](01-current-state-audit.md)
- [Target chapter architecture](02-target-chapter-architecture.md)
- [Mod coverage plan](03-mod-coverage-plan.md)
- [Source restoration plan](04-source-restoration-plan.md)
- [Implementation phases / slices](05-implementation-phases.md)
- [Open questions](06-open-questions.md)
- [Research sources](research-sources.md)
- [Phase 5 result](12-phase-5-result.md)

## Current baseline

The unmodified quest baseline is stored under `ftbquests/current/` in the repository. Implementation work should branch from that baseline and keep IDs stable whenever a quest is being moved rather than replaced.
