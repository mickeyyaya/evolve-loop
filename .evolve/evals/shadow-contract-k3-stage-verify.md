---
score_cap:
  - criterion: "The verdict of a stage is recomputed from a chain fixture and equals the golden counts and cases"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/stageevidence/ -run '^TestVerdictRecomputesFromFixtureChain$'"
  - criterion: "A changed scorer changes the verdict, so the golden is not a constant"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/stageevidence/ -run '^TestScorerMutationChangesGolden$'"
  - criterion: "The decision version digest stays stable under an edit outside the closure and the policy subtree"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/stageevidence/ -run '^TestVersionDigestStableUnderUnrelatedChange$'"
  - criterion: "The decision version digest changes with the decider closure and the policy subtree"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/stageevidence/ -run '^TestVersionDigestChangesWithDeciderClosure$'"
  - criterion: "Evidence of another decision version is reported and never counted"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/stageevidence/ -run '^TestEvidenceOfOtherVersionNotCounted$'"
  - criterion: "A broken chain gives not ready and a typed error with no partial verdict"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/stageevidence/ -run '^TestBrokenChainGivesNotReadyAndTypedError$'"
  - criterion: "Every stage key of the registry has a scorer and a report line through evolve stage report"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./cmd/evolve/ -run '^TestEveryStageHasScorerAndVerdict$'"
---

# Eval: evolve stage verify and stage report

> Pins the contract of the K3 component of the shadow stage plan (plan section 6, rules E3 and E4 of
> docs/architecture/shadow-stage-contract.md). The verb recomputes the readiness of a shadow stage from a
> hash chain alone, binds the verdict to a decision version digest, and gives a typed error on a broken chain.
> Source incident: cycle 2554 (inbox item shadow-contract-k3-stage-verify); no verb recomputed a verdict before.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| E3 recompute | The verdict equals the golden | 6/10 | `TestVerdictRecomputesFromFixtureChain` |
| E3 mutation | A changed scorer changes the verdict | 6/10 | `TestScorerMutationChangesGolden` |
| E4 stable | An unrelated edit keeps the digest | 7/10 | `TestVersionDigestStableUnderUnrelatedChange` |
| E4 closure | A closure or subtree edit changes the digest | 7/10 | `TestVersionDigestChangesWithDeciderClosure` |
| E4 binding | Evidence of another version does not count | 7/10 | `TestEvidenceOfOtherVersionNotCounted` |
| E1 and D8 | A broken chain is not ready with a typed error | 7/10 | `TestBrokenChainGivesNotReadyAndTypedError` |
| S4 | Every stage key has a scorer and a verdict | 7/10 | `TestEveryStageHasScorerAndVerdict` |
