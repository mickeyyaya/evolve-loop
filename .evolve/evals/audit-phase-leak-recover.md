---
score_cap:
  - criterion: "Binary rebuild churn during the audit phase is discarded and the cycle continues (binary restored to committed content)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_AuditLeakRecover/binary_churn_recovered' ./internal/core/"
  - criterion: "A non-artifact main-tree leak during audit still aborts the cycle (recovery is not a trust-kernel hole) and never reverts operator files"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_AuditLeakRecover/non_binary_leak_aborts|TestOrchestrator_AuditLeakRecover/mixed_leak_still_aborts' ./internal/core/"
  - criterion: "Audit-leak regression test file tracked"
    max_if_missing: 6
    evidence: "git ls-files --error-unmatch go/internal/core/orchestrator_auditleak_test.go"
---

# Eval: Audit-phase binary-churn leak recovery

> Pins the cycle-235 fix for the audit-phase cycle-killer: `evolve acs suite`
> run during AUDIT rebuilds `go/evolve` in the MAIN tree; the post-phase
> tree-diff guard saw the rebuilt binary as a sandbox leak and aborted the
> whole cycle (recoverBuildLeak only covered PhaseBuild). The fix adds a
> phase-agnostic build-artifact discard before the guard's abort: leaks that
> are ONLY tracked build artifacts (go/evolve, go/bin/evolve) are checked out
> back to HEAD, the guard re-checks, WARNs, and the cycle continues. The
> negative subtests are load-bearing: a real source leak must STILL abort,
> and the recovery must never revert non-artifact (operator) files — without
> them the "fix" could simply disable the guard. Source incident: cycle-234
> abort at audit start (inbox 2026-06-06T05-48-00Z-audit-leak-recover);
> binary-drift hazard per cycle-153.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| churn-recovered | audit binary churn → discard + continue | 5/10 | `go test -run 'TestOrchestrator_AuditLeakRecover/binary_churn_recovered' ./internal/core/` |
| guard-intact | real/mixed leak still aborts; no operator-file revert | 4/10 | `go test -run '…/non_binary_leak_aborts\|…/mixed_leak_still_aborts' ./internal/core/` |
| regression-test-tracked | test file in git | 6/10 | `git ls-files --error-unmatch go/internal/core/orchestrator_auditleak_test.go` |
