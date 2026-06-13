---
score_cap:
  - criterion: "internal/triagecap statement coverage stays at or above the 93% floor"
    max_if_missing: 7
    evidence: "cd go && go test -cover ./internal/triagecap/... 2>&1 | awk -F'coverage: ' '/coverage:/{split($2,a,\"%\"); exit !(a[1]>=93)}'"
  - criterion: "the four formerly-0% functions (NewReviewer, readFailedApproaches, CommittedFloorPackages, readWindow) remain exercised"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/tc-eval.cover ./internal/triagecap/... >/dev/null 2>&1 && go tool cover -func=/tmp/tc-eval.cover | awk '/[[:space:]](NewReviewer|readFailedApproaches|CommittedFloorPackages|readWindow)[[:space:]]/{split($NF,a,\"%\"); if(a[1]+0==0) exit 1}'"
  - criterion: "the full triagecap suite passes under -race (no regression)"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 ./internal/triagecap/... >/dev/null 2>&1"
---

# Eval: triagecap uncovered-functions coverage

> Pins coverage of the four 0.0%-coverage functions in `internal/triagecap`
> closed in cycle 329 (harden): `NewReviewer` (the production constructor of the
> R9.2 triage-capacity clamp), `readFailedApproaches` (the ADR-0046 Layer 2
> demotion state.json read), `CommittedFloorPackages` (the Gate C committed-vs-
> deferred floor reconciliation), and `readWindow` (the rolling throughput
> window read). These were wired and exercised only indirectly through the
> seam-injected reviewer harness; cycle 329 added direct unit tests, raising the
> package from an 86.8% baseline to >= 93%. This eval is the PERMANENT regression
> entry that caps the audit score if any of the four falls dark again.
> Source incident: cycle 329 scout coverage survey (no prior research entry).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| package-floor | triagecap total coverage >= 93% | 7/10 | `go test -cover ./internal/triagecap/...` ≥ 93% |
| zero-cov-funcs | the four named functions are no longer 0% | 6/10 | `go tool cover -func` rows are non-zero |
| no-regression | full suite green under -race | 6/10 | `go test -race ./internal/triagecap/...` |
