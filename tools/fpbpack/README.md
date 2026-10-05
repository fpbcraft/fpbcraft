# fpbpack

`fpbpack` is the FPBCraft modpack inventory/update helper.

It understands the current FPBCraft layout:

- `mods/*.jar` — server/common mods
- `automodpack/host-modpack/main/mods/*.jar` — AutoModpack client-only mods

## Inventory

`fpbpack inventory` is intentionally **read-only**. It inventories the live Crafty server without changing, deleting, moving, or downloading any mod JARs.

For each JAR it records SHA-1/SHA-512 hashes and the CurseForge Murmur2 fingerprint, reads NeoForge/Forge metadata (and Fabric metadata as a fallback for Connector-hosted mods), and performs an exact SHA-512 lookup against Modrinth's version-file API. CurseForge resolution is intentionally deferred to the catalog stage and delegated to upstream Packwiz.

```bash
./fpbpack inventory \
  --server-root /path/to/crafty/server \
  --json fpbpack-inventory.json
```

To skip the Modrinth lookup and generate a local-only inventory:

```bash
./fpbpack inventory --server-root /path/to/crafty/server --offline
```

If your paths differ from the defaults:

```bash
./fpbpack inventory \
  --server-root /path/to/server \
  --server-mods mods \
  --client-mods automodpack/host-modpack/main/mods
```

## Catalog migration

`fpbpack catalog` converts a verified inventory JSON into a Packwiz catalog without touching the live server. It can optionally use the bundled upstream Packwiz helper to identify CurseForge-only JARs and an explicit SHA-512 source registry for GitHub/custom artifacts.

```bash
./fpbpack catalog \
  --inventory fpbpack-inventory.json \
  --output modpack
```

To identify CurseForge-only JARs during migration:

```bash
./fpbpack catalog \
  --inventory fpbpack-inventory.json \
  --output modpack \
  --resolve-curseforge \
  --packwiz ./packwiz-linux-amd64
```

This does **not** run Packwiz against the live server directories. FPBPack copies only unresolved JARs into the generated catalog workspace, runs `packwiz curseforge detect` there, removes any unmatched temporary copies, refreshes the Packwiz index, and records successful CurseForge matches in `migration-report.json`. The original JARs remain untouched.

After provider detection, resolve known GitHub/custom artifacts with the checked-in source registry:

```bash
./fpbpack catalog \
  --inventory fpbpack-inventory.json \
  --output modpack \
  --resolve-curseforge \
  --packwiz ./packwiz-linux-amd64 \
  --sources ./fpbpack-sources.json
```

Source registry entries are keyed by the installed JAR's exact SHA-512. A `github_release` entry is accepted only after FPBPack downloads the declared release asset and verifies that its SHA-512 is byte-identical to the installed JAR. A `pinned_local` entry explicitly accounts for a custom or historical artifact that should be preserved but does not yet have a verified update source. Neither mode mutates the live server.

For the current FPBCraft migration, the six BlueMap/custom artifacts are intentionally configured as `pinned_local`. They remain visible in `migration-report.json`, but FPBPack generates no Packwiz metafiles for them, so `packwiz update` cannot update, replace, or remove them.

The generator is conservative:

- byte-identical JARs found in both locations are represented once;
- if the same Modrinth project has multiple installed versions, all versions for that project are withheld from the generated Packwiz catalog and reported as a conflict;
- JARs without an exact Modrinth match are initially unresolved; optional Packwiz detection can convert exact CurseForge matches without filename guessing;
- remaining custom artifacts can only be resolved by an explicit SHA-512 registry rule; GitHub release rules are re-hashed before acceptance and pinned-local rules remain intentionally non-updatable;
- the original deployment location (`server` or `client`) is preserved separately from Packwiz `side` metadata;
- Modrinth environment metadata that disagrees with the current deployment location is reported as a warning only; it never moves a live JAR;
- generated output is deterministic and written separately from the Crafty server.

The output contains:

```text
modpack/
├── pack.toml
├── index.toml
├── migration-report.json
└── mods/
    └── <modrinth-project-id>.pw.toml
```

`migration-report.json` is also the machine-readable deployment map for later plan/deploy work. It contains the managed projects, original source paths, exact duplicates, unresolved artifacts, explicit pinned artifacts, version conflicts, and placement warnings.

During migration, a NeoForge version may be supplied explicitly:

```bash
./fpbpack catalog \
  --inventory fpbpack-inventory.json \
  --output modpack \
  --minecraft 1.21.1 \
  --neoforge 21.1.x
```

Omitting `--neoforge` is allowed while building the initial migration catalog. Before the catalog becomes authoritative for updates, the exact server NeoForge version should be recorded.

Use `--strict` when the migration is expected to be complete. Strict mode exits non-zero if unresolved artifacts or version conflicts remain.

The generator refuses to replace a non-empty output directory unless `--force` is provided.

## Diagnostics and web service

Slice 1 adds a shared management-state layer used by both the CLI and the dashboard API.

Run diagnostics against a fresh inventory and the accepted migration report:

```bash
./fpbpack doctor \
  --inventory fpbpack-inventory.json \
  --report modpack/migration-report.json
```

`doctor` reports blocking drift such as missing, moved or externally replaced managed JARs, as well as unresolved catalog entries and version conflicts. Explicitly unmanaged/pinned artifacts remain visible but are not treated as managed-file drift.

Generate a read-only update report from the accepted catalog state:

```bash
./fpbpack updates \
  --report modpack/migration-report.json \
  --output fpbpack-updates.json \
  --minecraft 1.21.1 \
  --loader neoforge
```

Update discovery supports the source types FPBPack can verify safely:

- Modrinth: Minecraft/loader filtering, release classification, required dependency closure, icons/project links, and target/intermediate changelogs;
- CurseForge: official API discovery, project links, changelogs, and conservative required-dependency resolution when a key is configured in the GUI or through `FPBPACK_CURSEFORGE_API_KEY`; files that disable third-party downloads remain Review candidates with a direct manual CurseForge file link;
- GitHub releases: only for artifacts already verified against an explicit GitHub release source. GitHub candidates require an unambiguous JAR asset with a SHA-256 digest and are always classified Review because GitHub does not provide Minecraft/loader compatibility metadata.

Pinned/unmanaged artifacts stay pinned/unmanaged; FPBPack does not guess an update source for them.

Blocking source/catalog findings can be repaired in the GUI. **Updates → Fix issues** opens **Mods → Needs attention**. From an affected mod you can retry automatic identification, keep the JAR intentionally unmanaged, or verify an explicit Modrinth, CurseForge, or GitHub source. Explicit provider assignments are accepted only when the selected provider artifact hash matches the installed JAR.

Provider traffic is rate-aware: background work is deliberately slower/lower-concurrency than interactive per-mod refreshes, and provider requests retry 429/408/5xx responses with `Retry-After` / rate-limit-reset handling and bounded exponential fallback.

CurseForge projects that disable third-party direct downloads are shown as **Review / manual download required** instead of permanently Blocked. FPBPack keeps the direct CurseForge file-page link in Updates and persisted plan review. From Review, upload the JAR downloaded from CurseForge; FPBPack streams it into its state cache, verifies the provider checksum, computes SHA-512, and rejects any mismatch before the plan can become ready.

The GUI also provides state-only remediation for blocking diagnostics. **Updates → Fix issues** opens **Mods → Needs attention**, where an installed artifact can be refreshed individually, explicitly marked unmanaged, or assigned a verified GitHub release source. GitHub assignment hashes the installed JAR and requires its SHA-256 to match the selected release asset. Missing accepted catalog entries can be explicitly forgotten. None of these actions mutates the live JAR.

FPBPack serves the static GUI and API from one process. **Serve mode is self-contained**: the normal GUI path does not require running `inventory`, `catalog`, or `updates` first.

```bash
./fpbpack serve \
  --server-root /path/to/crafty/server \
  --state-dir /path/to/fpbpack-state
```

On normal startup FPBPack loads durable state plus the last valid inventory/update caches and starts the HTTP server immediately. It does **not** automatically query providers or rescan the live mod directories at startup. The first automatic refresh waits for the configured `--refresh-interval` (6h by default), and the GUI can trigger inventory-only, update-only, or full refreshes at any time.

The legacy `migration-report.json` is a one-time bootstrap source only. If no FPBPack state exists yet, `serve` can import an existing migration report and then persists its own `state.json`; subsequent starts no longer require the migration report. If neither `state.json` nor a migration report exists, the very first startup performs the one-time inventory/provider bootstrap needed to establish accepted state.

Open the same address in a browser, for example `http://tower.local:8787/`. The GUI calls relative `/api/*` routes.

Release binaries embed the static GUI. For local frontend development, build/export the GUI separately and point FPBPack at it with `--web-dir web/out`.

The management API covers discovery, remediation, planning, server control, Apply, and Restore:

- `GET /healthz`
- `GET /api/status`, `/api/inventory`, `/api/mods`, `/api/diagnostics`, `/api/updates`
- `POST /api/refresh`, `POST /api/inventory/refresh`, `POST /api/updates/check`
- `GET /api/catalog`, `POST /api/catalog/preview`, `GET /api/logs`
- `GET|POST /api/plans`, `GET /api/plans/{id}`
- `POST /api/placement-plans`
- `POST /api/plans/{id}/manual-artifact?candidate_key=...`
- `POST /api/plans/{id}/apply`
- `GET /api/history`
- `POST /api/backups/{id}/restore` with explicit confirmation
- `GET|PUT /api/settings`
- update-rule, provider-credential, mod-management, and per-mod refresh endpoints
- `GET|PUT /api/crafty`, `DELETE /api/crafty/credentials`
- `POST /api/server/start`, `POST /api/server/stop`

Live mutation is deliberately narrower than the rest of the API. Apply accepts only a persisted ready/verified plan, rechecks the complete accepted managed state, requires Crafty to positively report the Minecraft server stopped, verifies cached target bytes again, and mutates only the exact file operations in that plan. Restore likewise requires the server stopped and verifies the currently applied files plus backup hashes before reverting them. An unknown or unreachable Crafty state fails closed.

The service owns inventory/reconciliation/update refreshes and persists generated cache snapshots under its state directory. Manual refresh/check requests return immediately and continue on a server-owned context, so reloading or closing the browser does not cancel provider discovery. Refresh status exposes phase, current provider item, totals, and percentage; the GUI shows this globally. Safe catalog-only remediation remains usable while provider discovery runs, while source/provider actions that would race fail immediately instead of waiting invisibly. Cancelled/timed-out refreshes never replace the last good update cache. Provider metadata refresh is non-destructive: transient failures retain the previous target, changelog, dependency and project metadata, mark it stale, and record the refresh error. The standalone `inventory`, `doctor`, and `updates` commands remain available for scripting and debugging, but are not required for GUI operation.

## Plan & Protect

The GUI can turn selected Safe/Review candidates into a persisted update plan without changing live mod JARs.

A plan contains:

- requested updates and mechanically required dependency changes;
- old/new versions and provider IDs;
- exact download URLs, provider checksums, and normalized SHA-512 hashes after prefetch;
- explicit `add` / `replace` filesystem operations;
- warnings and blockers;
- whether a future Apply requires the server to be stopped;
- the linked restore-point ID.

For a plan to be marked `ready`, FPBPack also:

1. resolves required provider dependency closure where safe provider metadata is available;
2. rejects conflicting or incompatible requirements;
3. rejects target paths that would overwrite unrelated/unmanaged artifacts;
4. downloads every target into `state-dir/cache/artifacts`;
5. verifies the provider checksum (Modrinth SHA-512, CurseForge SHA-1, or verified GitHub SHA-256) and computes an FPBPack SHA-512;
6. copies every current JAR that would be replaced into `state-dir/backups/<backup-id>`;
7. verifies the backup hashes and writes a manifest.

Plans and history are retained under the state directory. The default retention count is 20 and can be changed from Settings (1–100). Reducing retention prunes old plan records and their linked restore points.

## Apply & Restore

Review is the mandatory boundary before mutation. Normal update flow is:

```text
Check updates → select candidates → Review exact plan
             → stop server in Crafty → Apply
             → verify inventory/history → start server manually
```

Apply stages target JARs in their final filesystem directory and verifies SHA-512 before renaming them into place. Immediately before mutation it rechecks all blocking diagnostics, accepted managed identity/path/hash, target occupancy, and prefetched artifacts. A restore point contains both affected pre-change files and the complete accepted catalog state. If post-apply verification or persistence fails, FPBPack attempts to roll the filesystem and accepted state back to that restore point.

Current placement and preferred placement are separate. Changing a preference does not silently move a JAR. **Review move** creates a same-version verified placement plan, caches the current bytes as its target artifact, creates a restore point, and sends the move through the same stopped-server Apply path. This works even when no version update exists.

History records plan, Apply, manual-artifact verification, and Restore operations. Restore shows the exact affected paths, requires explicit confirmation, preserves unmanaged artifacts, and leaves server start manual.

### Web stack / visual system

The embedded UI uses Tailwind CSS 4 and daisyUI 5 with a custom FPBPack theme. The layout intentionally favors compact self-hosted-admin patterns: flat surfaces, thin borders, restrained status color, dense lists/tables, and minimal decorative effects.

## Docker

The production container is a single-process image. Node is used only in the build stage to produce the static export; the final image contains FPBPack and CA certificates, not a Node runtime.

Automated Docker builds run only for published GitHub releases. Pull-request/development CI does not build Docker images.

Build from the repository root:

```bash
docker build -f tools/fpbpack/Dockerfile -t fpbpack .
```

Run it with the Minecraft server and FPBPack state mounted:

```bash
docker run --rm \
  -p 8787:8787 \
  -v /path/to/crafty/server:/server \
  -v /mnt/user/appdata/fpbpack:/data \
  fpbpack
```

The image defaults to `fpbpack serve`, with `/server` and `/data` as the standard mounts. No command override or separate inventory/update generation job is required.

For Apply/Restore, the server mount must be writable. FPBPack never clears a mod directory; writes are limited to verified plan operations and restore files. The state mount stores durable catalog state, cached artifacts, history, secrets, plans, and restore points.

Crafty can be configured in **Settings → Crafty**. Environment fallbacks are also available: `FPBPACK_CRAFTY_URL`, `FPBPACK_CRAFTY_SERVER_ID`, `FPBPACK_CRAFTY_TOKEN`, and `FPBPACK_CRAFTY_INSECURE=true` for an explicitly accepted self-signed TLS certificate.

## Unraid

The Unraid host's Python installation is not used. CI builds static Linux `amd64` binaries for FPBPack and a pinned upstream Packwiz helper with `CGO_ENABLED=0`.

Download the `fpbpack-linux-amd64` artifact from the **FPBPack** GitHub Actions workflow. It contains `fpbpack-linux-amd64`, `packwiz-linux-amd64`, their checksums, and `fpbpack-sources.json`. Copy the binaries and source registry to the Unraid host:

```bash
chmod +x fpbpack-linux-amd64 packwiz-linux-amd64
```

## Development

```bash
cd tools/fpbpack/web
npm install
npm run typecheck
npm run build

cd ..
find internal/webui/dist -mindepth 1 -maxdepth 1 -exec rm -rf {} +
cp -a web/out/. internal/webui/dist/
go test ./...
go vet ./...
go run ./cmd/fpbpack inventory --server-root /path/to/test/server --offline
go run ./cmd/fpbpack catalog --inventory /path/to/fpbpack-inventory.json --output /tmp/fpbpack-modpack
# Packwiz-backed CurseForge detection is integration-tested with a fake isolated helper.
```

For CurseForge update discovery, the normal path is **Settings → Providers → CurseForge**. Enter the key there and FPBPack validates it before saving it to `state-dir/secrets.json` with owner-only (`0600`) permissions. The key is never returned by the API or repopulated into the browser.

`FPBPACK_CURSEFORGE_API_KEY` remains available as a deployment/environment fallback. A GUI-saved key takes precedence over the environment value.

For verified GitHub release sources, `FPBPACK_GITHUB_TOKEN` is optional and can be used to improve API rate limits or access eligible private sources.

Provider traffic is paced in two modes. Scheduled automatic refresh uses a slower background policy with lower concurrency. Startup itself performs no automatic provider refresh. Explicit **Check updates** and per-mod refresh use a faster interactive policy, while still sharing provider-wide pacing and honoring `Retry-After`, GitHub rate-limit reset headers, and bounded exponential backoff for transient errors.

Crafty integration uses API v2 with bearer-token authentication. Start/Stop remain explicit user actions; Apply and Restore never restart the Minecraft server implicitly. Deployment operates only on files represented by a verified plan, and unmanaged/pinned artifacts remain protected.


### GUI operations and manual reconciliation

The left navigation includes **Tools** and **Logs**. Tools exposes the practical running-server equivalents of `inventory`, `doctor`, `updates`, and a full refresh, plus JSON views for inventory, accepted catalog, diagnostics, updates, and status/version. The legacy catalog-migration workspace generator remains CLI-only because it accepts arbitrary output paths and an external Packwiz helper.

If a managed JAR is changed manually outside FPBPack, the next inventory/full refresh reports drift instead of silently accepting it. **Adopt current JAR** is the explicit recovery path for intentional manual replacement: FPBPack verifies the replacement against the same Modrinth project, CurseForge project, or GitHub repository before updating accepted state. A replacement that cannot be proven to belong to the accepted source remains blocked.
