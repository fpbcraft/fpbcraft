# FPBPack implementation progress

Last updated: **2026-10-06**

## Active slice

**Slice 5 — Complete AutoModpack Integration: implementation in PR #20**

Working branch: `fpbcraft/fpbcraft:feat/automodpack-integration` → draft PR #20 against `main`.

Slices 1–4, the operational/reconciliation follow-up, release automation, and NeoForge runtime management are merged. The user merges PRs manually; do not merge this branch automatically.

### Slice 5 implementation

- [x] Treat AutoModpack `server.conf` as the authoritative configuration instead of duplicating it into FPBPack state.
- [x] Parse/edit AutoModpack's documented HOCON subset while retaining unknown settings and unknown group fields.
- [x] SHA-256 guard config writes so a manual edit made after the GUI loaded cannot be silently overwritten.
- [x] Back up `server.conf` before every FPBPack config mutation and retain backups using FPBPack's retention setting.
- [x] Discover every `automodpack/host-modpack/<group>/mods` directory instead of scanning only `main`.
- [x] Carry AutoModpack group identity through inventory, accepted catalog, update candidates, plans, Apply, Restore, and the GUI.
- [x] Backwards-compatibly interpret existing `client` entries without a group as `AutoModpack/main`.
- [x] Support protected group-to-group moves through Review → Apply → Restore.
- [x] Let catalog installs and exact-version changes target a selected AutoModpack group.
- [x] Inherit the requesting client mod's group for newly planned client dependencies unless an explicit group already exists.
- [x] Add AutoModpack group filtering/current/preferred group controls to Mods.
- [x] Show the target AutoModpack group in Review.
- [x] Add a dedicated AutoModpack GUI with installation/config status, diagnostics, general settings, advanced host/security settings, and group/category editing.
- [x] Validate duplicate/invalid group identities, missing/self dependencies, dependency cycles, `requires` + `breaks-with` contradictions, and invalid platform names.
- [x] Diagnose configured-vs-directory drift, managed artifacts targeting missing groups, direct cross-group content collisions, and AutoModpack self-updater ownership conflicts.
- [x] Require explicit confirmation for category/group identity-changing edits.
- [x] Add an explicit group-ID migration operation that updates config references, renames the group directory, and migrates FPBPack catalog/source identities together.
- [x] Track FPBPack changes that have not yet been published to AutoModpack clients.
- [x] Send `config reload`, `host restart`, generation preview/publish, group summary, host activity, and generation rollback commands through the existing Crafty connection.
- [x] Read AutoModpack's append-only `automodpack/server/journal.jsonl` and expose generation notes, change counts, restore lineage, preview rollback, and confirmed rollback in the GUI.
- [x] Add parser round-trip, group validation, multi-group inventory, group-to-group planning, config concurrency/backup, journal, and HTTP API regression coverage.
- [x] Parse real AutoModpack/Reconf config syntax including bare URLs/globs, colon-less objects, and quoted empty values without confusing value colons with assignment separators.
- [x] Keep publication pending after Crafty accepts generate/revert; clear it only after AutoModpack's durable journal confirms a newer generation, including automatic generate-on-start.
- [x] Show server.conf read-only by default with an explicit edit toggle and floating Save/Discard controls for pending edits.
- [x] Provide both structured form editing and a validated raw server.conf editor with optimistic concurrency protection.
- [x] Keep ordinary AutoModpack config saves online; require a stopped server only for filesystem-mutating operations such as group-ID migration and live mod moves.
- [x] Replace comma-delimited group rule fields with add/remove entry controls for loaders, requires, conflicts, platforms, from-server, exclude, and editable patterns.
- [x] Replace the embedded 500-file limit with lazy, searchable, paginated published-content browsing.
- [x] Capture and show terminal output for AutoModpack operations issued through Crafty, with an explicit fallback when terminal permission/output is unavailable.
- [x] Replace the opaque rollback-preview command with an in-GUI journal-derived head-to-target file diff before rollback publication.
- [x] Move generation history below the primary configuration and operations workflow.
- [x] Get the complete Slice 5 branch green in FPBPack CI after the final frontend/backend integration.
- [ ] Exercise config save/reload against the real FPBCraft AutoModpack installation.
- [ ] Exercise one new optional group and one `main → optional group` protected move.
- [ ] Preview then publish a real generation and verify the pending-publication indicator clears correctly.
- [ ] Preview and confirm one generation rollback against the real server.
- [ ] Capture real-server edge cases for the follow-up **Slice 5.1 — AutoModpack UX & hardening** pass.

## Architecture / runtime completed

- [x] FPBPack serves the embedded GUI and same-origin API from one process.
- [x] `serve` is self-contained; inventory/update JSON are generated caches, not required inputs.
- [x] Durable `state.json` under `--state-dir`.
- [x] Legacy migration report is a one-time bootstrap/import source.
- [x] Serve starts immediately from durable state/caches with **no startup refresh**; the first automatic refresh waits for the configured interval and manual refresh remains available.
- [x] Cached GUI/API state remains available while a background refresh is running.
- [x] Manual refresh/check jobs are server-owned, survive browser disconnects/page reloads, deduplicate active refreshes, and never persist cancelled partial reports.
- [x] Provider discovery uses bounded concurrency instead of serial per-project requests.
- [x] Provider HTTP calls are paced per provider/refresh mode and retry 429/408/5xx responses with Retry-After / rate-limit reset handling and bounded exponential fallback.
- [x] Truncated HTTP-200 JSON bodies are retried atomically instead of publishing partially decoded provider metadata; exhausted retries report an incomplete provider response clearly.
- [x] Broad Modrinth version history omits changelog payloads; changelogs are hydrated only for the configured Minecraft/loader versions to reduce response size without losing rejected-version visibility.
- [x] Scheduled automatic provider refresh uses a deliberately slower background policy; user-triggered checks use a faster interactive policy. Startup itself performs no provider refresh.
- [x] Provider clients pace requests globally and honor Retry-After / rate-limit reset headers with bounded exponential backoff.
- [x] Failed provider metadata refreshes preserve the last-good target/changelog/dependency/project metadata and mark it stale instead of flushing it.
- [x] Multi-stage production Dockerfile with no Node runtime.
- [x] PR/dev CI does not build Docker images.
- [x] Stable published releases build and smoke-test the Docker image.
- [x] Separate GUI repository/PR is superseded by `tools/fpbpack/web`.

## Discover, Decide & Review capabilities included in Slice 2

The remaining decision/review hardening is being completed in PR #13 instead of being carried into Apply & Restore.

- [x] Diagnostics and managed-file drift detection.
- [x] Safe / Review / Blocked / Ignored update model and real GUI update data.
- [x] Modrinth discovery with Minecraft/NeoForge compatibility filtering.
- [x] CurseForge candidate discovery through the official API using either a GUI-managed key or `FPBPACK_CURSEFORGE_API_KEY`.
- [x] Verified GitHub release discovery for artifacts already mapped to an explicit GitHub release source.
- [x] Recursive required Modrinth dependency resolution with explicit target artifacts.
- [x] Required CurseForge dependency additions can be resolved conservatively.
- [x] Incompatible dependencies and unsafe unresolved requirements block candidates.
- [x] Reverse-dependency metadata is exposed where provider dependency metadata is available.
- [x] Target + intermediate changelogs are aggregated for Modrinth, CurseForge, and verified GitHub release sources.
- [x] Modrinth/CurseForge/GitHub project links are exposed directly in the GUI.
- [x] GUI-managed CurseForge credentials are validated before save, stored separately in `secrets.json` with `0600` permissions, never echoed back, and override the environment fallback.
- [x] Persistent pin-current / ignore-version / ignore-mod / review-later rules live in `state.json`.
- [x] Updates rows expose changelogs, provider links, project icons, and decision actions.
- [x] Mods page shows installed/latest/status and opens a responsive detail surface.
- [x] Mod detail shows provider link, changelogs, dependencies, reverse dependencies, update preference controls, path and provider identifiers.
- [x] Blocking diagnostics are actionable: Updates links to Mods → Needs attention, blockers can open the affected mod, and missing accepted entries can be forgotten explicitly.
- [x] Mod detail can auto-detect metadata, mark an artifact intentionally unmanaged, or assign an exact verified Modrinth, CurseForge, or GitHub source.
- [x] Per-mod metadata refresh uses server-owned background work and preserves last good metadata on transient provider failure.
- [x] CurseForge files that prohibit third-party direct download remain Review candidates with a manual CurseForge file link rather than becoming permanently blocked.
- [x] Updates blocking-diagnostics banner links directly to Mods → Needs attention.
- [x] Needs-attention view exposes blocker-specific remediation instead of dead-end diagnostics.
- [x] Live mod details can refresh metadata for only that artifact, explicitly mark it unmanaged, or assign a verified GitHub release source.
- [x] Missing accepted catalog entries can be explicitly forgotten without touching live files.
- [x] Verified GitHub source assignment requires the current JAR SHA-256 to match the selected release asset digest.
- [x] CurseForge files that prohibit third-party direct downloads remain Review candidates with a manual CurseForge file link instead of being treated as permanently unavailable.
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
- [x] Treat a dependency target already occupied by SHA-512-identical managed bytes as already satisfied, including when the installed JAR is managed under another provider identity; different/unmanaged occupants still block.
- [x] Carry blocking inventory/drift diagnostics into plan readiness.

### Prefetch and verification

- [x] Prefetch every ready-plan target into `/data/cache/artifacts`.
- [x] Verify Modrinth SHA-512, CurseForge SHA-1, or GitHub SHA-256 during prefetch, then compute/persist FPBPack SHA-512.
- [x] Reuse cached artifacts only after re-verifying their hash.
- [x] Block the plan when an artifact cannot be downloaded or verified.
- [x] Manual-download provider artifacts remain reviewable, retain their provider/manual URL in the plan, and stay unappliable until Slice 3 can accept and verify a user-supplied artifact.
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

- [x] Merged PR #10 passed GUI build, Go test/vet, static binary build, and artifact upload.
- [ ] PR #13 must pass the same validation for remediation/rate-limit/manual-download hardening.
- [x] Refresh progress exposes phase/current/total/percentage and is shown globally in the GUI.
- [x] Safe catalog remediation can run during provider refresh; conflicting provider/source operations fail fast instead of hanging.
- [x] Tools page exposes inventory, doctor-style diagnostics, updates, full refresh, catalog/state JSON, and version/status through the same service layer.
- [x] Logs page is available from the left navigation with a bounded structured runtime event buffer.
- [x] Intentional manual JAR replacements can be adopted only after same-source provider verification.
- [x] Post-Apply empty update reports serialize stable empty arrays and the GUI defensively normalizes legacy/null collections.
- [ ] Exercise the revised scheduled/manual refresh behavior and Adopt current JAR against the real FPBCraft server.
- [ ] Exercise Modrinth changelog/rule/detail flows against the real inventory.
- [ ] Configure/test CurseForge discovery on the real pack if a CurseForge API key is available.
- [ ] Resolve any real-pack dependency/provider edge cases exposed by those tests.

## Safety checkpoint

Slice 2 may write only FPBPack-owned state/cache/history/backup files under the state directory. It may read and hash live Minecraft mod JARs to inventory them and create restore points, but it does **not** add, replace, move, or delete live mod JARs.

Docker images are built and smoke-tested only for stable published releases, not PR/dev builds.
