# Build Explanation — Cycle 1753

## Build Binding
- Cycle: 1753
- Base SHA: 05e5efc672f89c1479f11fb891778c9ef29330bf

## Summary
Two size-ratchet offenders (`cmd/evolve.applyCarryoverDecisions` at 57 lines
and `cmd/evolve.gcWorkspaceSweep` at 60 lines) are shrunk to the ratchet's
50-line ceiling by extracting one self-contained block out of each into a
named, independently-tested helper. Both extractions are behavior-preserving
refactors with no change to observable CLI behavior.

## Rationale
`applyCarryoverDecisions`'s unlocked pre-read fast path (an optimization that
skips a no-op locked write when no removal id is present in state) is a
distinct, testable unit from the locked read-modify-write it guards. Pulling
it into `carryoverNoOpFastPath` is the smallest change that removes both the
size violation and the untested-in-isolation fast-path logic, without
touching the locked-update path at all.

`gcWorkspaceSweep`'s `--project-root` resolution (explicit passthrough,
dry-run-only cwd fallback, mutating-run refusal) is likewise a self-contained
decision block ahead of the actual sweep logic. Extracting it into
`resolveGCProjectRoot` isolates the three branches for direct unit testing
and shrinks the caller by exactly the amount needed to clear the ratchet.

Both are pure "lift a block into a named function" moves — the simplest
option that satisfies the ratchet, with no restructuring of the surrounding
logic and no new abstractions beyond the two helpers the tasks name.

## Changed Areas
- `go/cmd/evolve/cmd_carryover.go` — extracts the unlocked pre-read fast path
  out of `applyCarryoverDecisions` into `carryoverNoOpFastPath(statePath,
  remove) (carryoverApplyResult, bool)`; the caller now short-circuits on
  `ok==true` and otherwise falls through to the unchanged locked
  `statemap.UpdateStateMap` path. `applyCarryoverDecisions` is now 43 lines
  (was 57).
- `go/cmd/evolve/cmd_gc.go` — extracts the `--project-root` resolution block
  out of `gcWorkspaceSweep` into `resolveGCProjectRoot(projectRoot string,
  dryRun bool, stderr io.Writer) (string, int, bool)`; the caller reassigns
  `projectRoot` and returns `code` when `ok==false`. `gcWorkspaceSweep` is
  now 50 lines (was 60).
- `go/internal/sizeratchet/offenders.json` — removes the
  `cmd/evolve.applyCarryoverDecisions` and `cmd/evolve.gcWorkspaceSweep`
  entries now that both functions are at or under the ceiling.
- `go/cmd/evolve/cmd_carryover_fastpath_test.go` — unit tests for
  `carryoverNoOpFastPath`: no-op hit, fall-through on a matching removal id,
  and fall-through on an unreadable/missing state path (TDD-engineer
  authored; not modified by the build).
- `go/cmd/evolve/cmd_gc_projectroot_test.go` — four unit tests for
  `resolveGCProjectRoot`: refused empty+mutating, cwd fallback on
  empty+dry-run, and explicit-value passthrough under both dry-run states
  (TDD-engineer authored; not modified by the build).
- `go/acs/cycle1753/predicates_test.go` — the cycle's ACS predicates pinning
  both line-count ceilings, both extraction call-sites, both offenders.json
  removals, gofmt/vet cleanliness on the two touched files, and the
  accuracy of this document's claims about the two new test files, and the
  consistency of its Verification predicate and suite counts (TDD-engineer
  authored).
- `.evolve/evals/shrink-carryover-apply.md`, `.evolve/evals/shrink-gc-workspace-sweep.md` —
  score-cap eval definitions for the two tasks (scout authored; score-cap
  entries added by the TDD engineer).

## Design Decisions
`carryoverNoOpFastPath` keeps its own `os.Stat` guard ahead of
`statemap.ReadStateMap`, even though the caller already stats the path
first — `statemap.ReadStateMap` treats a missing file as an empty, err-nil
state (by its own contract), which would otherwise make the helper report a
false no-op instead of falling through. Duplicating the guard makes the
helper correct in isolation, independent of what its one caller happens to
check first.

`resolveGCProjectRoot` takes `stderr io.Writer` directly (matching
`gcWorkspaceSweep`'s existing `stdout, stderr io.Writer` parameters) rather
than a wrapped options struct — the minimal signature for a three-way
branch with two possible error messages.

## Verification
- `go test -count=1 -run 'TestCarryoverNoOpFastPath_|TestCarryover|TestResolveGCProjectRoot_|TestRunGC_' ./cmd/evolve/...` — PASS
- `go test -count=1 ./cmd/evolve/...` (full package, both touched files' pre-existing suites) — PASS
- `go test -tags acs -count=1 ./acs/cycle1753/...` — all 10 predicates PASS
- `go test -count=1 ./internal/sizeratchet/...` — PASS
- `gofmt -l go/cmd/evolve/cmd_carryover.go go/cmd/evolve/cmd_gc.go` — clean
- `go vet ./cmd/evolve/...` — clean
- `evolve acs suite --cycle 1753` — verdict=PASS, green=177 red=0 skip=53 total=230 (cycle=10 regression=214 red-team=6)
- `evolve selfcheck build` — GREEN

## Compatibility
No public CLI flag, exit code, or output message changes for `evolve gc` or
the carryover-apply path. Both new helpers are unexported, package-internal
additions with a single production caller each.

## Limitations
This cycle addresses only the two `top_n`-committed offenders; the remaining
262 entries in `go/internal/sizeratchet/offenders.json` are explicitly
deferred to future cycles (triage's `sizeratchet-offenders-remainder`
carryover item), not attempted here.
