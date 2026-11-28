---
score_cap:
  - criterion: "The ACS subprocess cap plus the largest lane sum fits inside the checked-in acs.go_timeout_s"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSubprocessCapFitsGoTimeout$' ./pkg/acsassert/"
  - criterion: "The fit check rejects a policy whose go_timeout_s is too small or unset"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSubprocessCapFitsGoTimeout_RejectsATightPolicy$' ./pkg/acsassert/"
  - criterion: "The bound message names the caller deadline only when that deadline is the earlier bound"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestBoundLineNamesCallerDeadline$' ./pkg/acsassert/"
---

# Eval: The ACS subprocess cap fits the ACS Go timeout

> Pins the relation `subprocessCap + largest lane sum <= acs.go_timeout_s` against the checked-in `.evolve/policy.json`, and pins that the bound message names the caller deadline when it ends a call. Source: architecture review of the #1709 rebuild, 2026-11-23; cycle 2684.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| relation-over-real-policy | cap plus 187 s fits the policy value | 6/10 | `go test -run '^TestSubprocessCapFitsGoTimeout$' ./pkg/acsassert/` |
| negative-policy | 400 s, one second short and unset all fail | 6/10 | `go test -run '^TestSubprocessCapFitsGoTimeout_RejectsATightPolicy$' ./pkg/acsassert/` |
| caller-deadline-message | message names the caller deadline only when it is earlier | 7/10 | `go test -run '^TestBoundLineNamesCallerDeadline$' ./pkg/acsassert/` |
