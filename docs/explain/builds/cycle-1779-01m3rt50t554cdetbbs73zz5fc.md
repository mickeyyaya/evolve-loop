# Build Explanation — Cycle 1779

## Build Binding
- Cycle: 1779
- Base SHA: 227081d81feb6c91c947e2260ec70ee6c488d884

## Summary
The cycle-run root now pauses on a quota wall, three cmd/evolve silent-failure sites are repaired, and the TDD scope gate no longer approves when triage-report.md is missing.

## Rationale
Each change closes a fail-open or divergence with the smallest edit: reuse the existing quota-pause emitter, the existing policy loader error, the existing paths.AbsoluteRoot helper, and the existing committed-set reader.

## Changed Areas
- `go/cmd/evolve/cmd_cycle.go` — runCycleRun builds its orchestrator through wireOrchestratorDepsFn and exits 5 on ErrAllFamiliesExhausted like the loop and resume roots; the composition root's policy WARN goes to the console it is given; a stale comment claiming an out-of-lane build aborts is removed.
- `go/cmd/evolve/cmd_consensus_dispatch.go` — a malformed policy.json is reported and exits 2 instead of dispatching under a silent default.
- `go/cmd/evolve/cmd_cycle_health.go` — the project root is absolutized before any path is derived.
- `go/cmd/evolve/cmd_cycle_signal_center_test.go` — the WithSignalCenter pin counts call expressions through the new countCallExprs, not comment text.
- `go/internal/topngate/gate.go` — tddScopeGate reconciles from triage-decision.json or lane-scope.json without triage-report.md and blocks when no commitment record exists.
- `go/internal/topngate/gate_test.go` — the fail-open subtest is replaced by a fail-loud one.
- `docs/operations/runtime-reference.md` — out-of-lane build is documented as advisory label drift; only the TDD gate blocks.
- `docs/architecture/packages/internal-config.md` — same correction for the TopNGate row.

## Design Decisions
A single committed member with no report falls back to that member as top_n; two or more use the existing member-set reconciliation; zero blocks naming the missing records.

## Verification
go test -count=1 for cmd/evolve, internal/topngate and internal/core/... pass, the cycle1779 ACS package passes with the acs tag, and evolve selfcheck build is GREEN.

## Compatibility
No exported symbol is added. Exit code 5 from cycle run is the same pause code the loop and resume roots already use.

## Limitations
The load-once policy read in cycle-health is not done; only the absolutized root and reported error are.
