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

The first provider implementation performs Modrinth project/version discovery, rejects incompatible Minecraft/loader releases, classifies pre-releases and major-version jumps for review, preserves rejected newer candidates, and surfaces required/incompatible dependency relationships. CurseForge and GitHub update discovery are still reported as blocked/pending rather than guessed.

FPBPack serves the static GUI and API from one process. **Serve mode is self-contained**: the normal GUI path does not require running `inventory`, `catalog`, or `updates` first.

```bash
./fpbpack serve \
  --server-root /path/to/crafty/server \
  --state-dir /path/to/fpbpack-state
```

On startup FPBPack scans the live server, reconciles it with durable FPBPack state, refreshes provider/update information, and writes its own inventory/update cache files under the state directory.

The legacy `migration-report.json` is a one-time bootstrap source only. If no FPBPack state exists yet, `serve` can import an existing migration report and then persists its own `state.json`; subsequent starts no longer require the migration report.

Open the same address in a browser, for example `http://tower.local:8787/`. The GUI calls relative `/api/*` routes.

Release binaries embed the static GUI. For local frontend development, build/export the GUI separately and point FPBPack at it with `--web-dir web/out`.

The initial API is deliberately read-only:

- `GET /healthz`
- `GET /api/status`
- `GET /api/inventory`
- `GET /api/mods`
- `GET /api/diagnostics`
- `GET /api/updates`
- `POST /api/refresh`
- `POST /api/updates/check`

The service owns inventory/reconciliation/update refreshes and persists generated cache snapshots under its state directory. The standalone `inventory`, `doctor`, and `updates` commands remain available for scripting and debugging, but are not required for GUI operation. Live-JAR mutation endpoints remain intentionally absent in Slice 1.

## Docker

The production container is a single-process image. Node is used only in the build stage to produce the static export; the final image contains FPBPack and CA certificates, not a Node runtime.

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

The same image is suitable for an eventual Unraid template. Future write-capable slices will mount only the server/state paths FPBPack actually needs.

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

Future slices will add explicit update-plan and deploy commands. Deployment will only operate on files recorded as managed or explicitly pinned by fpbpack. FPBPack does not embed or require a user-provided CurseForge API key.
