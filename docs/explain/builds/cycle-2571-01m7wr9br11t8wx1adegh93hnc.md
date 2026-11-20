# Build Explanation — Cycle 2571

## Build Binding
- Cycle: 2571
- Base SHA: 93775a47986454c5b38b92a036334ba0005e7f25

## Summary
The main-CI check now reads every run of the SHA. A green run wins over a newer cancelled run. A red run stays red.

## Rationale
A newer cancelled run of the same SHA made main read as not green. The batch then waited or refused. The fix changes only the verdict step, so the run listing and the watch of the newest run stay the same.

## Changed Areas
- `go/internal/integration/ghci.go` — lists up to 20 runs of the SHA and judges them together in `tipVerdict`. Watch and Peek both use it.
- `go/internal/integration/mainci_greenwins_test.go` — tests green over cancelled, red stays red, only cancelled stays not green, and no run stays no run.
- `go/acs/cycle2571/predicates_test.go` — the acceptance predicates of the cycle.
- `.evolve/evals/integration-accepts-any-green-run-for-the-sha.md` — the eval of the task.

## Design Decisions
Red in any run wins. Otherwise one green run wins. Otherwise the error of the newest run stands. Older runs that are still pending are ignored, and the newest run is still waited on.

## Verification
The integration package tests and the cycle ACS suite pass.

## Compatibility
No exported symbol changes. A SHA with one run behaves as before.

## Limitations
Only the 20 newest runs of a SHA are read.
