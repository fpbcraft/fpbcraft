# Phase 4A + 4B result — Farming, Food, Fishing and Sea Exploration

Status: **complete**.

Phases 4A and 4B were implemented together so food, fishing and ocean exploration can cross-link cleanly without duplicating progression.

## Phase 4A — Farming and Food

### Farmer's Delight

The existing `Farm to Table` chapter has been expanded into a real Farmer's Delight progression while preserving its original chapter ID and existing quest IDs where possible.

It now covers:

- Cabbage;
- Tomatoes;
- Onions;
- Rice and its shallow-water growing setup;
- Cutting Board;
- Knife;
- Skillet;
- Stove;
- Cooking Pot;
- representative rice, cabbage and stew recipes;
- Straw;
- Organic Compost;
- Rich Soil;
- the Farming for Blockheads Market as an optional convenience branch;
- the existing Cooking for Blockheads kitchen branch.

The page deliberately teaches crops, ingredients and cooking techniques instead of adding a quest for every meal.

Create automation was removed from this page where it duplicated the existing **Create Kitchen** chapter.

### Create Kitchen

The existing source-derived Create Kitchen chapter remains the automation/integration page and is placed after Farmer's Delight and Immortalers Delight.

It continues to cover:

- Create + Farmer's Delight handling;
- Slice and Dice;
- sprinkler/fluid interactions;
- Create Integrated Farming;
- Some Assembly Required sandwiches.

### Immortalers Delight

A dedicated 1.21.1 progression page was added.

The implementation was validated against the upstream `Renyigesai/immortalers_delight` `1.21.1Neo` branch.

It covers:

- obtaining a Sniffer Egg;
- Sniffer Fur Brush;
- Evolutcorn;
- Pearlip;
- Himekaido;
- Leisamboo;
- Kwat Wheat;
- Alfalfa;
- Warped Laurel;
- Ancient Stove;
- Enchantal Cooler;
- representative Kwat, Leisamboo and Alfalfa foods;
- optional Rusty Ancient Blade → Ancient Blade progression.

The individual crop quests explain their unusual discovery/growing rules rather than grouping all seeds into one checklist.

### Delight Addons

A compact shared chapter was added instead of creating many nearly-empty pages.

Concrete food milestones include:

- Ocean's Delight Fugu Roll;
- Ocean's Delight Guardian Soup;
- Crabber's Delight Bisque;
- Crabber's Delight Turtle Stew;
- a catch-to-dinner crossover milestone.

Compact optional introduction quests are also provided for:

- Chef's Delight;
- Crate Delight;
- Miner's Delight;
- End's Delight;
- My Nether's Delight;
- Rustic Delight;
- Storage Delight.

These optional quests intentionally direct players to EMI for a representative recipe rather than attempting to enumerate every addon food.

## Phase 4B — Fishing and Sea Exploration

### Underwater Engineering

The old mixed `Deep Seas` chapter has been reduced to its Create Submarine progression and renamed **Underwater Engineering**.

Its original chapter ID and submarine quest IDs remain intact.

It covers:

- Ballast Tank;
- Ballast Vent;
- Submarine Propeller;
- Barometer;
- Iron Pressurizer.

Tide, Aquamirae and Better Fishtanks were moved out into dedicated chapters.

### Tide Fishing

A full Tide progression page was added, preserving the existing Journal / Depth Meter / Fish Finder quest IDs.

The current Tide source and README were used to validate the mechanics and item IDs.

The page covers:

- Fishing Journal;
- the fishing minigame;
- Angling Table;
- bait;
- hooks;
- fishing lines;
- bobber/rod customization;
- Depth Meter;
- Climate Gauge;
- Weather Radio;
- Fish Finder;
- Fish Satchel;
- catching fish across multiple habitats;
- optional Crystal Fishing Rod;
- bringing a live bucketable catch home.

The Journal is treated as the backbone of progression because Tide updates it through actual catches.

### Aquamirae

A dedicated Aquamirae expedition page was added, preserving the existing Echo Compass quest ID.

The implementation was validated against the current `ObscuriaLithium/Aquamirae` `1.21.1` source branch.

Progression includes:

- Echo Compass;
- reaching the Ship Graveyard;
- Ship Graveyard Echo;
- Abyssal Amethyst;
- optional full Abyssal armor;
- Rune of the Storm;
- Dreadwake;
- Terrible Chakram;
- Mother of the Maze encounter;
- Maze Rose;
- Shell Horn;
- Tidepiercer / Captain Cornelia progression.

The explicit guided boss-kill quests are intentionally deferred to the dedicated Bosses phase. This page handles discovery, materials, equipment and encounter progression.

### Myths of the Sea

A discovery-oriented page was added for:

- Bake Kujira / ocean coasts;
- Bunyip / swamps;
- Abaia / warm oceans;
- Hippocampus;
- Leviathan territory;
- deep-ocean Kraken discovery.

Major boss kills remain deferred to the Bosses section.

### Aquariums

Better Fishtanks now has a dedicated compact page rather than being attached to Aquamirae.

Existing Guide Book and Aquarist Table quest IDs were preserved.

It covers:

- Aquarist Guide;
- Aquarist Table;
- building a working aquarium;
- bringing a live Tide catch home and displaying it.

## Cross-section design

The combined implementation deliberately creates several links between systems without forcing hard dependencies:

- Tide catches → Delight addon meals;
- Tide live fish → Better Fishtanks;
- submarine engineering → ocean exploration;
- Aquamirae and Myths encounters → later Bosses progression.

## Validation

Final combined quest tree:

- **53 chapter files**
- **824 unique quest IDs**
- **0 duplicate quest IDs**
- **0 missing dependency targets**
- **0 chapters assigned to unknown groups**
- **0 missing title keys for quests added/modified in this slice**
- **0 missing chapter title localization keys**
- **0 missing chapter-group title localization keys**
- both English locale files are byte-identical
- all generated chapter IDs remain below the signed 64-bit boundary
- no structural titles contain a literal unescaped ampersand
- test ZIP passes a full archive integrity test

## Source validation

Version-sensitive item IDs/mechanics were checked against:

- Farmer's Delight 1.21.x public documentation/source;
- `Renyigesai/immortalers_delight` — `1.21.1Neo`;
- `Lightning-64/Tide-2`;
- `ObscuriaLithium/Aquamirae` — `1.21.1`;
- the current FPBcraft quest/localization baseline for installed Better Fishtanks, Ocean's Delight and Crabber's Delight content.

## Next phase

Phase 5 is split into:

- 5A — Building and Decoration;
- 5B — Adventure and Exploration;
- 5C — Combat and Equipment.
