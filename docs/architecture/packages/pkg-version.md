# pkg/version

## Purpose

`pkg/version` exposes the identity that the build stamps into the binary. It is a public package.

## Design

- **Three sources, in order.** (1) The values that the linker injects with `-ldflags '-X .version=... -X .commit=... -X .builtAt=...'` (the release and `make build` path). (2) The VCS revision in `runtime/debug.BuildInfo` (the `go install` path). (3) The literals `dev` and `unknown` (the `go run` and `go test` path). The linker variables are for the linker only: call `Get`, do not read them.
- **`Commit`** returns the raw embedded build commit (short sha) for provenance checks. [ADR-0065](../adr/0065-per-phase-binary-integrity.md) calls a binary legitimate when its build commit is an ancestor of `HEAD`. It returns `""` when the binary is not stamped, and callers treat that as unverifiable provenance. It prefers the linker `commit`, else the `vcs.revision`.
- **Pure resolvers.** `composeCommit`, `composeVersion` and `formatVersion` hold no package state, so tests drive every branch. `readBuildInfo` is the seam over `debug.ReadBuildInfo`, and tests swap it to inject `vcs.revision` and `vcs.time`, which do not appear under `go test`.
- **`shortSHA`** trims whitespace and cuts to 12 characters, the default short-sha length of git and the `--short=12` flag of the Makefile.

## Invariants

- **No panic, and a shaped string, without injection.** Under `go test` the exact value depends on the VCS info, so the tests pin the shape and the 12-character limit, not the value. Pinned by `TestGetReturnsShapedString` and `TestCommit_ShortShapeNeverPanics`.
- **Linker values win.** Pinned by `TestGet_HonorsLdflagInjectedValues` and `TestCommit_HonorsLdflagInjectedCommit`.
