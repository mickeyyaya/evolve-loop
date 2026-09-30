---
score_cap:
  - criterion: "a lane without triage-report.md is reconciled from triage-decision.json / lane-scope.json"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/topngate -run '^TestTDDScopeGate_(ReconcilesFrom|MatchingDeclaration)'"
  - criterion: "no commitment record at all blocks rather than approving"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/topngate -run '^TestTDDScopeGate_NoCommitmentRecordAtAllFailsLoud$'"
  - criterion: "no doc or comment says an out-of-lane build aborts"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle1779 -run 'C1779_004'"
---

# Eval: topngate-tdd-scope-fail-open

> tddScopeGate.check failed open when triage-report.md was absent although the multi-member reconciliation reads other files; docs still said out-of-lane builds abort. Source: comment-reduction batch 16.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fail-open | reconcile without the report | 6/10 | `go test -run TestTDDScopeGate_ReconcilesFrom` |
| fail-loud | all records missing blocks | 6/10 | `go test -run TestTDDScopeGate_NoCommitmentRecordAtAllFailsLoud` |
| stale-docs | no "aborts" prose | 8/10 | `go test -tags acs -run C1779_004` |
