# fpbpack

`fpbpack` is the FPBCraft modpack inventory/update helper. The first implementation slice is intentionally **read-only**: it inventories the live Crafty server without changing, deleting, moving, or downloading any mod JARs.

It understands the current FPBCraft layout:

- `mods/*.jar` — server/common mods
- `automodpack/host-modpack/main/mods/*.jar` — AutoModpack client-only mods

For each JAR, `fpbpack inventory` records SHA-1/SHA-512 hashes, reads NeoForge/Forge metadata (and Fabric metadata as a fallback for Connector-hosted mods), and performs an exact SHA-512 lookup against Modrinth's version-file API.

## Unraid

The Unraid host's Python installation is not used. CI builds a static Linux `amd64` executable with `CGO_ENABLED=0`, so the server only needs the resulting `fpbpack` binary.

Download the `fpbpack-linux-amd64` artifact from the **FPBPack** GitHub Actions workflow and copy the executable to the Unraid host, then:

```bash
chmod +x fpbpack
./fpbpack inventory --server-root /path/to/crafty/server --json fpbpack-inventory.json
```

The command only reads the server directories. The optional `--json` path is the only file it writes.

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

## Development

```bash
cd tools/fpbpack
go test ./...
go vet ./...
go run ./cmd/fpbpack inventory --server-root /path/to/test/server --offline
```

Future slices will use this verified inventory to generate the Packwiz catalog and, only after that, add explicit plan/deploy commands.
