# Build Explanation — Cycle 2014

## Build Binding
- Cycle: 2014
- Base SHA: 297594d3107574b939f6d6466d0aceb86091c68e

## Summary
The cycle adds the read-only report of the retry ladder (component L7). `evolve ladder report` prints the tries, fixes and tokens for each rung and failure class.

## Rationale
The try records already carry the rung, the failure class, the exit code and the tokens. A pure aggregator over these records is the smallest design. It needs no new storage and cannot change a run.

## Changed Areas
- `go/internal/retryladder/report.go` — adds `BuildReport` (pure aggregation) and `LoadRuns` (reads each run dir ledger).
- `go/cmd/evolve/cmd_ladder.go` — adds the `ladder report` verb with `--json` and `--project-root`.
- `go/cmd/evolve/registry.go` — registers the `ladder` command.
- `go/internal/retryladder/apicover_named_test.go` — names the new exports for the apicover gate.
- `docs/operations/runtime-reference.md` — documents the verb and its exit codes.
- `docs/architecture/packages/internal-retryladder.md` — records the report design.

## Design Decisions
A fix is a try above R0 with exit code 0. A first try that passes is not a fix, and a retry without an exit code is not a fix. A record without a rung is left out, so pre-ladder ledgers cannot skew the counts. An absent runs dir is an error, and an empty one is an empty report.

## Verification
The retryladder and cmd/evolve tests cover exact counts, JSON fields, the no-write guarantee and the exit codes 2 and 10. The cycle 2014 ACS predicates run these tests.

## Compatibility
No existing command, file format or flag changes. The verb only reads.

## Limitations
The report has no filter by cycle or date. It counts every run dir under `.evolve/runs/`.
