# Source restoration plan

Principle: when a source quest is still accurate for FPBCraft, **restore/reuse it rather than rewriting it**. Preserve layout, wording, icons, dependencies and quest IDs where practical. Only adapt it where mod/version differences require it.

## All of Create Aeronautics restorations

### Restore whole or substantial chapters

#### Electro Energetics
The source pack already has a substantial Electro Energetics chapter and FPBCraft has the mod again. Restore it into Engineer's Paradise, then validate every task against the current 1.21.1 build. This should replace the current tiny generic electricity coverage as the detailed Electro Energetics path, while Create Crafts & Additions and Power Grid get their own linked pages.

#### Create: Optical
The source chapter exists and the mod is present. Restore rather than recreate.

#### Create: Blaze Burner Fuels
The source chapter exists and the mod is present. Restore and validate recipes/items.

### Restore quests into retained chapters

#### Create Kitchen
Restore the source `Some Assembly Required` branch because FPBCraft has the mod again:

- slicer axe/knife usage where still applicable;
- Sandwich Station;
- representative sandwich/burger quests.

#### Andesite Age
Restore the `Create: Mobile Packages` content (Robo Bee / Bee Port) because that addon is present again. Do **not** restore Rubberworks-only content unless Rubberworks returns.

#### Locomotive Age / Trains
Restore compatible `create: things and misc` train utilities such as train buffer, train stop, portable whistle and brass speaker after validating their current item IDs/version.

#### Fun Additions
Restore source quests for installed addons such as Trading Floor and any source quest that was removed solely because the mod was absent. Do not blindly restore `create_fantasizing` content just because another Dreams n' Desires/Fantasizing-family mod exists; validate the exact namespace/items first.

## Create Chronicles restorations

The old `Getting Started!` source should **not** be restored wholesale. Instead, restore its useful quests into their new domain pages.

High-confidence restorations because the corresponding mods are present now:

- Additional Lights → Building & Decoration
- Handcrafted → Furniture
- Game Discs → Collectibles / lifestyle branch
- MultiBeds → Furniture / bedding branch
- Redden's Stone Lanterns → Building & Decoration
- Better Archeology content → dedicated Archaeology page
- Exposure chain → existing Photography page
- Supplementaries chain → dedicated Supplementaries page
- Measurements → building/tool utility branch
- Farmer's Delight quests → dedicated Farmer's Delight page

### Do not restore absent-mod branches

Examples from the Chronicles source that are not currently in FPBCraft include Chipped, Refurbished Furniture, Spice of Life: Carrot Edition, Torchmaster, Ribbits, Building Gadgets 2, Construction Sticks and several Apotheosis/Deeper-and-Darker branches. Keep these out unless the mod list changes again.

## Existing chapters to preserve

- The main AOC Create age chapters are strong and should remain the backbone.
- Existing Trains/Aeronautics pages should be evolved rather than replaced.
- Simply Swords already has deep source-derived coverage; the main change is organizational placement, not expansion for expansion's sake.
- Sophisticated Backpacks coverage should remain, but do **not** restore Sophisticated Storage tasks because Sophisticated Storage is not in the current mod list.

## Restoration workflow for implementation

For every source quest considered for restoration:

1. confirm the mod is in the current FPBCraft manifest;
2. confirm item/entity IDs exist in the installed 1.21.1 jar;
3. check that the recipe/mechanic still matches the source text;
4. reuse the source quest/layout when valid;
5. move it to the target chapter if the source location was a catch-all;
6. retain dependencies only when they still make progression sense;
7. run an orphan/duplicate localization-ID audit afterward.
