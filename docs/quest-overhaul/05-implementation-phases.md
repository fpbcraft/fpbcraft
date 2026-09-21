# Implementation phases / parallel slices

The overhaul is intentionally split so multiple chats/agents can work without editing the same files at once.

## Phase 0 — Baseline and validation (this pass)

Deliverables:

- commit the current quest pack unchanged;
- document current structure and source provenance;
- research requested mods;
- record pack-owner design decisions;
- no quest edits.

## Phase 1 — Structural cleanup

**Owner scope:** chapter groups, simple Tutorial / Getting Started, moving existing quests only.

- remove `FPBCRAFT Additions` group;
- create/rename the approved top-level groups, including **Combat & Equipment**, **RPG Series** and **Bosses**;
- redistribute existing `fpb_*` pages without expanding them yet;
- move Supplementaries / archaeology / photography / Farmer's Delight content out of `Getting Started!`;
- preserve a compact first-session Tutorial / Getting Started page;
- keep quest IDs stable where possible;
- clean duplicate/orphan localization entries after moves.

This phase should not also add hundreds of new quests; its purpose is a clean baseline architecture.

## Phase 2 — Source restoration

**Owner scope:** source-pack-derived files only.

- restore Electro Energetics, Create Optical and Blaze Burner Fuels source chapters;
- restore compatible AOC quests into Create Kitchen, Andesite Age, Locomotive/Trains and Fun Additions;
- restore compatible Chronicles quests into their new domain pages;
- validate every restored item ID against 1.21.1.

## Phase 3A — Heavy industry / power

**Files isolated from other agents:** new/expanded Engineer's Paradise chapters.

- Create Nuclear
- Create Power Grid
- electricity overview / Create Crafts & Additions cleanup
- Electro Energetics post-restoration adjustments
- Create Ore Excavation / Big Cannons placement

## Phase 3B — Aeronautics / transport

- Create Aeronautics: Toolgun
- Aeroworks / rope connector integration
- Create Power Loader
- Railway Navigator / train services cleanup
- cross-links to Skybound where Aeronautics quests benefit from a grappling/moving-airship milestone

3A and 3B can run in parallel.

## Phase 4A — Farming & food

- Farmer's Delight deep progression
- Immortalers Delight
- Create Kitchen integration
- Delight-family addon branches

## Phase 4B — Fishing / sea exploration

- Tide Fishing
- Aquamirae
- Myths of the Sea
- Better Fishtanks / Ocean's Delight / Crabber's Delight cross-links

4A and 4B can run in parallel if their shared dependencies are agreed first.

## Phase 5A — Building & decoration

- Supplementaries
- furniture page
- Macaw's page
- Additional Lights / Redden's Stone Lanterns / MultiBeds
- Measurements and other small building utility quests

## Phase 5B — Adventure & exploration

- Better Archeology
- Galosphere
- Naturalist
- Exposure page final integration
- Hang Glider
- exploration utility cleanup

## Phase 5C — Combat & Equipment

- move/preserve Simply Swords source-derived progression
- Just Hammers
- Immersive Armors
- **Grappling Hook Mod: Skybound**
  - basic hook movement
  - Smithing Table upgrade progression
  - Long Fall Boots
  - functional upgrade branches
  - Create contraption milestone
  - Sable / Create Aeronautics airship milestone
- Traveler Tool Belt and other small equipment branches selected during implementation

5A, 5B and 5C can run in parallel once Phase 1 has created their target groups.

## Phase 6 — RPG Series

This should be its own focused workstream due to size.

Suggested sub-slices:

1. shared RPG onboarding / Gazebos / Spell Engine / Jewelry / Skill Tree;
2. Archer;
3. Rogue + Warrior (separate visual trees, shared implementation owner);
4. Paladin + Priest (separate visual trees, shared implementation owner);
5. Wizard Arcane/Fire/Frost;
6. Armory / Arsenal / Relics end-game integration.

## Phase 7 — Bosses

Use the approved **guided progression** model.

- enumerate actual bosses from installed jars / Boss Checklist registry;
- organize encounters into loose practical tiers/dimensions;
- use discovery → preparation → defeat → signature drop → onward progression where appropriate;
- connect boss drops to RPG / Aquamirae / Nuclear gates;
- avoid hard-gating unrelated bosses behind one another;
- avoid duplicate kill quests already owned by a mod page when a cross-chapter dependency is sufficient.

## Phase 8 — Book-wide polish and QA

- visual spacing/layout review;
- dependency graph / dead-end check;
- duplicate quest/task check;
- localization orphan check;
- missing item/entity ID check;
- quest completion detection sanity check on a test server;
- rewards pass (if rewards are desired);
- verify optional branches do not block unrelated progression;
- verify no source quest references a mod that is no longer installed.

## Coordination rule for parallel chats

Each implementation chat should own a small explicit set of chapter files. Changes to `chapter_groups.snbt`, global `data.snbt` or shared localization should be staged by one integration owner to reduce merge conflicts. When possible, new quest text should be kept in predictable locale-key ranges or reconciled in a dedicated integration pass.
