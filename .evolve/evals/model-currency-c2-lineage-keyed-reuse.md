---
score_cap:
  - criterion: "A version bump inside one lineage makes no classifier call and promotes the newest version"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestRefresh_LineageBump_NoClassifierCall|TestRefresh_LineageBump_RetiredVersionPromotesReplacement' ./internal/modelquery"
  - criterion: "A new or vanished lineage still calls the classifier, and an unsound prior is never reused across a bump"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestRefresh_NewLineage_StillClassifies|TestRefresh_RemovedLineage_StillClassifies|TestRefresh_LineageBump_RefusedForUnsoundPrior' ./internal/modelquery"
  - criterion: "A freshness-policy change or a prior hash that is not derived from its recorded offering still calls the classifier across a bump"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestRefresh_LineageBump_FreshnessPolicyChangeStillClassifies|TestRefresh_LineageBump_PriorHashNotDerivedFromItsOfferingStillClassifies' ./internal/modelquery && go test -tags acs -count=1 -run 'TestC2147_006|TestC2147_007' ./acs/cycle2147"
  - criterion: "An unchanged non-default policy still carries across a bump with no classifier call"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestRefresh_LineageBump_UnchangedNonDefaultPolicyStillCarries ./internal/modelquery && go test -tags acs -count=1 -run TestC2147_008 ./acs/cycle2147"
  - criterion: "Every function of modelquery/query.go is at 100% coverage"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2147_005 ./acs/cycle2147"
---

# Eval: Model currency C2, lineage-keyed classifier reuse

> Pins the C2 component of docs/plans/model-currency-2026-10.md. liveTiers in
> go/internal/modelquery/query.go reused a prior classification only when the exact candidate
> fingerprint matched, so a version bump inside one lineage (gpt-5.5 to gpt-5.6) paid a classifier
> call. Cycle 2147 adds a lineage-set reuse path: the prior tiers, promoted through PromoteLatest,
> carry over with zero calls. A new lineage still classifies.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| bump-no-call | Version bump in one lineage: 0 classifier calls, newest version promoted | 3/10 | `go test -run TestRefresh_LineageBump_NoClassifierCall` |
| new-lineage-calls | New or vanished lineage and unsound priors still classify | 5/10 | `go test -run TestRefresh_NewLineage_StillClassifies` |
| invalidation-kept | Policy change or an underived prior hash classifies across a bump (audit round 1 H1) | 3/10 | `go test -run TestC2147_006\|TestC2147_007 ./acs/cycle2147` |
| carry-kept | Unchanged non-default policy still carries | 4/10 | `go test -run TestC2147_008 ./acs/cycle2147` |
| full-coverage | query.go functions at 100% | 4/10 | `go test -tags acs -run TestC2147_005 ./acs/cycle2147` |
