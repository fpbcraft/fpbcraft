# Mod coverage plan

This document records the **planned quest depth**, not final quest IDs or exact item tasks. Exact item IDs and installed-version mechanics must be validated against the actual 1.21.1 jars before implementation.

## Create Nuclear — major expansion

Current coverage is only a few representative items. Replace that sampler with a real progression page.

Planned branches:

1. **Entry and safety** — uranium/lead discovery, radiation concept, anti-radiation equipment.
2. **Material processing** — coal dust → steel, lead, raw uranium → uranium powder/yellowcake/enriched material, graphite/graphene where relevant.
3. **Fuel** — graphite rod and uranium rod production; explain moderator/fuel roles where the installed version distinguishes them.
4. **Reactor components** — casing, reinforced glass, input/output, frame, core, cooler, controller.
5. **Reactor blueprint / assembly** — teach the multiblock and controller activation instead of merely asking for the controller item.
6. **First criticality** — an explicit “operate a reactor” milestone, preferably advancement/manual-check based if FTB cannot detect operation directly.
7. **Automation** — automate fuel preparation and feeding with Create machinery.
8. **Safety outcome** — optional informational quest on radiation/meltdown behavior rather than incentivizing a meltdown.

Version caution: Create Nuclear's published 1.21.1 line and newer V2 feature set diverge. Do not author quests for V2-only reactor tiers/coolants unless the actual installed jar exposes them.

## Create Aeronautics: Toolgun — major expansion

Current coverage only scratches the surface. Planned progression:

- survival structure tool / toolgun entry;
- magnetic gun and physical-structure manipulation;
- save a physical vehicle blueprint;
- print/reconstruct a saved vehicle;
- portable structure/vehicle containers;
- move / rotate / weld / delete operations;
- collision editing where available in survival/config;
- preserve multi-sublevel vehicles and supported constraints/ropes/wires as an advanced informational quest.

The page should be task-oriented (“save and reprint a small craft”) rather than a row of item acquisitions.

## Electricity suite — one section, multiple pages

### Create Crafts & Additions
Use as the bridge between Create rotational power and FE where appropriate. Cover rolling mill, connectors/wires, accumulator/storage, alternator and electric motor. Reuse existing AOC Andesite/Brass quests when they fit rather than duplicating them.

### Create: Electro Energetics
Restore the AOC chapter because it already has substantial progression. Audit it against the current 1.21.1 release and prune only unavailable items. Keep electric trains as a cross-link into Transportation.

### Create: Power Grid
Add a dedicated page focusing on its own electrical model rather than treating it as generic FE:

- first source/load circuit;
- wiring and measurement/diagnostics;
- generation and consumption;
- safe distribution;
- transformers/substations or equivalent voltage-management progression present in the installed version;
- motors / Create integration;
- larger grid milestone;
- advanced circuitry components as an optional branch rather than mandatory early progression.

The three electricity pages should share prerequisites and cross-links but not repeat the same “craft a wire” lesson three times.

## Create: Power Loader — medium expansion

Move out of generic FPB navigation and into Transportation.

- Andesite chunk loader: one-chunk entry point.
- Demonstrate loading/unloading and controls.
- Brass chunk loader: configurable 1x1–5x5 range.
- Train Station attachment.
- Train/contraption usage.
- Optional server-etiquette info quest explaining that chunk loading should be deliberate.

## Farmer's Delight — major expansion

Use a crop → ingredient → cooking-technique → meal structure.

### Crops and ingredients
Give cabbage, tomato, onion and rice explicit quests. Include how each is obtained/grown, with rice separated because its planting behavior differs from normal farmland crops. Add straw, tree/bark ingredients and other foundational ingredients only when they unlock recipes/mechanics.

### Farming systems
- organic compost → rich soil;
- crop harvesting / knife utility;
- baskets or storage interactions if useful;
- Farming for Blockheads market as a convenience branch rather than a replacement for learning crops.

### Kitchen progression
- knife;
- cutting board;
- skillet;
- cooking pot;
- stove / heat source;
- bowls/serving where relevant;
- representative recipes that exercise different mechanics.

### Addon integration
Keep Create Kitchen / Slice & Dice / Integrated Farming as automation follow-ups. Restore Some Assembly Required source quests because the mod is present again.

Do not create one quest for every food item. The requested density should come from specific crops/ingredients and meaningful cooking families, not recipe spam.

## Immortalers Delight — major page

This addon is large enough for a dedicated Farming & Food chapter with an archaeology cross-link.

Planned structure:

- find its ruins / obtain a Sniffer Egg where applicable;
- use Sniffer-based discovery;
- individual crop quests for Evolutcorn, Pearlip, Himekaido/Hibonberry, Leisamboo, Kwat Wheat, Alfalfa, Warped Laurel and other crops actually present in the installed 1.21.1 build;
- teach each unusual cultivation rule rather than grouping all seeds into one task;
- Sniffer brush / scent utility;
- Ancient Stove and Enchantal Cooler;
- representative processing branches (stewing, detoxification, drinks/tea, etc. according to installed recipes);
- Eternal Blade / notable ancient equipment as optional goals;
- a small set of showcase dishes, not 100+ food-item quests.

## Tide Fishing — major page

The Fishing Journal should be the backbone because fish profiles unlock by **catching** fish, not merely possessing them.

Planned branches:

- fishing journal;
- basic Tide fishing rod / bait / hook systems;
- line upgrades and hook upgrades present in the installed build;
- informational tools: depth meter, climate/weather/time utilities and Fish Finder;
- catch-category milestones rather than every fish species;
- rare/special rod branch;
- bucketable/live fish mechanic;
- cross-links to Ocean's Delight / Crabber's Delight / Aquamirae where catches feed cooking or exploration.

## Tom's Simple Storage — medium/major page

Expand the current three-item sampler into the actual storage network:

- Inventory Connector;
- connect multiple inventories;
- Storage Terminal;
- Crafting Terminal;
- Open Crate;
- Inventory Hopper;
- Filtered Connectors / item filters;
- Wireless Terminal;
- Create Contraption Terminals integration: demonstrate a terminal on a moving/assembled Create contraption if practical.

## Supplementaries — dedicated page

Move the current starter-page Supplementaries chain intact where useful (crank, wrench, spikes, cage, speaker, turn table, key, gold doors/trapdoors, sack, safe, notice board, blackboard). Re-layout it into small functional branches such as storage/security, redstone/utility, display/signage and decorative interaction.

## Better Archeology — dedicated page

Move archaeology content out of Getting Started. Planned progression:

- brush / suspicious-block basics;
- structures/discovery loop;
- artifact shards / restoration or crafting mechanics;
- the four existing totem quests (Radiance, Growth, Soul, Torrent) as a reward branch;
- Sniffer / archaeology links where relevant;
- cross-link Immortalers Delight rather than merging the two mods into one page.

## Aquamirae — major exploration page

The current Echo Compass sampler is insufficient.

- Ship Graveyard / biome discovery;
- Echo Compass;
- key materials such as Echo of the Ship Graveyard and Abyssal Amethyst;
- important weapons/tools (e.g. Poisoned Chakram, Divider, Whisper of the Abyss) if present in the installed version;
- Blinding Abyss armor branch;
- Mother of the Maze mini-boss;
- Shell Horn / Captain Cornelia progression;
- signature boss drops and completion milestone.

Boss kills should also surface in the Bosses section through dependencies/cross-links rather than duplicate full quest trees.

## Galosphere — medium page

Focus on why the player should explore its underground content:

- discover its cave biomes/resources;
- major ore/material progression;
- representative tools/equipment;
- unique mobs/interactions;
- Echo Altar (present in the 1.21.1 line) as a later milestone;
- Galosphere Trimming integration as optional equipment/collection content.

## Naturalist — compact/medium page

Do not make the player hunt every animal. Emphasize useful interactions and tools/equipment from the installed 2.x line (the current quest localization suggests at least Shellstone and Knapsack content). Exact 1.21.1 item enumeration must be validated from the jar before implementation.

Suggested shape:

- intro/discover several Naturalist animals;
- one or two interaction examples;
- tool/material branch;
- Shellstone / Knapsack and any other meaningful utility items;
- optional animal-observation milestones rather than exhaustive collection.

## Just Hammers — compact progression page

Teach area-of-effect mining clearly:

- entry 3x3x1 hammer;
- 3x3x3 tier;
- 5x5 family milestones;
- material progression (iron → diamond/netherite where useful);
- explain shift/safe-use behavior and the “stops at 1 durability” mechanic if it applies to the installed version.

Avoid one quest for every material × area combination.

## Immersive Armors — medium equipment page

Use armor-set milestones rather than 40 individual item quests. Introduce representative sets and explain their distinctive effects (e.g. Berserk, bounceback, spikes, divine protection), then branch toward a few playstyle-oriented sets. Full-set tasks are more meaningful than one quest per armor piece.

## Furniture mods — compact shared page

Provide at least one clear entry quest per furniture mod while keeping the page visually useful:

- Another Furniture — representative chair/table/shelf plus interactive features;
- Handcrafted — introductory furniture recipe / representative furnishing;
- Immersive Furniture — Artisan's Workstation, then optionally create/download a furniture piece;
- MultiBeds can live here or on its own compact bedding branch;
- Additional Lights / Redden's Stone Lanterns can be adjacent decorative-light branches.

## Macaw's mods — medium shared page

Use one `Macaw's Building` chapter with a visual branch for each installed module rather than eight nearly empty chapters:

- Bridges
- Doors
- Fences & Walls
- Paintings
- Roofs
- Stairs & Balconies
- Trapdoors
- Windows
- Macaw's Quark compatibility as informational/integration content

Each branch should ask for one or two representative pieces and explain the construction system; avoid every variant/wood type.

## Hang Glider — compact page/branch

- craft a Hang Glider;
- first glide / informational use quest;
- reinforced glider upgrade;
- optional travel challenge or link to Aeronautics/exploration.

## Grappling Hook — compact/medium branch

The exact installed `Grappling Hook Mod` project still needs confirmation. The mod list display name alone is ambiguous between multiple 1.21.1 projects. Do not author exact item IDs until the jar/project is verified.

If the installed mod is **Grappling Hook Mod: Skybound**, explicitly teach grappling onto Create contraptions/Sable airships because that integration is unusually relevant to FPBCraft.

## RPG Series — major new group

### Shared onboarding

- Spell Engine casting/HUD basics;
- Runes as spell ammunition where applicable;
- find a village Gazebo / Spell Binding Table;
- create/bind a first spell book;
- Jewelry gems and accessory slots;
- Skill Tree introduction and first points.

### Archer tree

- starter bow / class equipment;
- Archer spell book;
- tiered skills;
- class armor;
- quiver / ranged utility;
- Skill Tree specialization;
- Armory end-game upgrade;
- Arsenal/Relic goals relevant to ranged builds.

### Rogue tree

- dagger/sickle or other quick weapon entry;
- Rogue Manual and evasive/trick skills;
- dual-wield guidance where supported;
- class armor;
- Skill Tree specialization;
- Armory epic set;
- relevant Relics/Arsenal loot.

### Warrior tree

- heavy weapon entry (double axe/glaive/claymore family);
- Warrior Codex and class skills;
- class armor;
- Skill Tree specialization;
- Armory epic set;
- relevant Relics/Arsenal loot.

### Paladin tree

- mace/great hammer/claymore + shield style entry;
- Paladin spell book / protection skills;
- paladin armor;
- Skill Tree specialization;
- Justicar / Lightbringer end-game armor paths in current Armory versions;
- support-oriented relics.

### Priest tree

- holy stave/wand;
- Holy Book and healing/protection spells;
- priest armor;
- Skill Tree specialization;
- Avatar / Absolution end-game armor paths;
- party-support relics/jewelry.

### Wizard tree

The Wizards mod itself has three distinct schools, so make the page visibly split:

- Arcane: Tome of Arcane → tiered spells → robes/staff → Tempest/Astral end-game path;
- Fire: Tome of Fire → tiered spells → robes/staff → Scarlet/Smouldering path;
- Frost: Tome of Frost → tiered spells → robes/staff → Glacier/Rimeweave path.

### Shared end-game RPG content

- Armory: superior armor upgrades / upgrade crystals from end-game loot and bosses;
- Arsenal: 40+ epic loot-only weapons with passive spells — use acquisition milestones, not craft tasks;
- Jewelry: six gem discovery + representative rings/necklaces + rare loot pieces;
- Relics: representative trinkets and boss/dungeon loot;
- Skill Tree: specialization milestones, not every node.

Boss quests should link to the RPG end-game because Armory/Arsenal/Relics explicitly use boss and end-game loot.

## Boss content — major new section

FTB Quests should be the guided path, while **Boss Checklist** remains the exhaustive encyclopedia (model, drops, spawn info, defeated state).

Proposed boss quest pattern:

1. discovery / summoning requirement;
2. short preparation note unique to that encounter;
3. kill task where reliable;
4. signature drop/reward task;
5. link onward to gear or RPG progression unlocked by the drop.

Candidate boss families:

- Legendary Monsters — dedicated branch(es), organized by practical tier/biome/dimension rather than one giant row;
- Mowzie's Mobs — Ferrous Wroughtnaut, Frostmaw, Umvuthi/Sunbird, Tongbi and other meaningful encounters in the installed version;
- Aquamirae — Mother of the Maze and Captain Cornelia;
- Myths of the Sea — Leviathan, Kraken and other boss-class encounters validated from the installed version;
- Ender Dragon — explicitly reflect Ender Dragon Fight Remastered rather than a vanilla-only description;
- Wither — important because it gates Create Nuclear's controller via Nether Star in the current 1.21.1 recipe set;
- any additional bosses discovered from the runtime Boss Checklist registry.
