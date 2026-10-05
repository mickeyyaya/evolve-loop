---
score_cap:
  - criterion: "release preflight runs the profiles, phasecoherence and phasespec suites"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1795_001 ./acs/cycle1795"
  - criterion: "a red repo-contract suite fails preflight with ErrCheckFailed naming the suite"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1795_002 ./acs/cycle1795"
---

# Eval: release preflight gates the repo-contract suites

> DefaultGateTestSuites listed only guards and phases/ship, so a release could cut with a red profile, phasecoherence or phasespec suite (v22.13.0 shipped assetless). Pins the three added suites and the failing-runner negative case.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| suites-run | three suites invoked by Run | 6/10 | `go test -tags acs -run TestC1795_001 ./acs/cycle1795` |
| red-blocks | red suite yields ErrCheckFailed | 7/10 | `go test -tags acs -run TestC1795_002 ./acs/cycle1795` |
