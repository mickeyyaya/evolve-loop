---
score_cap:
  - criterion: "No tautological testaudit finding remains outside internal/core, internal/bridge, cmd/evolve and acs, and no baseline shard carries one"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_001' ./acs/cycle2501"
  - criterion: "Every flagged test is rewritten in place, none is deleted or renamed"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_002' ./acs/cycle2501"
  - criterion: "The analyzer still separates a tautological test from a behavioral one, so the finding count is not zero by accident"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_003' ./acs/cycle2501"
  - criterion: "The protected-file carve-out of predicate 001 never grows past the 16 findings that the console follow-up owns"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_004' ./acs/cycle2501"
  - criterion: "Every exported symbol of an apicover-enforced package stays named by a test after the rewrites"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_005' ./acs/cycle2501"
  - criterion: "A container filled from production output is not flagged asserts-own-setup, so the detector is fixed and no caller changes syntax to dodge it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_006' ./acs/cycle2501"
  - criterion: "A container filled only from literals is still flagged asserts-own-setup"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2501_007' ./acs/cycle2501"
---

# Eval: Fix the tautological test findings in the other packages (T6d)

> Pins the rest-of-repo half of the T6 test-quality work. The testaudit analyzer reports 141 tautology-rule
> findings (`no-assertion`, `log-only`, `asserts-own-setup`) outside internal/core, internal/bridge, cmd/evolve
> and acs, in test files a lane builder may change. Each one is rewritten to assert a value a mutant of the code
> under test would change. Source incident: cycle 2501, inbox item test-audit-t6d-tautology-rest. Audit round 1
> rejected the first build: apicover left 4 exports unnamed (KeyBinding, ClaimOutcome, EditOp, OrphanReap), and
> about 40 asserts-own-setup false positives were dodged by a `make(map...)` syntax change instead of a fix of
> the detector rule.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| zero-findings | no tautology finding remains in scope | 3/10 | `go test -tags acs -run TestC2501_001 ./acs/cycle2501` |
| rewrite-not-delete | every flagged test still exists | 4/10 | `go test -tags acs -run TestC2501_002 ./acs/cycle2501` |
| detector-calibrated | analyzer separates tautology from behavior | 5/10 | `go test -tags acs -run TestC2501_003 ./acs/cycle2501` |
| protected-ceiling | the console-owned protected findings do not grow | 5/10 | `go test -tags acs -run TestC2501_004 ./acs/cycle2501` |
| apicover-named | no enforced export is left unnamed | 4/10 | `go test -tags acs -run TestC2501_005 ./acs/cycle2501` |
| detector-false-positive | production-filled container is not own setup | 4/10 | `go test -tags acs -run TestC2501_006 ./acs/cycle2501` |
| detector-true-positive | literal-filled container is still own setup | 5/10 | `go test -tags acs -run TestC2501_007 ./acs/cycle2501` |
