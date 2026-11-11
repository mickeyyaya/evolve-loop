# internal/goload

> The design: section 7.7 of [gate-contract-g2-hermetic-guard](../gate-contract-g2-hermetic-guard.md), commit 2 (ADR-0165). The first user: [internal-adapters-codeindex-gotypes](internal-adapters-codeindex-gotypes.md).

## Purpose

`internal/goload` lists Go packages with `go list` and type-checks them with `go/types`. It uses the standard library and `sysexec` only. The call-graph hermeticity guard and the code index both load a module through it.

## Design

- **`Loader.Load` and `Loader.LoadWithTests`.** Each runs `go list -buildvcs=false -json -deps` through the `sysexec.RunFunc` of the loader. `LoadWithTests` adds `-test`. The environment is the request environment plus `CGO_ENABLED=0`.
- **`Loader.Walk`.** It calls a visitor for each module package, in dependency order, as soon as that package passes its check. The visitor gets the shared `token.FileSet` and the package. A visitor error stops the walk, and `Walk` returns it. `Load` and `LoadWithTests` call the same walk with a visitor that does nothing.
- **A loader with no runner.** It returns an error, not a panic.
- **One program.** `Program` holds the shared `token.FileSet`, each package by its import path, and `Order`. A test variant has a key such as `m/b [m/b.test]`, and its `.test` main package is dropped.
- **Order is dependency order.** A package enters `Order` after the check of each package that it imports. This is the order that the code index had before the move, so the index does not change.
- **Standard-library bodies are skipped.** A standard package is checked with `IgnoreFuncBodies`. Its `Info` is empty, and it keeps no syntax trees.
- **A module package keeps its facts.** A package of the main module has `Module` set, a `types.Info` with `Defs` and `Uses`, its syntax trees, and its errors. A parse error or a type error is an entry of `Errors`, not a failure of the load.
- **The importer is the loader.** The importer reads the `ImportMap` of the package and checks the mapped package from the same `go list` answer. It does not use `go/importer`, which can start `go tool cgo` outside the runner.
- **Failures.** A runner error, a non-zero exit, a bad JSON answer and a cancelled context each return an error. The load never returns a partial program with a nil error.

## Invariants

- **The loader imports only the standard library and `sysexec`.**
- **The package is a protected surface** (`/go/internal/goload/` in `guards.ProtectedSurfaceManifest`).
- **Enforced at 100%** statement coverage of its own tests.

## Findings

- **The move kept the memory in a bound.** The code index calls `Walk`, indexes each module package in its visitor, and sets `Files` and `Info` of the package to nil. The next package needs only the `Types` of the earlier ones. So the trees go. On this module, one index build peaked at 1.39 to 1.47 GB of resident memory with the eager `LoadWithTests`. With `Walk`, it peaked at 0.69 to 0.70 GB.
