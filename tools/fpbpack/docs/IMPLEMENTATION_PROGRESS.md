# FPBPack implementation progress

Last updated: **2026-10-05**

## Active slice

**Slice 1 — Discover & Decide: in progress**

Working branch: `fpbcraft/fpbcraft:feat/self-contained-serve` → draft PR #9.

The user merges PRs manually. Do not merge this branch automatically.

## Architecture migration

- [x] Decide that FPBPack serves both GUI and API.
- [x] Move the active GUI source into `tools/fpbpack/web`.
- [x] Convert the GUI contract to same-origin `/api/*` requests.
- [x] Configure Next.js as a static export.
- [x] Add a Go `embed` web UI package without making backend unit tests require Node.
- [x] Add a `--web-dir` development override.
- [x] Remove the CORS / Local Network Access requirement from the new architecture.
- [x] Add a multi-stage Docker build with no Node runtime in the final image.
- [x] Keep PR/dev CI focused on GUI typecheck/export, Go test/vet, and static Linux binary artifacts.
- [x] Add a release-only Docker workflow triggered by published GitHub releases.
- [x] Mark the separate GUI repository/PR as superseded after the combined branch has been accepted.

## Self-contained serve mode

- [x] Define `serve` as the normal application/GUI mode.
- [x] Define inventory/update JSON as generated internal cache/debug artifacts, not required inputs.
- [x] Define migration report as a one-time bootstrap/import format rather than permanent runtime state.
- [x] Add durable `state.json` ownership under `--state-dir`.
- [x] Make `serve` automatically scan/reconcile the live server.
- [x] Make `serve` automatically refresh update discovery and caches.
- [x] Add GUI/API-triggered refresh endpoints.
- [x] Change Docker defaults to `/server` + `/data` with no pre-generated JSON requirement.

## Backend / FPBPack completed

- [x] Diagnostics and managed-file drift detection.
- [x] Shared read-only management state.
- [x] Read-only status, inventory, mods, diagnostics, update-report, and health endpoints.
- [x] First Modrinth update discovery and compatibility classification.
- [x] Cached update report support.

## Remaining in Slice 1

- [ ] Finish provider-aware candidate discovery, including CurseForge.
- [ ] Complete required dependency closure and reverse-dependency metadata.
- [ ] Project icons, provider links, release metadata, and changelog aggregation.
- [ ] Durable pin/ignore/review-later state.
- [ ] Connect real update candidates and counts to the GUI.
- [ ] Safe / Review / Blocked / Ignored selection workflows.
- [ ] Mod detail modal/sheet and changelogs.
- [ ] Finish mobile feature parity.

## Safety checkpoint

The current service remains read-only. Serve-mode refreshes scan/reconcile live files and update FPBPack-owned state/cache data, but do not mutate live mod JARs. Docker images are built and smoke-tested only for published releases, not PR/dev builds.
