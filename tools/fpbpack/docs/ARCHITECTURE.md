# FPBPack application architecture

## Deployment unit

FPBPack is one product and one deployable service.

```text
Browser
   │
   ▼
FPBPack HTTP server :8787
   ├── /, /mods/, /updates/, /history/, /settings/  static GUI
   ├── /api/*                                      management API
   └── /healthz                                    service health
        │
        ├── live inventory + reconciliation
        ├── update discovery/cache
        ├── persistent FPBPack state
        ├── Minecraft + AutoModpack filesystem
        └── provider integrations
```

The GUI and API are same-origin. The GUI never owns update eligibility, dependency resolution, filesystem mutation, backup, restore, or provider policy.

## Serve mode is the application mode

`fpbpack serve` must be self-contained. Running the GUI must not require the administrator to run `inventory`, `catalog`, `doctor`, or `updates` first.

The production contract is:

```bash
fpbpack serve --server-root /server --state-dir /data
```

On startup FPBPack:

1. loads its durable application state from `state-dir`;
2. imports a legacy migration report once when state does not yet exist and one is available;
3. scans the live server/common and AutoModpack client-only mod directories;
4. refreshes exact provider identity that can be verified safely;
5. reconciles the live inventory against accepted management state;
6. computes diagnostics;
7. refreshes update candidates;
8. writes internal cache/debug snapshots;
9. serves the GUI and API.

Inventory and update JSON files are implementation artifacts/cache files, not required command-line inputs.

## Persistent state

The state directory is FPBPack-owned:

```text
/data/
├── state.json                 durable accepted management state + settings
├── inventory.json             generated cache/debug snapshot
├── updates.json               generated cache/debug snapshot
├── plans/
│   └── plan-<id>.json         deterministic persisted update plans
├── history/
│   └── <time>-plan-<id>.json  structured audit events
├── backups/
│   └── backup-<id>/
│       ├── manifest.json
│       └── files/...           verified copies of affected current JARs
└── cache/
    └── artifacts/
        └── <sha512>.jar        prefetched + hash-verified target artifacts
```

`state.json` is authoritative for durable management identity such as provider/project ownership and unmanaged/pinned artifacts. The old `migration-report.json` is only a bootstrap/import format.

A legacy migration report may be imported explicitly on the first run, or auto-discovered from supported legacy locations. Once imported, future starts use `state.json` and do not require the migration report.

## Plan readiness contract

Planning remains non-mutating with respect to the live Minecraft installation.

A persisted plan is only `ready` when:

1. every selected update has an exact provider target;
2. required Modrinth dependency additions/updates have an exact compatible target, recursively;
3. no dependency requirements conflict;
4. no blocking inventory/managed-file drift finding exists;
5. planned target paths do not collide with unrelated live artifacts, including unmanaged/pinned files;
6. every target artifact has been downloaded into FPBPack state and its SHA-512 verified;
7. every current JAR that would later be replaced has been copied into a linked restore point and re-hashed successfully.

Dependency additions are represented as explicit `add` operations. Installed dependency upgrades and requested updates are explicit `replace` operations.

A ready plan still does **not** imply that Apply is permitted. Slice 3 must re-check the live state, verify the plan has not gone stale, and require the appropriate server state immediately before any live mutation.

## Refresh model

The service owns refreshes.

- startup performs an initial inventory/update refresh;
- the GUI can request an inventory/update refresh through the API;
- serve mode may periodically refresh read-only provider/update data;
- refreshes never mutate live mod JARs.

Standalone CLI commands remain available for diagnostics, scripting, migration, and development, but are not prerequisites for GUI operation.

## Repository layout

```text
tools/fpbpack/
├── cmd/fpbpack/
├── internal/
│   ├── httpapi/
│   ├── service/        long-running application state/refresh owner
│   ├── webui/
│   │   └── dist/       generated static export staged before release builds
│   └── ...
├── web/                Next.js static-export source
├── Dockerfile
└── go.mod
```

## Frontend runtime

Next.js is used only as a static build tool. Production does not run a Next.js server or Node process.

The frontend fetches relative routes such as `/api/mods`. Release builds copy `web/out` into `internal/webui/dist` and Go's `embed` package places those assets in the FPBPack binary.

## Docker / Unraid

The Dockerfile is multi-stage and is built by automation only for published full releases:

1. Node builds/typechecks the static GUI.
2. Go compiles FPBPack with those files embedded.
3. The runtime image contains the FPBPack binary and CA certificates only.

The target container mounts:

- `/server` — Minecraft/Crafty server root;
- `/data` — FPBPack durable state/cache/history/backups.

The container exposes port 8787 and runs one process. The image defaults to `fpbpack serve`, so normal container startup needs no command override and no pre-start inventory/update job.

PR/dev CI does not build Docker images. The release workflow builds and smoke-tests the image only when a GitHub release is published.

## Development

Backend unit tests remain independent of Node. The web handler accepts an `fs.FS` for testing, while the CLI injects the embedded production assets.

A developer can run `fpbpack serve --web-dir web/out` after a frontend build to test without rebuilding the binary for every static change.

## Repository decision

The former `fpbcraft-gui` repository is superseded by `fpbcraft/tools/fpbpack/web`. New GUI/API work is implemented together in the FPBPack branch so wire-contract changes, tests, and releases stay synchronized.
