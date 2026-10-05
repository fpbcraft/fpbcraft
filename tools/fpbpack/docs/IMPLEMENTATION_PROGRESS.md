# FPBPack implementation progress

Last updated: **2026-10-05**

## Active slice

**Slice 2 — Plan & Protect: in progress**

Working branch: `fpbcraft/fpbcraft:feat/plan-protect` → PR #10 against `main`.

PR #9 (`feat/self-contained-serve`) has been merged; PR #10 is now independently reviewable against `main`.

The user merges PRs manually. Do not merge these branches automatically.

## Architecture / runtime completed

- [x] FPBPack serves the embedded GUI and same-origin API from one process.
- [x] `serve` is self-contained; inventory/update JSON are generated caches, not required inputs.
- [x] Durable `state.json` under `--state-dir`.
- [x] Legacy migration report is a one-time bootstrap/import source.
- [x] Serve starts immediately from durable state/caches; inventory/provider refresh runs in the background.
- [x] Cached GUI/API state remains available while a background refresh is running.
- [x] Provider discovery uses bounded concurrency instead of serial per-project requests.
- [x] Multi-stage production Dockerfile with no Node runtime.
- [x] PR/dev CI does not build Docker images.
- [x] Stable published releases build and smoke-test the Docker image.
- [x] Separate GUI repository/PR is superseded by `tools/fpbpack/web`.

## Discover, Decide & Review capabilities included in Slice 2

The remaining Slice 1 decision/review work is intentionally being completed in PR #10 instead of being carried into a later slice.

- [x] Diagnostics and managed-file drift detection.
- [x] Safe / Review / Blocked / Ignored update model and real GUI update data.
- [x] Modrinth discovery with Minecraft/NeoForge compatibility filtering.
- [x] CurseForge candidate discovery through the official API when `FPBPACK_CURSEFORGE_API_KEY` is configured.
- [x] Verified GitHub release discovery for artifacts already mapped to an explicit GitHub release source.
- [x] Recursive required Modrinth dependency resolution with explicit target artifacts.
- [x] Required CurseForge dependency additions can be resolved conservatively.
- [x] Incompatible dependencies and unsafe unresolved requirements block candidates.
- [x] Reverse-dependency metadata is exposed where provider dependency metadata is available.
- [x] Target + intermediate changelogs are aggregated for Modrinth, CurseForge, and verified GitHub release sources.
- [x] Modrinth/CurseForge/GitHub project links are exposed directly in the GUI.
- [x] Persistent pin-current / ignore-version / ignore-mod / review-later rules live in `state.json`.
- [x] Updates rows expose changelogs, provider links, project icons, and decision actions.
- [x] Mods page shows installed/latest/status and opens a responsive detail surface.
- [x] Mod detail shows provider link, changelogs, dependencies, reverse dependencies, update preference controls, path and provider identifiers.
- [x] The six custom BlueMap/FPBCraft artifacts remain explicitly pinned/unmanaged and are never guessed into an update source.

### Provider caveats

- CurseForge update discovery requires a CurseForge API key. Without one, CurseForge-managed artifacts remain explicitly Blocked with a configuration reason rather than being guessed.
- GitHub release updates are only considered for artifacts that FPBPack already verified against an explicit GitHub release source. GitHub candidates require an unambiguous JAR asset and provider SHA-256 digest, and remain Review because GitHub releases do not declare Minecraft/loader compatibility.
- GitHub authentication is optional via `FPBPACK_GITHUB_TOKEN` and is useful for rate limits/private verified sources.

## Slice 2 — Plan & Protect completed

### Deterministic planning

- [x] Select Safe/Review candidates from the real Updates page.
- [x] Persist deterministic plans under `/data/plans`.
- [x] Include requested changes and dependency-driven changes.
- [x] Record old/new versions, provider IDs, exact target filenames, URLs, and provider checksums; normalize prefetched artifacts to SHA-512.
- [x] Record exact add/replace filesystem operations and deployment location.
- [x] Coalesce duplicate dependency requirements.
- [x] Block conflicting dependency target versions.
- [x] Block target-path collisions, including collisions with unmanaged/pinned artifacts.
- [x] Carry blocking inventory/drift diagnostics into plan readiness.

### Prefetch and verification

- [x] Prefetch every ready-plan target into `/data/cache/artifacts`.
- [x] Verify Modrinth SHA-512, CurseForge SHA-1, or GitHub SHA-256 during prefetch, then compute/persist FPBPack SHA-512.
- [x] Reuse cached artifacts only after re-verifying their hash.
- [x] Block the plan when an artifact cannot be downloaded or verified.
- [x] Bound individual artifact downloads with a 2 GiB safety limit.

### Protection / restore points

- [x] Create a restore point before a plan can be considered protected.
- [x] Back up only current files that the plan would replace.
- [x] Verify current-file SHA-512 before backup.
- [x] Verify copied backup SHA-512.
- [x] Persist a backup manifest linked to the plan.
- [x] A cached plan is reusable only if its required restore-point manifest still exists.
- [x] No live mod JAR is mutated.

### Persistent history/settings

- [x] Persist plan audit events under `/data/history`.
- [x] History page reads structured operation records.
- [x] Persist backup/history retention in `state.json`.
- [x] Default retention is 20; valid range is 1–100.
- [x] Settings GUI can update retention.
- [x] Pruning removes old plan records and their linked restore points.
- [x] Future non-plan operation types are deliberately not pruned by the Slice 2 logic.

### Review UI

- [x] Mandatory persisted review screen between selection and future Apply.
- [x] Show requested vs dependency-driven changes.
- [x] Show warnings and blockers.
- [x] Show exact artifacts, hashes, and filesystem operations.
- [x] Show artifact-verification status.
- [x] Show restore-point ID.
- [x] Apply remains disabled until Slice 3.

## Visual-system refresh

- [x] Tailwind CSS 4.
- [x] daisyUI 5 with a custom FPBPack theme.
- [x] Lucide icons.
- [x] Compact active-navigation shell.
- [x] Flat, low-contrast surfaces with thin borders and restrained accent usage.
- [x] Dense tables/lists inspired by self-hosted admin tools rather than decorative dashboard cards.
- [x] Overview, Updates, Mods, Review, History, and Settings migrated to the new system.
- [x] Next.js remains a static export embedded in the Go binary.

## Remaining before Slice 2 can be called complete

- [ ] Latest PR #10 head must pass GUI build, Go test/vet, static binary build, and artifact upload.
- [ ] Exercise the revised fast-start/background-refresh behavior on the real FPBCraft server.
- [ ] Exercise Modrinth changelog/rule/detail flows against the real inventory.
- [ ] Configure/test CurseForge discovery on the real pack if a CurseForge API key is available.
- [ ] Resolve any real-pack dependency/provider edge cases exposed by those tests.

## Safety checkpoint

Slice 2 may write only FPBPack-owned state/cache/history/backup files under the state directory. It may read and hash live Minecraft mod JARs to inventory them and create restore points, but it does **not** add, replace, move, or delete live mod JARs.

Docker images are built and smoke-tested only for stable published releases, not PR/dev builds.
