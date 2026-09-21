# Phase 2 result — source restoration

Status: **complete**.

This slice restores compatible quest content from **All of Create Aeronautics** and **Create Chronicles** while preserving the Phase 1 organization. It intentionally does not perform the larger bespoke mod expansions planned for later phases.

## Whole AOC chapters restored

Under **Engineer's Paradise**:

- **Create: Optical** — restored from the source pack.
- **Create: Blaze Burner Fuels** — restored from the source pack.
- **Create: Electro Energetics** — restored from the source pack.

Electro Energetics was adapted for the current FPBcraft mod list: the obsolete Rubberworks-only gate was removed and the insulation progression now connects directly to the installed copper-wire path.

The AOC reward tables used by restored quests are included under `quests/reward_tables/`.

## AOC branches restored into existing chapters

- **Create Kitchen / Some Assembly Required**
  - Sandwich Station
  - Chicken Sandwich
  - Burger
- **Andesite Age / Create: Mobile Packages**
  - Robo Bee + Bee Port
- **Locomotive Age / Create Things & Misc**
  - Train Buffer
  - Train Stop
  - Portable Whistle
  - Brass Speaker
- **Fun Additions**
  - Trading Depot
  - Hypertube

Dependencies were remapped onto the equivalent quests already present in FPBcraft instead of restoring duplicate prerequisites.

## Create Chronicles content restored

- Better Archeology artifact progression added to the dedicated **Archaeology** chapter.
- **Additional Lights** added to Building & Furniture.
- **Handcrafted** added to Building & Furniture.
- **Redden's Stone Lanterns** added to Building & Furniture.
- **MultiBeds** progression restored as a branch in Building & Furniture.
- **Game Discs** restored as its own compact Adventure & Exploration chapter.

Existing FPBcraft quests for Another Furniture and Measurements were retained instead of restoring source duplicates.

## Getting Started visual cleanup

The Phase 1 content split left the surviving Getting Started nodes at their old source coordinates, producing a very wide, sparse graph.

This slice:

- preserves the same 22 first-session quests;
- compacts the main vanilla progression into a readable horizontal chain;
- groups the smaller side branches around that chain;
- removes stale decorative images positioned for the old 74-quest layout;
- does not change the quests' tasks or intended progression.

## Validation

Generated working tree:

- **40 chapter files**
- **688 quest IDs**
- **0 duplicate quest IDs**
- **0 missing dependency targets**
- **0 chapters assigned to unknown/removed groups**
- **0 remaining `rubberworks:` references**

## Test package

Every completed slice now produces a ZIP with a top-level `quests/` folder so it can be dropped into an FTB Quests instance for testing.
