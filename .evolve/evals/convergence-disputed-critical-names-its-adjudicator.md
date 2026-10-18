---
score_cap:
  - criterion: "a DISPUTED CRITICAL in a console-lane round yields CONVERGENCE_DISPUTED naming the operator"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestDisputedReason_NamesTheAdjudicatorOfTheLoop ./internal/convergence"
  - criterion: "a round with no DISPUTED row gains no reason and the decision is unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestDisputedReason_(Absent|LeavesTheDecision|ReadsOnly)' ./internal/convergence"
  - criterion: "the reason order stays rung, gain, concentration, disputed, settle"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestDisputedReason_SitsAfter|TestReasons_FollowTheRungInAFixedOrder' ./internal/convergence"
---

# Eval: a DISPUTED CRITICAL names its adjudicator

> Pins the `CONVERGENCE_DISPUTED loop= count= critical= adjudicator=` reason of `convergence.Decide`.
> A disputed CRITICAL never blocks a land (findings_test.go TestDisputed_NeverBlocksAndIsLeftToTheAdjudicator),
> so without this reason the console cannot see who must rule. Source: convergence-policy.md rule 5 (a dispute goes to the adjudicator).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| adjudicator-named | operator for console-lane, audit for code-review | 6/10 | `go test -run TestDisputedReason_NamesTheAdjudicatorOfTheLoop` |
| no-noise | no DISPUTED row, no line | 7/10 | `go test -run TestDisputedReason_Absent` |
| fixed-order | disputed sits after concentration | 6/10 | `go test -run TestDisputedReason_SitsAfter` |
