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

## Class chapters

Separate visual trees now exist for:

- **Archer** — Composite Longbow, Archer armor, Ranger armor, Battle Quiver, native class/mastery advancements;
- **Rogue** — dagger/sickle progression, Rogue armor, Assassin armor, native class/mastery advancements;
- **Warrior** — double-axe/glaive progression, Warrior armor, Berserker armor, native class/mastery advancements;
- **Paladin** — mace/shield progression, Paladin armor, Crusader armor, native class/mastery advancements;
- **Priest** — holy wand/staff progression, Priest and Prior robes, native class/mastery advancements;
- **Wizard** — one apprentice root splitting visibly into Arcane, Fire, and Frost paths, each with its own book, staff, full robe set, and native mastery advancement.

Full equipment sets use four concrete item tasks rather than informational or manual completion nodes.

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

- **8 RPG chapter files**;
- **157 item tasks**;
- **17 advancement tasks**;
- **1 manual checkmark**;
- **0 duplicate 16-character IDs** within the Phase 6 chapters;
- both English localization files are byte-identical.

The single checkmark is the Skill Tree interaction described above.

## Next phase

Phase 7 is the boss and progression-link pass: guided boss progression, intentional cross-links from boss rewards into RPG end-game equipment, and final ordering / dependency cleanup.
