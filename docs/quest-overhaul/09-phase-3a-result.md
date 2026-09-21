# Phase 3A result — Heavy Industry & Power

Status: **complete**.

This slice expands the Engineer's Paradise / heavy-industry side of the quest book. It does not include the Aeronautics and transportation work planned for Phase 3B.

## Heavy Industry split

The old `Heavy Industry` page mixed Create Ore Excavation, Create Big Cannons and Create Nuclear into one small sampler. It has been removed and replaced with three focused chapters.

### Create Ore Excavation

Existing FPBcraft quest IDs were preserved while moving the branch into its own chapter.

Progression now covers:

- introduction to hidden ore veins;
- Vein Finder;
- Sample Drill;
- Drilling Machine;
- Drill;
- Diamond Drill;
- Fluid Extractor.

The page teaches the discovery → sampling → extraction loop instead of presenting the machines as unrelated items.

### Create Big Cannons

Existing casting/drilling/building/mount quests were moved into a dedicated chapter.

The page now covers:

- cannon casting entry;
- Casting Sand;
- Cannon Drill;
- Cannon Builder;
- Cannon Mount;
- Ram Rod;
- Block Armor Inspection Tool.

This remains a compact introduction. A much larger weapons/ammunition expansion is deliberately outside this slice.

## Create Nuclear — major expansion

The old four-node sampler has been replaced by a guided reactor progression.

The implementation targets the **Minecraft 1.21.1 / Create Nuclear 1.3.2-beta.3** line used by the public 1.21.1 source. It deliberately does **not** teach newer V2-only reactor tiers, thorium or coolant systems.

Progression now includes:

1. uranium and lead discovery;
2. coal dust and steel production;
3. lead and reinforced glass;
4. uranium powder;
5. yellowcake;
6. enriched yellowcake;
7. Graphite Rod;
8. Uranium Rod;
9. reactor casing;
10. reactor input/output;
11. reactor frame/core/cooler;
12. radiation-safety equipment;
13. Reactor Controller;
14. reactor blueprint/configuration;
15. first criticality / operation milestone;
16. automated fuel-cycle milestone.

The Reactor Controller quest explicitly explains the Nether Star / Wither progression gate present in this 1.21.1 version.

Original Nuclear quest IDs were preserved for the intro, Graphite Rod, Reactor Controller and Uranium Rod.

## Electricity organization

Electricity is now presented as three adjacent pages rather than one generic electricity sampler.

### Create Crafts & Additions

The former `Electricity` page has been renamed/reworked as a focused Create Crafts & Additions bridge between rotational power and FE.

It covers:

- Rolling Mill;
- Alternator;
- Connector;
- Copper Spool;
- Modular Accumulator;
- Electric Motor;
- Capacitor;
- Portable Energy Interface;
- Tesla Coil.

Existing quest IDs were retained and previously unused localization-backed IDs were reused for the restored additions.

### Create: Electro Energetics

The restored source chapter remains intact and sits directly after Crafts & Additions. This preserves the detailed All of Create Aeronautics progression rather than duplicating it with newly written quests.

### Create: Power Grid

A new guided chapter was added for Power Grid's own electrical model rather than treating it as ordinary FE wiring.

The 1.21.1 source branch (`patryk3211/PowerGrid`, `architectury-1.21.1/dev`) was used to validate registry names.

Progression covers:

- insulated wire;
- a first closed-loop source/load circuit;
- switches and lamps;
- multimeter diagnostics;
- batteries;
- generator assembly;
- power measurement;
- Electric Motor / Create integration;
- transformer-core and in-world transformer milestone;
- fuses, grounding and safe distribution;
- heavy wire / high-voltage switching;
- solar generation;
- FE Inverter bridging;
- Circuit Design Table / Circuit Board;
- operating a larger real grid.

The transformer is intentionally represented by a `transformer_core` item task plus a manual assembly milestone. The medium transformer is assembled in-world and is not an inventory item in the validated 1.21.1 source.

## Engineer's Paradise order

The section is now ordered roughly as:

1. Aquatic Ambitions
2. Create: Connected
3. Enchantment Industry
4. Fun Additions
5. Pump Dat Oil / Diesel Generators
6. Create Ore Excavation
7. Create Big Cannons
8. Create: Optical
9. Create: Blaze Burner Fuels
10. Create Crafts & Additions
11. Create: Electro Energetics
12. Create: Power Grid
13. Create Nuclear

## Validation

Final quest tree:

- **43 chapter files**
- **730 unique quest IDs**
- **0 duplicate quest IDs**
- **0 missing dependency targets**
- **0 chapters assigned to unknown groups**
- both English locale files are byte-identical
- **0** remaining `rubberworks:` references
- **0** invalid `powergrid:transformer_medium` item references
- old `fpb_workshop.snbt` removed

The test ZIP contains a top-level `quests/` directory and passes a full ZIP integrity test.

## Deferred to later slices

Not included here:

- Create Aeronautics Toolgun;
- Create Power Loader;
- Railway Services;
- Skybound/Aeronautics links;
- broader Big Cannons progression;
- farming/food, sea exploration, building, combat, RPG or boss expansions.


## Post-slice formatting hotfix

Testing Phase 3A in-game exposed two FTB Quests localization issues:

- literal ampersands in structural titles (for example `Combat & Equipment` and `Create Crafts & Additions`) were interpreted as legacy formatting codes;
- several newly generated chapter/group IDs used the high bit of the 64-bit ID range, causing those chapter/group titles to resolve as `Unnamed` in the client.

The hotfix:

- replaces literal structural-title ampersands with `and`;
- remaps only the newly generated affected chapter/group IDs into the normal positive ID range;
- updates every group reference and localization key;
- preserves all existing source IDs, quest IDs, tasks and dependencies.

The corrected test package is `fpbcraft-quests-phase-3a-format-fix.zip`.
