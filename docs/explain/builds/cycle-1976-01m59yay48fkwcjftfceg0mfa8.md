# Build Explanation — Cycle 1976

## Build Binding
- Cycle: 1976
- Base SHA: e66b2a66792670e650922b3f637176446143d611

## Summary
The new verbs `evolve handoff check` and `evolve handoff show` run the deterministic handoff layer and the exit decision on the ledger of a run directory, and print the result.

## Rationale
The packages `internal/handoff`, `evidence`, `exit` and `request` already decide pass, retry and blocked. The CLI only reads the inputs from disk and calls them, so no second decision path exists. The blocker prover of `internal/core` proves a `BLOCKER` claim; the CLI passes it the floor failures from `handoff-<pair>-failures.json`. A claim without proof stays `retry`. The rejected alternative was a new decision routine in the command: it would copy the exit logic and could drift from it.

## Changed Areas
- `go/cmd/evolve/cmd_handoff.go` — the verbs: argument parsing, request and ledger loading, evidence resolution, the prover wiring, text and JSON output, and the exit codes 0, 1, 2 and 10.
- `go/cmd/evolve/registry.go` — registers the `handoff` command in the CLI registry.
- `go/internal/repocontract/repocontract.go` — selects the handoff tests that read the whole tree in `TreeReadingTests()`, because the repo-contract pack fails otherwise.
- `go/cmd/evolve/cmd_handoff_cover_test.go` — covers the error branches of the verbs so each changed function reaches 100% statement coverage.
- `go/cmd/evolve/main.go` — adds the two verbs to the usage text.
- `docs/operations/runtime-reference.md` — documents the verbs, the input files, the outcomes, the JSON shape and the exit codes.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the 16 handoff tests that the TDD phase wrote without `t.Parallel`, because the parallel-default check fails otherwise.
- `go/cmd/evolve/cmd_handoff_test.go` — the unit tests of the verbs, written by the TDD phase and left unchanged.
- `go/acs/cycle1976/predicates_test.go` — the end-to-end predicates against the built binary, written by the TDD phase and left unchanged.
- `.evolve/evals/handoff-h7-cli.md` — the eval of the task, written by the TDD phase and left unchanged.

## Design Decisions
`--json` is read before the verb, so a usage error also prints one JSON object. `show` lists a pair as `absent` when its ledger file is missing, and does not build its request. A missing failures file means no floor failure. The explainable floor failures are the acs predicate failures only.

## Verification
`go test -count=1 ./cmd/evolve ./internal/handoff/...` passes with the EVOLVE_* variables unset, and `go test -tags acs -count=1 ./acs/cycle1976` passes all six predicates. The replays of cycles 1946 and 1950 print `blocked` with the protected path and the conflicting predicates.

## Compatibility
The change adds a command and changes no existing command or file format.

## Limitations
The command does not read the cover-strict floor failure headline, so a blocker that only a cover-strict failure explains stays `retry`. The `command` evidence kind never resolves, because the CLI has no list of recorded commands.
