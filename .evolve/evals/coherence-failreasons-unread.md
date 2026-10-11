---
score_cap:
  - criterion: "VerdictInputs declares no write-only FailReasons field"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1855_001 ./acs/cycle1855"
  - criterion: "A sentinel-less audit report reads as auditRan=true with empty verdict and never counts as forgery"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestReadCycleVerdicts_AuditReportWithoutSentinel ./internal/coherence"
  - criterion: "A sentinel-bearing PASS against a recorded FAIL still halts as verdict-incoherence"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1855_004 ./acs/cycle1855"
---

# Eval: coherence FailReasons removal and sentinel-less audit pin

> Pins the removal of the write-only `VerdictInputs.FailReasons` field and its writer in `core/failure_dossier.go`, and the outcome of an audit report that exists without a readable sentinel. Source incident: comment-reduction batch 28 (2026-09-26).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dead-field | FailReasons absent from VerdictInputs | 6/10 | `go test -tags acs -run TestC1855_001 ./acs/cycle1855` |
| sentinel-less | audit report without sentinel is coherent | 6/10 | `go test -run TestReadCycleVerdicts_AuditReportWithoutSentinel ./internal/coherence` |
| forgery-preserved | sentinel PASS vs recorded FAIL still halts | 7/10 | `go test -tags acs -run TestC1855_004 ./acs/cycle1855` |
