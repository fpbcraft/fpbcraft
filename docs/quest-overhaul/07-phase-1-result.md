# Phase 1 result — structural cleanup

Status: **complete**.

This slice changes quest-book organization only. It deliberately does not perform the large mod-content expansions planned for later phases.

## Structural changes

Top-level chapter groups are now:

1. Create
2. Aeronautics
3. Engineer's Paradise
4. Transportation
5. Farming & Food
6. Storage
7. Building & Decoration
8. Adventure & Exploration
9. Combat & Equipment
10. RPG Series
11. Bosses

The old duplicate Create/Electricity group, `Others`, `Collectibles`, and `FPBCRAFT Additions` presentation are removed/reused as appropriate.

## Existing FPBCraft chapters integrated

The existing custom chapters keep their quest layouts and IDs, but are moved into the appropriate sections and lose the visible `FPBCRAFT:` title prefix:

- Heavy Industry → Engineer's Paradise
- Flight Workshop → Aeronautics
- Railway Services → Transportation
- Farm to Table → Farming & Food
- Storage & Computers → Storage
- Deep Seas → Adventure & Exploration
- Photography → Adventure & Exploration
- Choose Your Loadout → RPG Series
- Electricity → Engineer's Paradise
- Golem Overhaul → Adventure & Exploration
- Simply Swords → Combat & Equipment

## Getting Started cleanup

The original `Getting Started!` had 74 quests. It now has 22 and is limited to first-session / vanilla-server progression.

Moved into dedicated pages:

- Supplementaries → `Building & Decoration / Supplementaries`
- Better Archeology + Sniffer archaeology progression → `Adventure & Exploration / Archaeology`
- Waystones / sleeping / map / spyglass material → `Adventure & Exploration / Travel & Exploration`
- decoration/furniture entry → `Building & Decoration / Building & Furniture`
- combat/Simply Swords introductory material → `Combat & Equipment / Combat Basics`
- unique Farmer's Delight farming quests → existing `Farming & Food / Farm to Table`

The old Exposure chain in Getting Started was removed because the existing Photography page already covers the same progression. The duplicate Farmer's Delight intro/cooking quests were likewise removed in favor of the existing Farm to Table quests.

The stale Arts & Crafts quest was removed because that mod is not in the current FPBCraft mod list.

## ID and dependency preservation

Moved quests retain their original quest IDs, task IDs, rewards, descriptions, and dependencies unless a coordinate needed changing for the new page layout.

Validation after the move:

- 573 quest IDs
- 0 duplicate quest IDs
- 0 missing dependency targets
- 0 chapters assigned to removed/unknown groups

## New chapter IDs

- Supplementaries: `51A9AEEA7D5D1001`
- Archaeology: `A8C4AE0109F01001`
- Travel & Exploration: `7A4E1001A0B10001`
- Building & Furniture: `B01D1E6F0A100001`
- Combat Basics: `C0B47BA51C500001`

## New/reused group IDs

- Building & Decoration: `B17DDEC0A7E6F001`
- Adventure & Exploration: `AD7E4E8A10A10001`
- Combat & Equipment: existing `4863600DDEA1AF02` (formerly Collectibles)
- RPG Series: existing `4A93CDBE321781DA` (formerly FPBCRAFT Additions)
- Bosses: `B055E50000000001`

## Deliberately deferred

No new deep progression was added for Create Nuclear, Toolgun, Tide, Farmer's Delight crops, Tom's Storage, Power Grid, Power Loader, Immortalers Delight, furniture, Aquamirae, Just Hammers, Macaw's, Galosphere, Immersive Armors, Naturalist, Skybound, Hang Glider, RPG classes, or bosses. Those belong to subsequent slices.
