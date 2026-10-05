# FPBPack — Implementation Plan

Status: **in progress — Slice 1**

Progress log: [IMPLEMENTATION_PROGRESS.md](./IMPLEMENTATION_PROGRESS.md)

This document turns the product contract in [ARCHITECTURE.md](./ARCHITECTURE.md) into a deliberately short implementation sequence.

The priority is speed to a useful product. We should avoid turning every feature into its own slice or PR.

## Delivery strategy

FPBPack and its GUI now ship as **one product from one repository**. The GUI source lives under `tools/fpbpack/web`, is statically exported, and is served by `fpbpack serve` from the same origin as the API.

Implementation remains grouped into only **three substantial slices**:

1. **Discover & Decide** — make the app useful for understanding updates.
2. **Plan & Protect** — make update batches reviewable, persistent, and safe.
3. **Apply & Restore** — make the app capable of changing the live pack safely.

A slice may change Go domain services, the HTTP API, and the static GUI together in one FPBPack PR. API/UI contract changes should be tested and released together.

Do not create extra slices merely for UI polish, icons, mobile layout, changelogs, individual provider work, or isolated API endpoints when they naturally belong to one of the three slices.

We still keep commits internally coherent and tests focused, but the user-facing delivery cadence should stay coarse.

---

# Foundation — current work

This is the current GUI PR and the FPBPack migration work already completed.

## Already established

- inventory of server/common and client-only mods;
- exact Modrinth identification;
- isolated CurseForge detection;
- conservative Packwiz migration;
- strict placement validation;
- explicit unmanaged/pinned custom artifacts;
- no blind `packwiz update --all`;
- responsive Next.js static-export GUI embedded in FPBPack;
- single-process Docker/Unraid deployment target;
- one combined FPBPack binary/image serving GUI and API;
- product/architecture contract.

## Foundation exit condition

The foundation is complete when:

- the combined FPBPack PR is mergeable and green;
- the six custom artifacts remain outside Packwiz;
- the GUI and API run together on Unraid from the same Docker container;
- the release binary embeds the static GUI;
- subsequent work can rely on the product contract.

No additional foundation slice should be created unless a blocking architectural defect is found.

---

# Slice 1 — Discover & Decide

## Goal

Turn the current dashboard into a genuinely useful mod-management console without allowing live mutations yet.

At the end of this slice, the app should answer:

> What can I update, what changed, what is safe, and why?

## FPBPack work

Implement the reusable application/service layer needed by both CLI and GUI.

### Application runtime, inventory, and diagnostics

- make `fpbpack serve` the self-contained application mode;
- require only the server root and FPBPack state directory for normal GUI operation;
- load/persist durable FPBPack state under the state directory;
- import the legacy migration report only as a one-time bootstrap when needed;
- automatically scan the live server at startup and on GUI refresh;
- automatically refresh/update internal inventory and update caches;
- keep standalone CLI commands optional rather than prerequisites;
- add `doctor`-style diagnostics;
- detect managed files changed outside FPBPack;
- distinguish actionable problems from informational metadata;
- expose current server/common/client placement cleanly.

### Update discovery

Implement provider-aware update discovery without using raw `packwiz update --all` as the decision engine.

For every managed mod:

- find candidate versions;
- enforce Minecraft 1.21.1 compatibility;
- enforce NeoForge compatibility;
- reject Fabric/Forge-only substitutions;
- understand release channel;
- recognize major-version jumps;
- exclude ignored/pinned/unmanaged artifacts;
- preserve rejected candidates for advanced diagnostics.

Classification:

- Safe / recommended
- Review
- Blocked
- Ignored / pinned

### Dependency resolution

- read provider dependency metadata;
- determine required dependency additions/updates;
- automatically include mechanically required dependencies in the candidate result;
- block an update when required dependencies cannot be resolved safely;
- expose reverse dependencies where provider metadata allows it.

This does not need to become a fully general SAT solver. It should solve the dependency relationships relevant to the currently installed pack and provider metadata.

### Changelog/project metadata

Expose enough metadata for the GUI to show:

- project/mod name;
- icon URL;
- provider;
- project URL;
- installed version;
- latest valid version;
- release date;
- release channel;
- full provider changelog;
- all intermediate changelogs when skipping versions;
- dependency changes;
- useful file metadata.

### Pin/ignore state

Introduce persistent rules for:

- pin current version;
- ignore this version;
- ignore updates for this mod;
- review/remind later.

A simple durable JSON/state file is acceptable initially. Avoid adding a database unless the data model actually requires one.

### Local API

The FPBPack HTTP service owns current application state instead of reading user-supplied inventory/report/update files on every request.

Introduce the minimum useful read/update-discovery API.

Expected capabilities, not necessarily exact routes:

- server/status;
- inventory/mod list;
- mod details;
- diagnostics;
- refresh inventory/reconciliation;
- refresh/check updates;
- current update candidates;
- pin/ignore mutations.

CLI commands should call the same Go services rather than duplicating logic.

## GUI work

Replace the current migration/dashboard UI with the target admin UI.

### Navigation

Move to:

- Overview
- Updates
- Mods
- History
- Settings

History can initially show an empty/not-yet-used state until Slice 2 provides real data.

### Overview

Build the inbox:

- server running/stopped;
- number of updates available;
- blocking findings;
- top update candidates;
- external changes if detected;
- recent actions when available.

Remove:

- provider-count dashboard cards;
- schema/version/debug information;
- file modification timestamps;
- explanatory safety panels;
- migration-oriented copy.

### Updates page

Provide:

- Update all safe updates;
- individual selection;
- Safe / Review / Blocked / Ignored grouping;
- pre-release labels;
- major-update warning;
- dependency-driven update indication;
- refresh/check updates action.

No Apply button yet. Selection can lead to a preview/review stub that becomes fully functional in Slice 2.

### Mods page

Prism Launcher-like inventory:

- mod icon;
- mod name;
- installed version;
- latest valid version;
- status;
- side;
- compact pin/unmanaged indicators.

Filters:

- updates available;
- server/client/both;
- pinned/unmanaged;
- Modrinth;
- CurseForge;
- unresolved.

### Mod detail modal/sheet

Desktop modal/drawer and mobile sheet:

- project icon/name;
- versions;
- status;
- provider link;
- release metadata;
- complete changelog;
- aggregated intermediate changelogs;
- dependency relationships;
- pin/ignore controls;
- advanced filename/path/provider identifiers.

### Responsive requirement

Mobile feature parity is part of this slice, not a later polish ticket.

The Updates list, mod browser, detail modal/sheet, changelogs, filters, and actions must all be usable on phone and desktop before Slice 1 is considered done.

## Slice 1 exit condition

Slice 1 is complete when, on the real FPBCraft inventory:

- the GUI shows real available updates;
- Fabric/incorrect-loader candidates do not appear as normal updates;
- required dependencies are identified;
- changelogs can be read in-app;
- mod icons and provider links work;
- pins/ignores persist;
- starting the GUI does not require separately generated inventory, migration-report, or updates JSON files;
- the six unmanaged custom artifacts appear normally but have no update action;
- no live mod JAR can yet be changed.

This should already be useful enough to replace manual browsing of Modrinth/CurseForge when deciding what to update.

---

# Slice 2 — Plan & Protect

## Goal

Make a selected update batch deterministic, reviewable, persistent, and safe before allowing actual application.

At the end of this slice, the app should answer:

> Exactly what would this update batch do, and can it be done safely?

## FPBPack work

### Update plan

Implement `plan` as a pure operation.

Input:

- selected update candidates.

Output:

- requested mod changes;
- automatically required dependency changes;
- old/new versions;
- exact source artifacts;
- exact hashes;
- deployment location;
- downloads required;
- filesystem removals/additions;
- warnings;
- blockers;
- whether the server must be stopped;
- backup requirement.

The plan must be deterministic enough to persist and later verify before Apply.

### Prefetch/verification

Before a plan can be considered ready:

- resolve all provider artifacts;
- verify expected metadata;
- verify download availability;
- verify hashes when provider data supports it;
- ensure the entire dependency closure is resolved;
- ensure no pinned/unmanaged artifact will be touched.

No live mod files are changed.

### Persistent management state

Persist:

- current managed artifact identity;
- pin/ignore rules;
- ignored versions;
- last inventory snapshot;
- external-change baseline;
- generated plans;
- operation/history records;
- backup references;
- retention setting.

Prefer a simple implementation such as JSON/structured files unless concurrency/data volume justifies SQLite.

### External change protection

Before planning or applying:

- compare current live files with the last accepted managed state;
- identify manual additions/removals/replacements/moves;
- clearly associate every external change with its mod where possible;
- block destructive operations until the changed state is reconciled.

Unmanaged artifacts should not create this warning merely because they changed.

### Backup design

Implement real restore points before Apply exists.

The backup should cover the files/state FPBPack is capable of mutating, rather than blindly archiving the whole Minecraft world unless needed.

Record:

- backup ID;
- timestamp;
- affected files;
- pre-change management state;
- relationship to the update plan.

Fixed-count retention is configurable.

### History model

Create structured history records for:

- update plans;
- completed operations later;
- failed operations;
- restores;
- inventory reconciliation.

## GUI work

### Review screen

Make the review screen the mandatory transition between selection and Apply.

Primary presentation:

- affected mods;
- old → new versions;
- dependency-driven changes;
- warnings;
- blockers;
- backup/restore-point notice;
- current server state.

Advanced expansion:

- exact downloaded artifacts;
- hashes;
- exact filesystem removals/additions;
- physical paths.

Every low-level file operation must remain grouped under the affected mod.

### History page

Display structured history with:

- timestamp;
- operation type;
- mods affected;
- version changes;
- success/failure/pending state;
- backup ID;
- restore relationship.

Restore can be visible but disabled until Slice 3 implements it.

### Settings

Only useful settings:

- backup/history retention count;
- FPBPack/Crafty connection status;
- refresh/check behavior if needed;
- diagnostics/advanced information.

Avoid turning Settings into a dump of implementation metadata.

## Slice 2 exit condition

Slice 2 is complete when:

- a set of updates can be selected;
- dependency changes are automatically included;
- a complete deterministic plan is produced;
- the GUI review screen accurately represents that plan;
- the full batch is pre-resolved and verified before mutation;
- external managed-file changes block unsafe plans;
- backup metadata/history exists;
- **Apply still does not mutate the live server**.

This gives us a strong dry-run checkpoint before enabling destructive operations.

---

# Slice 3 — Apply & Restore

## Goal

Make the reviewed plan executable on the real FPBCraft server with manual server control and recovery.

At the end of this slice, the GUI should be the normal way to update the pack.

## FPBPack work

### Controlled Apply

Apply only a previously generated/verified plan.

Before mutation:

- re-check the live state;
- ensure the plan has not gone stale;
- ensure all downloads are already resolved/verified;
- create the restore point;
- require the server to be stopped before changing relevant live files.

During mutation:

- only touch explicitly managed files in the plan;
- never clear entire mod directories;
- never touch unmanaged/pinned artifacts;
- preserve server/client physical destinations;
- apply the batch atomically where practical;
- fail with a clear per-mod error when an operation cannot proceed.

After mutation:

- regenerate/verify inventory;
- persist the new accepted managed state;
- record history;
- leave the Minecraft server **stopped**.

No automatic restart.

### Crafty integration

Expose explicit manual controls:

- Stop server
- Start server

These should be deliberate GUI actions.

Applying updates may tell the user to stop the running server and provide the Stop action, but it should not silently stop/restart it as an implicit side effect unless we deliberately revise the product contract.

### Restore

Implement manual Restore from a history/backup record.

Restore should:

- require an explicit confirmation;
- require safe server state;
- restore the exact managed files/state from the selected restore point;
- preserve unmanaged artifacts;
- record the restore as a new history event;
- leave server start/restart manual.

### Retention

Enforce the configured fixed backup/history retention without deleting a restore point still required by an active/incomplete operation.

## GUI work

### Apply flow

From Review:

- show readiness/blockers;
- provide Stop Server when needed;
- allow Apply only when prerequisites are satisfied;
- display deterministic progress stages;
- associate errors with affected mods;
- show final changed-mod summary;
- leave a clear manual Start Server action when complete.

### Restore flow

History → operation → Restore:

- show exactly what will be restored;
- require confirmation;
- show progress/result;
- never hide which mods/files are affected.

### Operational polish

Complete in this same slice rather than creating a separate polish phase:

- loading states;
- empty/error states;
- keyboard accessibility;
- mobile feature parity;
- sticky mobile actions where useful;
- compact desktop density;
- provider/mod icon fallbacks;
- clear disabled/blocking explanations;
- Docker/Unraid service integration;
- health checks.

## Slice 3 exit condition

The project is operational when the normal workflow works end to end:

```text
Check updates
    ↓
Read changelogs
    ↓
Select all or some
    ↓
Dependencies added automatically
    ↓
Review exact plan
    ↓
Stop server manually if running
    ↓
Apply
    ↓
Verify
    ↓
History + restore point recorded
    ↓
Start server manually
```

And:

- no unmanaged artifact is touched;
- no incompatible loader candidate can be applied;
- partial download/resolve failure does not mutate live files;
- external managed-file drift blocks unsafe application;
- restore works from history;
- desktop and mobile both support the full workflow.

---

# What we intentionally do NOT split into separate slices

Unless implementation reveals a real blocker, do not create independent slices for:

- mod icons;
- changelog rendering;
- CurseForge UI;
- Modrinth UI;
- dependency display;
- mobile navigation;
- responsive styling;
- pin/ignore controls;
- Settings;
- History UI;
- Crafty buttons;
- backup retention;
- accessibility cleanup;
- error/loading states;
- Docker tweaks.

They belong inside the three feature slices above.

Similarly, avoid building speculative infrastructure before its slice needs it.

Examples:

- no database migration project before simple state files prove insufficient;
- no generic multi-server API;
- no authentication system for LAN-only use;
- no second GUI service, CORS bridge, or cross-origin production API path;
- no generalized dependency solver beyond the actual FPBCraft problem;
- no full Crafty replacement.

---

# PR strategy

The user merges PRs manually.

To keep delivery fast:

- target at most **one combined FPBPack PR per slice**;
- keep GUI/backend changes together when they share a contract; do not fragment features into micro-PRs;
- keep commits internally reviewable even when the PR is substantial;
- do not merge on the user's behalf;
- CI must pass before calling a slice complete.

Expected remaining high-level delivery:

```text
Current foundation PR
        ↓
Slice 1: Discover & Decide
        ↓
Slice 2: Plan & Protect
        ↓
Slice 3: Apply & Restore
        ↓
Usable FPBCraft mod manager
```

That is the intended plan. Additional slices should only be introduced for genuinely independent work or an unforeseen architectural blocker.
