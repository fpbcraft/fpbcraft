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
        ├── inventory / doctor / update services
        ├── Minecraft + AutoModpack filesystem
        └── provider integrations
```

The GUI and API are same-origin. The GUI never owns update eligibility, dependency resolution, filesystem mutation, backup, restore, or provider policy.

## Repository layout

```text
tools/fpbpack/
├── cmd/fpbpack/
├── internal/
│   ├── httpapi/
│   ├── webui/
│   │   └── dist/        generated static export staged before release builds
│   └── ...
├── web/                 Next.js static-export source
├── Dockerfile
└── go.mod
```

## Frontend runtime

Next.js is used only as a static build tool. Production does not run a Next.js server or Node process.

The frontend fetches relative routes such as `/api/mods`. Release builds copy `web/out` into `internal/webui/dist` and Go's `embed` package places those assets in the FPBPack binary.

## Docker

The Dockerfile is multi-stage:

1. Node builds/typechecks the static GUI.
2. Go compiles FPBPack with those files embedded.
3. The runtime image contains the FPBPack binary and CA certificates only.

The container exposes port 8787 and runs a single process. This is the target deployment model for Unraid as well as ordinary Docker.

## Development

Backend unit tests remain independent of Node. The web handler accepts an `fs.FS` for testing, while the CLI injects the embedded production assets.

A developer can also run `fpbpack serve --web-dir web/out` after a frontend build to test without rebuilding the binary for each static change.

## Repository decision

The former `fpbcraft-gui` repository is being folded into `fpbcraft/tools/fpbpack/web`. New GUI/API work should be implemented together in the FPBPack branch so wire-contract changes, tests, and releases stay synchronized.
