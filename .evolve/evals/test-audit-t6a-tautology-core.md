---
score_cap:
  - criterion: "No tautological testaudit finding remains in internal/core, and no baseline shard under it carries one"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2506_001' ./acs/cycle2506"
  - criterion: "Every flagged core test except the port-only Guard test is rewritten in place, none is deleted or renamed"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2506_002' ./acs/cycle2506"
  - criterion: "Each rewritten core test fails on a mutant of the production code it covers (predicates 003 to 020)"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2506_0(0[3-9]|1[0-9]|20)' ./acs/cycle2506"
  - criterion: "Every exported symbol of the apicover-enforced ./internal/core stays named by a test after the rewrites"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2506_021' ./acs/cycle2506"
---

# Eval: Fix the tautological test findings in internal/core (T6a)

> Pins the internal/core half of the T6 test-quality work. The testaudit analyzer reports 18 tautology-rule
> findings in internal/core: 13 no-assertion and 5 asserts-own-setup. Most are no-op or swallowed-failure
> tests that call a function and assert nothing. The mutant predicates compile the core test binary with a
> `go test -overlay` copy of one production file, so each rewritten test runs against the mutant.
> Source incident: cycles 2501 and 2504 (inbox items test-audit-t6d and t6b), where the first build dodged
> the detector with syntax changes instead of asserting behavior.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| zero-findings | no tautology finding remains in internal/core | 3/10 | `go test -tags acs -run TestC2506_001 ./acs/cycle2506` |
| rewrite-not-delete | every flagged test still exists | 4/10 | `go test -tags acs -run TestC2506_002 ./acs/cycle2506` |
| mutant-kill | each rewritten test fails on a production mutant | 3/10 | `go test -tags acs -run 'TestC2506_0(0[3-9]|1[0-9]|20)' ./acs/cycle2506` |
| apicover-named | no enforced core export is left unnamed | 4/10 | `go test -tags acs -run TestC2506_021 ./acs/cycle2506` |
