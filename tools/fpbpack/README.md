# fpbpack

`fpbpack` is the FPBCraft modpack inventory/update helper.

It understands the current FPBCraft layout:

- `mods/*.jar` — server/common mods
- `automodpack/host-modpack/main/mods/*.jar` — AutoModpack client-only mods

## Inventory

`fpbpack inventory` is intentionally **read-only**. It inventories the live Crafty server without changing, deleting, moving, or downloading any mod JARs.

For each JAR it records SHA-1/SHA-512 hashes and the CurseForge Murmur2 fingerprint, reads NeoForge/Forge metadata (and Fabric metadata as a fallback for Connector-hosted mods), and performs an exact SHA-512 lookup against Modrinth's version-file API. If a CurseForge API key is available, JARs that did not match Modrinth are then resolved by exact CurseForge fingerprint.

```bash
./fpbpack inventory \
  --server-root /path/to/crafty/server \
  --json fpbpack-inventory.json
```

To also resolve CurseForge-only mods, provide the API key through the environment:

```bash
export CURSEFORGE_API_KEY="..."
./fpbpack inventory \
  --server-root /path/to/crafty/server \
  --json fpbpack-inventory.json
```

The key is used only for the API request. It is not written to the inventory JSON, Packwiz metadata, or migration report. Exact Modrinth matches take precedence; CurseForge is queried only for files that remain unmatched.

To skip all remote lookups and generate a local-only inventory:

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

`fpbpack catalog` converts a verified inventory JSON into a Packwiz catalog without touching the live server.

```bash
./fpbpack catalog \
  --inventory fpbpack-inventory.json \
  --output modpack
```

The generator is conservative:

- byte-identical JARs found in both locations are represented once;
- if the same Modrinth project has multiple installed versions, all versions for that project are withheld from the generated Packwiz catalog and reported as a conflict;
- JARs without an exact Modrinth or CurseForge match are reported as unresolved rather than guessed from filenames;
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

`migration-report.json` is also the machine-readable deployment map for later plan/deploy work. It contains the managed projects, original source paths, exact duplicates, unresolved artifacts, version conflicts, and placement warnings.

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

## Unraid

The Unraid host's Python installation is not used. CI builds a static Linux `amd64` executable with `CGO_ENABLED=0`, so the server only needs the resulting `fpbpack` binary.

Download the `fpbpack-linux-amd64` artifact from the **FPBPack** GitHub Actions workflow and copy the executable to the Unraid host:

```bash
chmod +x fpbpack
```

## Development

```bash
cd tools/fpbpack
go test ./...
go vet ./...
go run ./cmd/fpbpack inventory --server-root /path/to/test/server --offline
go run ./cmd/fpbpack catalog --inventory /path/to/fpbpack-inventory.json --output /tmp/fpbpack-modpack
```

Future slices will resolve explicit GitHub/custom artifacts and add explicit plan/deploy commands. Deployment will only operate on files recorded as managed by fpbpack.
