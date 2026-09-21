# Phase 6 result — RPG Series

Status: **complete**.

Phase 6 replaces the old six-node loadout sampler with a full RPG Series workstream under the existing RPG group.

## Shared onboarding

The shared **RPG Foundations** chapter now covers:

- Spell Binding Table;
- RPG class-selection advancement;
- Arcane / Fire / Frost / Healing rune materials;
- all six Jewelry gems;
- Jeweler's Kit plus representative ring / necklace crafting;
- Skill Tree reset item;
- one manual interaction milestone for spending the first Skill Tree point.

The Skill Tree checkmark is intentional: the installed Skill Tree mod exposes the reset item but does not expose spent-point state as an FTB-detectable advancement or inventory condition.

## Radial class tree

All class progression now lives on a single **RPG Classes** page, visually inspired by the RPG Skill Tree:

- **Choose a Path** is the central hub;
- **Wizard** radiates north and then splits into Arcane, Fire, and Frost sub-branches;
- **Archer** radiates northeast;
- **Rogue** radiates southeast;
- **Warrior** radiates south;
- **Paladin** radiates southwest;
- **Priest** radiates northwest.

Each spoke keeps its class-specific equipment and native class/mastery advancements. Full equipment sets still use four concrete item tasks rather than informational or manual completion nodes.

The old per-class chapter files were removed after their quests were merged into `fpb_classes.snbt`. Quest IDs, task IDs, rewards, and localization keys were preserved so the layout change does not unnecessarily invalidate progress.

## Shared RPG end game

A new **RPG End Game** chapter integrates:

- Armory's epic smithing template and four upgrade-crystal families;
- complete Armory end-game armor / robe sets for the supported class paths;
- representative loot-only Arsenal weapons;
- rare class-oriented Jewelry pieces;
- representative Relics dungeon / high-value loot.

Armory uses its actual namespace, `armory_rpgs`, and the crystal branches follow the mod's declared class applicability:

- Vanquisher — Archer / Frost Wizard;
- Champion — Rogue / Warrior;
- Redeemer — Paladin / Priest;
- Conqueror — Arcane / Fire Wizard.

## Validation

Phase 6 audit after implementation:

- **2 RPG chapter files** — one radial class page plus the shared end-game page;
- **157 item tasks**;
- **17 advancement tasks**;
- **1 manual checkmark**;
- **0 duplicate 16-character IDs** within the Phase 6 chapters;
- both English localization files are byte-identical.

The single checkmark is the Skill Tree interaction described above.

## Next phase

Phase 7 is the boss and progression-link pass: guided boss progression, intentional cross-links from boss rewards into RPG end-game equipment, and final ordering / dependency cleanup.
