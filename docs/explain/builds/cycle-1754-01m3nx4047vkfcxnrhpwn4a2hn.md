# Build Explanation — Cycle 1754

## Build Binding
- Cycle: 1754
- Base SHA: 37a65daa93266129021c17c65db958706b54b08b

## Summary
Two size-ratchet offenders, `cmd/evolve.runResetSHA` (51 lines) and
`cmd/evolve.setupLatestReport` (52 lines), now fit the ratchet's 50-line
ceiling. In each one, a self-contained block moved into a named helper that
is tested on its own. Both changes are behavior-preserving refactors: no CLI
flag, exit code or message changed.

## Rationale
`runResetSHA` resolved its project root inline. It tried the flag, then
`EVOLVE_PROJECT_ROOT`, then the cwd, and then made the result absolute with
`paths.AbsoluteRoot`. That decision block is separate from the re-pin logic
that comes after it. Moving it into `resetSHAProjectRoot` takes the function
from 51 lines to 41 and leaves the re-pin path unchanged.

`setupLatestReport` ran the whole per-CLI probe inside its goroutine literal:
tier lookup with the catalog override, timeout, list, and the stale/unverified
classification. Lifting that body into `probeCLILatest` makes each probe
branch testable without the WaitGroup fan-out, and takes the caller from 52
lines to 19. The fan-out itself is unchanged: one goroutine per ready CLI,
with each result written to its own slot.

## Changed Areas
- `go/cmd/evolve/cmd_resetsha.go` — the flag/env/cwd root resolution and the
  `paths.AbsoluteRoot` call move out of `runResetSHA` into
  `resetSHAProjectRoot(projectRoot, stderr) (string, error)`. The cwd error
  keeps its `cwd:` prefix, so the message on stderr is unchanged.
- `go/cmd/evolve/cmd_setup_latest.go` — the goroutine body of
  `setupLatestReport` becomes
  `probeCLILatest(ctx, c, catTiers, lister, fresh) setup.FamilyLatest`, and
  the goroutine now assigns `rows[i]` from it. The timeout and the per-row
  error isolation move with the body.
- `go/internal/sizeratchet/offenders.json` — removes the two entries, because
  both functions are now within the ceiling. This follows the removal
  convention of cycle 1753.
- `go/cmd/evolve/cmd_resetsha_test.go` — characterization tests for the
  flag > env > cwd precedence, relative flag and env roots, and a missing
  relative root that fails with the absolute state path (TDD-engineer
  authored; not modified by the build).
- `go/cmd/evolve/cmd_setup_latest_test.go` — `probeCLILatest` contract tests
  for the stale, lister-error, empty-listing, catalog-override and
  per-CLI-freshness branches (TDD-engineer authored; not modified by the
  build).
- `go/acs/cycle1754/predicates_test.go` — the cycle's seven ACS predicates
  (TDD-engineer authored).
- `.evolve/evals/shrink-resetsha-project-root-resolve.md`, `.evolve/evals/shrink-setuplatestreport-percli-probe.md` —
  score-cap eval definitions for the two tasks.

## Design Decisions
`resetSHAProjectRoot` returns an error rather than an exit code. The caller
keeps the one `evolve reset-sha:` prefix, and the existing "Mirrors
runShipCmd" rationale comment moved with the code it describes. A near-copy,
`loopStopRoot`, already exists in `cmd_loop_stop.go` but hard-codes the
`evolve loop-stop` WARN prefix. Consolidating the two would widen this diff
past the committed files, so it is recorded as a discovery instead.

`probeCLILatest` keeps the same parameter list as `setupLatestReport` (minus
the detect report), so the extraction is a pure lift, with no options struct
and no interface.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1754/` — 7/7 predicates PASS
- `go test -count=1 ./cmd/evolve/ ./internal/sizeratchet/` — PASS
- `gofmt -l cmd/evolve/` — clean; `go vet ./cmd/evolve/` — clean
- `evolve acs suite --cycle 1754` — verdict=PASS green=174 red=0 skip=53 total=227

## Compatibility
No public CLI flag, exit code or output message changes for
`evolve reset-sha` or `evolve setup latest`. Both new helpers are unexported
and each has one production caller.

## Limitations
The duplicated root-resolution logic in `loopStopRoot` is not consolidated.
The remaining entries in `offenders.json` are left for future cycles.
