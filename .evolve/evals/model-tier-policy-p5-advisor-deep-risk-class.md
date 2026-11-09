---
score_cap:
  - criterion: "A high-risk build routes to the deep tier, from classes that the registry data names"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2243_001|TestC2243_003' ./acs/cycle2243"
  - criterion: "A low-risk build never routes to the deep tier, and no risk class is a Go literal"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2243_002|TestC2243_004' ./acs/cycle2243"
  - criterion: "The replan path, the no-rule path and the no-evidence path keep their contracts"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2243_005|TestC2243_006' ./acs/cycle2243"
  - criterion: "The plan prompt names the policy classes and drops the blanket deep-build line"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2243_007 ./acs/cycle2243"
  - criterion: "Each function of the two rule files has 100 percent statement coverage"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2243_008 ./acs/cycle2243"
---

# Eval: the advisor sends a build to the deep tier by risk class

> Pins plan component P5 (decision D6 of docs/plans/model-tier-policy-2026-10.md). The advisor raised about half of all builds to deep, and deep builds use the scarce Claude pools. The rule now reads its risk classes from the `config.deep_build_risk` block of docs/architecture/phase-registry.json. Source incident: cycle 2243, inbox item model-tier-policy-p5-advisor-deep-risk-class.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| high-risk-deep | A high-risk build routes deep from data classes | 4/10 | `go test -tags acs -run 'TestC2243_001\|TestC2243_003' ./acs/cycle2243` |
| low-risk-not-deep | A low-risk build stays off deep, no Go literal class | 4/10 | `go test -tags acs -run 'TestC2243_002\|TestC2243_004' ./acs/cycle2243` |
| path-parity | Replan, no-rule and no-evidence paths keep their contracts | 6/10 | `go test -tags acs -run 'TestC2243_005\|TestC2243_006' ./acs/cycle2243` |
| prompt-policy | The prompt names the classes and drops the blanket line | 7/10 | `go test -tags acs -run TestC2243_007 ./acs/cycle2243` |
| full-coverage | 100 percent coverage on the rule functions | 5/10 | `go test -tags acs -run TestC2243_008 ./acs/cycle2243` |
