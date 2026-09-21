# Current-state audit

## Inputs inspected

- Current FPBCraft quest pack (`ftbquests-fpbcraft_v2`)
- All of Create Aeronautics quest pack
- Create Chronicles quest pack
- Current FPBCraft mod list
- Source-pack mod lists

The current FPBCraft baseline has **31 chapter files**. It is already a hybrid of the two source packs plus eight custom `fpb_*` chapters. The main problem is not lack of source material; it is that source content was selectively pruned and custom additions were then isolated in a separate group.

## Current chapter groups

| Current group | Observed role | Plan |
| --- | --- | --- |
| Create | Core Create ages / progression | Keep |
| Aeronautics | Aeronautics / Sable / propulsion | Keep; absorb Flight Workshop |
| Engineer's Paradise | Create addons / industrial systems | Keep; become home for nuclear/electricity/industrial pages |
| Transportation | Trains / minecarts | Keep; absorb Railway Services and Power Loader |
| Farming and Food | Create Kitchen | Keep; expand heavily |
| Create (Chronicles-derived duplicate group) | Electricity | Remove duplicate grouping; merge into Engineer's Paradise |
| Storage | Sophisticated Backpacks | Keep; absorb Tom's Storage / computer-storage material |
| Collectibles | Simply Swords | Reconsider: Simply Swords fits Combat/RPG better; keep a collectibles group only for genuinely collectible content |
| Others | Golem Overhaul | Reorganize by domain where possible |
| FPBCRAFT Additions | Custom FPBCraft pages | **Remove group** and redistribute its chapters |

## `Getting Started!` is overloaded

`end_basics.snbt` mixes unrelated systems that should not live on one starter page. It currently contains or historically inherited material for:

- basic vanilla progression and onboarding;
- Farmer's Delight;
- Supplementaries;
- Better Archeology;
- Exposure photography;
- Measurements;
- Waystones / travel utility;
- Another Furniture;
- Simply Swords fragments;
- Farming for Blockheads;
- several decorative / utility mods inherited from Create Chronicles.

This causes two kinds of duplication: a system appears in `Getting Started!` and again in its dedicated/custom page, or a mod gets only a single orphaned quest in the starter page even though it deserves a coherent branch elsewhere.

### Recommended starter rule

Keep only things a player benefits from knowing in the first session, such as:

- how to navigate the quest book;
- EMI / Jade / Ponder conventions;
- basic Create entry point if it is intentionally part of onboarding;
- claims / teams / recovery / travel basics if they are server-critical;
- a small number of vanilla milestones only when they unlock a later branch.

Move all substantive mod progression out of `Getting Started!`.

## Custom `fpb_*` chapter disposition

| Current custom chapter | Current contents | Target home |
| --- | --- | --- |
| `fpb_classes` | one shallow quest per RPG archetype | Replace with a full RPG Series group and per-class trees |
| `fpb_computers` | Tom's Storage + CC:Tweaked/AP/bridge | Storage & Logistics; optionally split Computers from Storage if the branch becomes large |
| `fpb_deep_seas` | Tide + Aquamirae + Better Fishtanks + submarine content | Adventure/Exploration and Fishing; preserve useful layout where possible |
| `fpb_flight_tools` | Toolgun + rope connector + Aeroworks | Aeronautics |
| `fpb_kitchen` | Farmer's Delight + Cooking for Blockheads + Create farming integrations | Farming & Food |
| `fpb_navigation` | Railway Navigator + Power Loader + train parts + Numismatics | Transportation |
| `fpb_photography` | Exposure addons | Exploration/Lifestyle or Building & Decoration; absorb the old Getting Started photography chain |
| `fpb_workshop` | Ore Excavation + Big Cannons + Create Nuclear | Engineer's Paradise; split into dedicated pages as needed |

## Stale localization / cleanup concern

The current `lang/en_us.snbt` contains strings for quests/items that are not present in the current chapter files (examples include a brass Power Loader quest, magnetic gun, Naturalist items, extra Tide/Aquamirae items). Before implementation, run an ID/reference audit so moved/restored quests do not accidentally create duplicate IDs or leave more orphaned localization entries.

Do not treat stale language strings as authoritative proof that a quest currently exists.
