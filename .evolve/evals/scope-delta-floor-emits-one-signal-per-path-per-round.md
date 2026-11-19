---
score_cap:
  - criterion: "Three out-of-scope paths give exactly one ORCHESTRATOR_SCOPE_DELTA summary event with counts, and scope-delta.json lists the three paths"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestScopeDeltaFloorChecks_ReportsOneSummarySignalForAllOutOfScopePaths' ./internal/core"
  - criterion: "A second floor pass over the same tree emits no new event"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestScopeDeltaFloorChecks_ASecondPassOverTheSameTreeEmitsNoNewSignal' ./internal/core"
  - criterion: "A pass whose out-of-scope set changed emits exactly one new summary event"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestScopeDeltaFloorChecks_APassWhoseSetChangedEmitsOneNewSummary' ./internal/core"
---

# Eval: scope-delta floor emits one summary signal per round

> Pins the summary contract of `ScopeDeltaFloorChecks`: one ORCHESTRATOR_SCOPE_DELTA event per changed out-of-scope set, with the paths in the workspace artifact scope-delta.json. Source incident: cycle 2523 made 804 events (205 paths x 4 passes).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| summary-event | 3 paths give one event with counts and a 3-path artifact | 4/10 | `go test -run TestScopeDeltaFloorChecks_ReportsOneSummarySignalForAllOutOfScopePaths ./internal/core` |
| repeat-pass | same tree, second pass, no event | 5/10 | `go test -run TestScopeDeltaFloorChecks_ASecondPassOverTheSameTreeEmitsNoNewSignal ./internal/core` |
| changed-set | changed set, one new summary | 5/10 | `go test -run TestScopeDeltaFloorChecks_APassWhoseSetChangedEmitsOneNewSummary ./internal/core` |
