---
score_cap:
  - criterion: "audit repair replays cycles 1595 to 1605 with a budget of two: judge effort raise, fresh-context fixer, no deferral"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestAuditRepairConvergence_Replay ./internal/core"
  - criterion: "the shadow stage records CONVERGENCE_RUNG and the repair flow does not change"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestAuditRepairConvergence_Shadow ./internal/core"
  - criterion: "the enforce stage follows the decision and never lifts an envelope decline"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestAuditRepairConvergence_Enforce ./internal/core"
---

# Eval: Convergence V5, audit repair consults convergence.Decide

> Pins the adapter that builds a convergence.Input from the audit rounds and the call that records or follows the decision after each audit FAIL. Source: inbox item convergence-policy-v5-audit-repair-decide (cycle 2142), spec docs/architecture/convergence-policy.md section 9.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| replay | N = 2 shows the judge raise and the fresh-context fixer, no deferral | 6/10 | `go test -run TestAuditRepairConvergence_Replay ./internal/core` |
| shadow | signal recorded, flow identical for Stop and Continue | 7/10 | `go test -run TestAuditRepairConvergence_Shadow ./internal/core` |
| enforce | Stop ends repair, Continue keeps it, decline is never lifted | 7/10 | `go test -run TestAuditRepairConvergence_Enforce ./internal/core` |
