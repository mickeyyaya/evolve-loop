---
score_cap:
  - criterion: "Dead TestValidateProfile_BashParity is no longer registered in internal/subagent"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run TestC1906_001 ./acs/cycle1906"
  - criterion: "No internal/subagent source references the retired bash dispatch script"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run TestC1906_003 ./acs/cycle1906"
  - criterion: "ValidateProfile unit coverage survives the removal"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1906_004|TestC1906_005' ./acs/cycle1906"
---

# Eval: Remove dead bash parity test from internal/subagent

> Pins the removal of validateprofile_parity_test.go, which gated on EVOLVE_BASH_PARITY=1 and a legacy/scripts dispatch script retired with the Go-only runtime. Source incident: inbox item subagent-bash-parity-test-is-dead, cycle 1906.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unregistered | parity test gone | 6/10 | `TestC1906_001` |
| no-reference | no retired script reference | 6/10 | `TestC1906_003` |
| preserved | ValidateProfile tests intact | 7/10 | `TestC1906_004`, `TestC1906_005` |
