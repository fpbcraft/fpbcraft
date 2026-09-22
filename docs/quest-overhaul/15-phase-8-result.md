# Phase 8 result — Book-wide polish and QA

Status: **complete for static/CI validation**.

Phase 8 audited the complete working quest pack rather than only the chapters added in the later phases.

## Validator

The existing `scripts/validate_ftbquests.py` / **Validate FTB Quests** GitHub Actions workflow was expanded into the permanent book-wide integrity check.

It now validates:

- chapter filename fields;
- chapter IDs and group references;
- globally unique 16-character object IDs;
- globally unique quest IDs;
- dependency targets;
- dependency cycles;
- quest coordinates and exact overlaps;
- detectable task presence and task-type counts;
- synchronization of `en_US.snbt` and `en_us.snbt`;
- malformed literal `\\t` / `\\n` localization text;
- missing chapter / quest / group titles;
- orphan localization IDs;
- optional-to-required dependency edges, with reviewed intentional edges explicitly allowlisted.

The final Phase 8 CI run passes with **0 errors and 0 warnings**.

## Final static inventory

The validated quest book contains:

- **62 chapter files**;
- **939 quests**;
- **1,226 tasks**;
- **1,053 item tasks**;
- **42 advancement tasks**;
- **6 kill tasks**;
- **2 observation tasks**;
- **123 manual checkmark tasks**.

The checkmark count includes preserved source/tutorial/action quests. Phase 5–7 content follows the stricter project rule established during the overhaul: use concrete item, advancement, kill or observation detection when it is reliable; reserve manual completion for interactions or authored source milestones that FTB Quests cannot reliably observe.

## Localization cleanup

Phase 8 fixed two large localization issues:

- authored Phase 5–7 entries contained **74 literal `\\t` sequences** instead of real indentation tabs; these were normalized in both English locale files;
- **376 orphan localization IDs** left behind by removed/moved source quests were removed.

The two English localization files remain byte-identical.

The audit also restored authored titles/descriptions for **16 live quests** that had no localized or inline title, covering:

- Create: Things and Misc locomotive additions;
- Electro Energetics tools/switchgear;
- MultiBeds components;
- Better Archeology artifact progression.

## Dependency cleanup

All quest dependencies resolve and the dependency graph has no detected cycles.

Ten required quests intentionally depend on optional overview/discovery nodes. These were reviewed individually and recorded in the validator allowlist because they only gate their own related branch; they do not block unrelated progression.

The Macaw's Building capstone also had one semantic omission that a generic graph validator could not infer: **Paintings** was the only module branch not included in its dependency list. Phase 8 added it, so the Architectural Toolkit capstone now represents all eight Macaw module branches.

## Layout checks

The validator checks for exact quest-coordinate collisions within each chapter. The final run reports none.

This is a structural collision check, not a substitute for visual review inside the FTB Quests UI. Dense or unusually shaped pages—especially the radial RPG class page—should still be viewed in-game after pack updates.

## Item / entity / advancement IDs

IDs introduced during the authored overhaul were checked against the relevant 1.21.1 mod sources while their phases were implemented, including RPG Series, Mowzie's Mobs, Legendary Monsters, Aquamirae and Myths of the Sea.

The repository does not contain a runtime Minecraft registry dump, so CI cannot prove that every historical/source-pack item ID resolves in the exact installed server instance. A real client/server launch remains the authoritative registry-level test.

## Rewards

No book-wide reward rebalance was performed. Reward philosophy remains an explicit pack-owner decision in `06-open-questions.md`; Phase 8 preserves existing/source rewards rather than inventing a new economy during QA.

## Test-pack artifact

The quest validation workflow now packages `ftbquests/quests/` as `fpbcraft-quests.zip` after every successful validation run and publishes it as a GitHub Actions artifact for 30 days.

This makes each future quest slice directly testable from the validation run without requiring a separate private-repository ZIP export.

## Runtime verification

Static Phase 8 QA is complete. The remaining operational check is to load the updated quest pack in the actual Minecraft instance/server and verify:

- all chapters open without FTB Quests parse errors;
- representative item/advancement/kill tasks complete in practice;
- manual action milestones read clearly;
- dense layouts are visually comfortable at normal zoom;
- no installed-mod registry mismatch appears in the game log.

That runtime check cannot be simulated by the repository-only CI validator.
