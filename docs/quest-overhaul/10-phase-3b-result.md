# Phase 3B result — Aeronautics & Transportation

Status: **complete**.

This slice expands the Aeronautics and Transportation sections while preserving existing quest IDs where content was moved from the old FPBcraft catch-all pages.

## Aeronautics

The old `Flight Workshop` page has been decomposed into three focused chapters.

### Aeronautics Toolgun

The original chapter ID and existing Survival Structure Tool / Portable Structure Container quest IDs were preserved.

The page now guides players through:

- Survival Structure Tool;
- the actual Toolgun modes exposed by the current 1.21.1 source:
  - Save
  - Load
  - Delete
  - No Collision
  - Weld
  - Simple Weld
  - Translate
  - Rotate
  - Disconnect
- saving a small vehicle blueprint;
- Portable Structure Container;
- printing/restoring a saved vehicle;
- Disposable Vehicle Container;
- Magnetic Gun;
- moving and rotating physical structures;
- welding/disconnecting structures;
- collision control;
- restoring a more complex multi-sublevel vehicle and checking ropes, wires, constraints and block-entity data.

The mechanics were validated against `userenxv/create-aeronautics-toolgun`, whose 1.21.1 implementation explicitly supports saved/printed multi-sublevel vehicles and structure manipulation.

### Aeroworks Controls

The former Aeroworks branch is now its own page.

It includes:

- Joystick;
- Gyroscope;
- Control Stand;
- Button Module;
- Control Desk;
- a practical “build a working cockpit” milestone.

Existing Joystick/Gyroscope/Control Desk quest IDs were retained.

### Docking and Rope

The throwable-rope branch is now its own page.

It covers:

- Throwable Rope Connector;
- Rope Connector Launcher;
- Mounted Rope Launcher;
- docking two physical structures;
- building a repeatable mounted capture/retrieval setup;
- a Skybound crossover milestone for boarding a moving Sable/Create Aeronautics craft.

Skybound’s full equipment/upgrade progression remains deferred to Combat and Equipment.

## Transportation

### Railway Services

The old `Railway Services` chapter ID is retained, but unrelated economy content has been removed.

It now focuses on:

- Create Railways Navigator;
- mapping station groups and sensible station names;
- Train Station Clock;
- Advanced Display;
- passenger-facing information;
- taking a planned multi-station connection;
- Create Train Parts crossing/infrastructure;
- testing crossings, clearances and route behavior before connecting to a busy line.

### Create: Power Loader

Power Loader now has a dedicated progression page instead of one orphaned loader quest.

The page is based on the current upstream 1.21.1 behavior:

1. Empty Andesite Chunk Loader;
2. capture a Ghast to activate the loader;
3. Andesite Chunk Loader;
4. test a single loaded chunk;
5. Empty Brass Chunk Loader;
6. Brass Chunk Loader;
7. configure 1x1 / 3x3 / 5x5 loading range;
8. load a moving/stationary contraption;
9. load a train;
10. attach a Brass loader to a Train Station;
11. server-friendly chunk-loading cleanup.

The upstream Ponder text/source confirms:

- empty loaders capture Ghasts;
- the Andesite tier loads one chunk with rotational power;
- Brass supports 1x1, 3x3 and 5x5 stationary ranges;
- assembled contraption loaders cover 3x3 and do not require rotational power;
- Brass loaders can attach to Train Stations and load a 3x3 area while a train is present.

The original FPBcraft Empty Andesite Loader quest ID was preserved.

## Storage cleanup — Player Economy

Numismatics was previously embedded in Railway Services despite not being railway progression.

It is now a compact **Player Economy** chapter under Storage, preserving the existing Banking Guide, Bank Terminal and Sale Point quest IDs and adding:

- Andesite Depositor;
- a practical small-shop test milestone.

Storage ordering is now:

1. Sophisticated Backpacks
2. Storage and Computers
3. Player Economy

## Final section order

### Aeronautics

1. Simulated
2. Aeronautics
3. Off-Road
4. Propulsion
5. Aeronautics Toolgun
6. Aeroworks Controls
7. Docking and Rope

### Transportation

1. Trains
2. Minecarts
3. Railway Services
4. Create: Power Loader

## Validation

Final Phase 3B quest tree:

- **47 chapter files**
- **756 unique quest IDs**
- **0 duplicate quest IDs**
- **0 missing dependency targets**
- **0 missing chapter title localization keys**
- **0 missing chapter-group title localization keys**
- both English locale files are byte-identical
- all newly generated chapter IDs remain below the signed 64-bit boundary used after the Phase 3A formatting fix
- test ZIP passes a full archive integrity check

## Deferred

Still deferred to later slices:

- the full Skybound Grappling Hook equipment/upgrades page;
- Farmer’s Delight / Immortalers / food expansion;
- Tide / Aquamirae / sea exploration;
- building/decor expansion;
- combat/equipment expansion;
- RPG class trees;
- guided boss progression.
