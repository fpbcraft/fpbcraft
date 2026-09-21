# Phase 7 result — Bosses and progression links

Status: **complete**.

Phase 7 creates the dedicated Bosses gameplay path while keeping Boss Checklist as the exhaustive encyclopedia.

## Boss Progression page

A new `boss_progression.snbt` chapter lives in the existing **Bosses** group and is split visually into four independent encounter families. Bosses are arranged by loose practical tier, but unrelated encounters are not hard-gated behind one another.

### Vanilla gates

- **Wither** — direct kill task plus required Nether Star trophy;
- **Ender Dragon** — direct kill task, with text explicitly acknowledging Ender Dragon Fight Remastered.

The Wither milestone is now a real dependency of the Create Nuclear Reactor Controller quest because the installed recipe progression requires the Nether Star.

### Mowzie's Mobs

- **Ferrous Wroughtnaut** — native Mowzie's kill advancement plus guaranteed Wrought Helm and Axe of a Thousand Metals;
- **Frostmaw** — native kill advancement; Ice Crystal is intentionally not required because it can be stolen before the kill;
- **Umvuthi, the Sunbird** — native kill advancement plus guaranteed Sol Visage.

Native advancements are preferred to manual checkmarks whenever the mod already exposes a reliable encounter-completion signal.

### Legendary Monsters

The installed 1.21.1 source exposes three entities under its actual `IAnimatedBoss` implementation:

- **Cloud Golem** — native boss advancement plus guaranteed Air Rune and Atmospheric Boots;
- **Possessed Paladin** — native boss advancement plus guaranteed Corrupted Soul and Soul Great Sword;
- **The Obliterator** — native boss advancement plus guaranteed Portal Shard.

The Obliterator is laid out as an outer-tier encounter, but the other Legendary bosses are not artificially required before it.

### Sea bosses

The Bosses page owns the actual kills while the Adventure pages continue to own discovery and exploration:

- **Maze Mother** — requires the existing Ship Graveyard discovery milestone, then a kill plus Maze Rose;
- **Captain Cornelia** — requires the existing Shell Horn progression before the kill;
- **Kraken** — requires the existing Myths of the Sea Kraken discovery quest, then a kill plus guaranteed Kraken Tentacle;
- **Leviathan** — requires the existing Leviathan discovery quest, then a kill plus guaranteed Leviathan Heart.

Aquamirae's Maze Rose and Cornelia's Legacy equipment nodes now depend back on their respective Bosses-page kills, giving the two sections a real cross-chapter progression instead of duplicate boss quests.

## RPG end-game link

The **RPG End Game** root now depends on the Phase 7 boss quests with:

`dependency_requirement: "one_completed"`

This means any one major boss unlocks the shared Armory / Arsenal / Jewelry / Relics end-game page. It creates a boss-to-RPG progression link without forcing players through a single prescribed boss chain.

## Detection policy

Phase 7 uses:

- direct FTB kill tasks for Vanilla, Aquamirae, and Myths of the Sea entities;
- native advancements for Mowzie's Mobs and Legendary Monsters;
- concrete guaranteed boss drops where they are reliable;
- **zero manual checkmarks** on the Boss Progression page.

## Next phase

Phase 8 is the book-wide polish and QA pass: layout review, dependency/dead-end checks, duplicate IDs, localization cleanup, missing item/entity validation, completion-detection sanity checks, and optional-branch review.
