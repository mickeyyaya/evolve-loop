---
score_cap:
  - criterion: "A two-member commitment whose TDD declaration omits a member blocks at the TDD->Build boundary with a named scope-mismatch defect"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run ^TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks$ ./internal/topngate"
  - criterion: "A complete multi-member declaration still proceeds, in any order, and single-member lanes are unaffected"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestTDDScopeGate_(TwoSlugCompleteDeclarationProceeds|TwoSlugDeclarationOrderIsIrrelevant|SingleSlugLaneUnaffected)$' ./internal/topngate"
  - criterion: "The reconciliation is deterministic, covers N>2, and refuses a complete handoff shown inside an outer example fence"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestTDDScopeGate_(ScopeMismatchIsDeterministic|ThreeSlugPartialDeclarationBlocks|OuterFenceFakeCompleteHandoffStillBlocks)$' ./internal/topngate"
  - criterion: "Exactly one parser projects the lane-scope slug set, and it fails open on an absent or malformed pin"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run ^TestLaneScopeProjection_ ./internal/cycleoutcome"
  - criterion: "The abort happens at the TDD->Build boundary, not after the build spend the gate exists to save"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run ^TestTDDScopeGate_AppliesBeforeBuildNotAfter$ ./internal/topngate"
  - criterion: "The packages carrying the reconciliation stay vet-clean and race-green"
    max_if_missing: 5
    evidence: "cd go && go vet ./internal/topngate/... ./internal/cycleoutcome/... && go test -race -count=1 ./internal/topngate/... ./internal/cycleoutcome/..."
---

# Eval: Multi-slug lane scope reconciliation

> Pins the TDD->Build scope-coherence contract introduced for inbox item
> `multi-slug-lane-scope-reconciliation`. Source incident: cycle-1480
> (batch-20260815c wave-2) dispatched the two-slug bundle
> `minted-phase-verdict-contract-unsatisfiable` + `dead-api-sweep`. TDD minted a
> cycle-wide ACS predicate suite covering BOTH members while the Builder's
> deliverable contract bound only the FIRST, so slug 2 was entirely undelivered
> (`go/internal/core/phase_judge.go` unchanged, predicates 006/007/008 red) and
> the lane burned the full ~12-phase spine before FAILing at audit. Verbatim
> audit H1: "TDD minted a cycle-wide predicate suite covering both slugs while
> the Builder contract bound only the first slug; nothing reconciles the two
> scopes." It RECURRED at cycle-1483 (batch-20260816a) even though both inbox
> items carried do-not-re-bundle notes — notes do not reach the partitioner, so
> the fix has to be structural. Reproduced again in cycle-1620
> (`.evolve/runs/cycle-1620/bug-reproduction-report.md`): the enforce-stage
> reviewer returns `Approve:true` on the two-member/one-declared shape.
>
> This eval is the slug's durable machine memory (cycle-1501 lesson: a criterion
> set re-derived per attempt loses the criterion the previous attempt was graded
> CRITICAL on). Every grader below is a named, `-run`-narrowed test in a single
> small package, so a later cycle that regresses any half of the fix is capped
> without re-reading this incident.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cycle-1480 shape | omitted committed member blocks before Build, naming the member | 9/10 | `go test -run TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks ./internal/topngate` |
| anti-no-op | complete declaration proceeds (any order); single-member lanes unaffected | 8/10 | `go test -run 'TestTDDScopeGate_(TwoSlugCompleteDeclarationProceeds\|TwoSlugDeclarationOrderIsIrrelevant\|SingleSlugLaneUnaffected)' ./internal/topngate` |
| determinism + OOD | stable verdict, N>2 coverage, outer-fence fake handoff refused | 7/10 | `go test -run 'TestTDDScopeGate_(ScopeMismatchIsDeterministic\|ThreeSlugPartialDeclarationBlocks\|OuterFenceFakeCompleteHandoffStillBlocks)' ./internal/topngate` |
| single projection | one lane-scope parser; fail-open on absent/malformed pin | 7/10 | `go test -run TestLaneScopeProjection_ ./internal/cycleoutcome` |
| boundary | the abort lands at TDD->Build, not after the build spend | 6/10 | `go test -run TestTDDScopeGate_AppliesBeforeBuildNotAfter ./internal/topngate` |
| toolchain | touched packages vet-clean and race-green | 5/10 | `go vet ./internal/topngate/... ./internal/cycleoutcome/... && go test -race ...` |

## Why the outer-fence control is load-bearing

The gate's handoff reader takes the first fenced block inside `## Handoff to Builder` that DECLARES something (`slugs` or `testFiles`; a status object never counts). A
report that documents a COMPLETE handoff inside an outer `~~~markdown` example
fence therefore hands the gate a fake complete declaration while really
declaring one of two committed members. This is the same class the eval-quality
scanner already closed for grader bullets — "a `[code]`-styled bullet inside a
text/markdown fence is illustration (or a decoy planted to fake rigor), never a
real command" (`go/internal/evalqualitycheck/evalqualitycheck.go`). The handoff
reader owes the same rule: illustration is not declaration.
