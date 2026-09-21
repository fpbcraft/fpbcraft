# Phase 5 result — Building, Adventure, Combat

Phase 5 is complete across the three planned slices.

## 5A — Building & Decoration

Implemented and/or integrated:

- Supplementaries remains a dedicated Building & Decoration page with the existing useful quest chain preserved.
- Furniture remains a shared page for Another Furniture, Handcrafted, Immersive Furniture, MultiBeds, Additional Lights, Redden's Stone Lanterns and Measurements.
- Added a dedicated **Macaw's Building** page with branches for Bridges, Doors, Fences & Walls, Paintings, Roofs, Stairs & Balconies, Trapdoors and Windows.
- Kept the building pages focused on representative systems and palettes instead of exhaustive material-variant collection.

## 5B — Adventure & Exploration

Implemented and/or integrated:

- Better Archeology remains in its dedicated Adventure & Exploration page with its existing artifact/totem progression.
- Added a compact/medium **Galosphere** progression covering cave exploration, materials, mobs/interactions and advanced content such as the Echo Altar.
- Added a compact **Naturalist** page focused on wildlife observation, Shellstone and useful gear rather than hunting every animal.
- Exposure photography remains in Adventure & Exploration and preserves the camera → film → Lightroom → album path plus Expanded/Polaroid branches.
- Added **Hang Glider** onboarding, first-flight, reinforced-flight and travel milestones.
- Existing travel/exploration utility pages remain separate so this slice does not create a second catch-all chapter.

## 5C — Combat & Equipment

Implemented and/or integrated:

- Simply Swords remains preserved intact in the Combat & Equipment group.
- Added **Just Hammers** with area-mining guidance for shallow 3×3, deeper 3×3×3 and large-area use, plus safe-use guidance.
- Added **Immersive Armors** as set/playstyle progression rather than one quest per armor piece.
- Added **Grappling Hook Mod: Skybound** with:
  - basic movement;
  - Smithing Table upgrade progression;
  - motor/rocket/advanced functional upgrade branches;
  - Long Fall Boots / fall-safety guidance;
  - a Create contraption grappling milestone;
  - a Sable / Create Aeronautics airship milestone;
  - a final moving-structure mastery milestone.

FTB Quests cannot reliably detect several of the movement/interaction states above, so those are deliberately manual checkmark milestones with explicit instructions instead of fragile item-only proxies.

## Localization / authoring pass

The initial Phase 5 structural commits created the new chapter files but did not add their authored localization. Phase completion adds titles and descriptions for the new pages/quests to both:

- `ftbquests/quests/lang/en_US.snbt`
- `ftbquests/quests/lang/en_us.snbt`

Both locale files intentionally remain synchronized.

## Design rules retained

- Reuse good existing source layouts/content rather than replacing working pages for novelty.
- Prefer representative mechanics over exhaustive item checklists.
- Keep optional building/decor choices from blocking unrelated progression.
- Keep exploration pages discovery-oriented.
- Keep Combat & Equipment distinct from the upcoming RPG Series class trees.
- Use manual milestones where FTB Quests lacks reliable runtime detection rather than inventing misleading acquisition tasks.

## Next phase

Phase 6 is the RPG Series workstream: shared onboarding, then separate Archer, Rogue, Warrior, Paladin, Priest and Wizard trees, followed by shared Armory / Arsenal / Jewelry / Relics end-game integration.


## Item-requirement revision

After the first Phase 5 implementation, the new pages were audited for excessive manual/checkmark tasks. The phase now follows a stricter rule: if a meaningful representative item exists, the quest requires that item. Manual completion is reserved for actions that FTB Quests cannot reliably detect.

Reworked pages:

- **Just Hammers** now requires the actual 3×3×1, 3×3×3, 5×5, 5×5×3 and 5×5×5 hammer progression, ending with a Diamond Destructor Hammer.
- **Immersive Armors** now requires the full four-piece Wooden, Heavy, Slime, Divine, Steampunk and Wither armor sets.
- **Galosphere** now requires Allurite, Pink Salt, Palladium, Opal, an Echo Altar and Sterling equipment.
- **Naturalist** now requires concrete mod items including the Knapsack and Shellstone.
- **Hang Glider** now requires a Glider Wing, Hang Glider and Reinforced Hang Glider; only the actual flight challenge remains manual.
- **Skybound** now requires the Grappling Hook and functional upgrade items including Motor, Rocket, Double Hook, Long Fall Boots, Magnet, Forcefield, Ender Staff and Hook Thrower upgrades. Manual checks remain only for swinging, hooking a moving Create contraption and boarding an Aeronautics/Sable airship.
