---
score_cap:
  - criterion: "RecordRootTaskFailure is gone so lifecycle.Mover.Promote is the only quarantine writer"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1864_001|TestC1864_002' ./acs/cycle1864"
  - criterion: "ClaimLaneScope returns only []string and no function returns an always-nil error"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1864_003|TestC1864_004' ./acs/cycle1864"
  - criterion: "The missing-processing-dir test no longer skips and the decomposition doc names every remaining skip"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1864_006|TestC1864_007' ./acs/cycle1864"
---

# Eval: retire the dead inboxmover quarantine writer

> Pins the removal of `RecordRootTaskFailure` (zero production callers, a second quarantine writer beside `lifecycle.Mover.Promote`), the single-return `ClaimLaneScope`, and the skip-free amplified test. Source: inbox item inboxmover-dead-quarantine-writer, follow-up 06-F2 of ADR-0103 unit 06, cycle 1864.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single-writer | no reference to RecordRootTaskFailure, no verifylock or .bump.lock | 4/10 | `go test -tags acs -run 'TestC1864_001\|TestC1864_002' ./acs/cycle1864` |
| no-fake-error | ClaimLaneScope returns []string; AST finds no always-nil error | 6/10 | `go test -tags acs -run 'TestC1864_003\|TestC1864_004' ./acs/cycle1864` |
| skip-honesty | amplified test un-skipped; doc names remaining skips | 6/10 | `go test -tags acs -run 'TestC1864_006\|TestC1864_007' ./acs/cycle1864` |
