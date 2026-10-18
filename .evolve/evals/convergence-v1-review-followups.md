---
score_cap:
  - criterion: "A seeded property test through Decide finds no Land, Split or AcceptWithLimits with an open CRITICAL, and runs 2000 to 5000 inputs"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestProperty_' ./internal/convergence | grep -q -- '--- PASS: TestProperty_'"
  - criterion: "The landing guard Stop names its own cause"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestLandingGuard_' ./internal/convergence"
  - criterion: "The once-only feedback change is pinned at max_fix_rounds 4"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestFeedbackChange_OnceOnlyAtMaxFixRoundsFour$' ./internal/convergence"
  - criterion: "Spec, ADR and package doc state the same once-only raise rule"
    max_if_missing: 8
    evidence: "grep -q 'once-only raise' docs/architecture/convergence-policy.md docs/architecture/packages/internal-convergence.md docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md"
---

# Eval: convergence v1 review follow-ups

> Pins the review follow-ups of the convergence policy: a landed round never holds an open CRITICAL,
> the landing guard Stop has its own cause, and the feedback change is made once. Source: cycle 1947,
> inbox item convergence-v1-review-followups.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| property | no Land with an open CRITICAL | 6/10 | `go test -run '^TestProperty_'` |
| guard cause | the guard Stop names itself | 7/10 | `go test -run '^TestLandingGuard_'` |
| once-only | the raise is pinned at N=4 | 7/10 | `go test -run TestFeedbackChange_OnceOnlyAtMaxFixRoundsFour` |
| docs | three docs agree | 8/10 | `grep 'once-only raise'` |
