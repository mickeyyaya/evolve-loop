---
score_cap:
  - criterion: "DefaultRules() no longer contains depRule, and items whose only shared signal is a Deps reference bind zero edges"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC1724_001_DefaultRulesDropsDepRuleAndDepsAloneBindsNothing -v ./acs/cycle1724/"
  - criterion: "Classify orders a campaign-bound cluster by weight-desc, ignoring Deps (topoOrder's deps-ordering is gone)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC1724_002_ClassifyIgnoresDepsForOrdering -v ./acs/cycle1724/"
  - criterion: "campaign grouping and the opt-in ConnectsRule keep working unaffected"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -run TestC1724_003_ConnectsToAndCampaignGroupingUnaffected -v ./acs/cycle1724/"
  - criterion: "the triage prompt's inbox_batches wording names exactly the grouping signals the triage path applies (campaign, file-area; connects_to only if triage co-batches connects_to-linked items) and carries no dependency language"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -run TestC1724_004_InboxBatchesWordingNamesOnlySignalsTriageApplies -v ./acs/cycle1724/"
  - criterion: "the checked-in ACS predicates that pinned the three-rule DefaultRules (cycle1205, cycle1206, cycle1633) are re-derived to the two-rule set and still run and pass"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC1724_005_HistoricalRuleSetPredicatesRederived -v ./acs/cycle1724/"
  - criterion: "go build and the inboxbatch package suite pass after the removal, no regression"
    max_if_missing: 7
    evidence: "cd go && go build ./internal/inboxbatch/... && go test -count=1 ./internal/inboxbatch/..."
---

# Eval: Remove dormant inboxbatch dependency grouping

> Pins the cycle-1724 dead-code removal identified by the inbox record
> `inboxbatch-dormant-dependency-grouping`: under ADR-0106 W3 dependency_blocked
> routing, a dependent item is never offered in the same lane menu as its
> unlanded dependency, so `depRule` (go/internal/inboxbatch/rules.go:103-116,
> registered at rules.go:24) and the deps-ordering half of `topoOrder`
> (go/internal/inboxbatch/classify.go:86,142-233) are live-but-unreachable in
> production dispatch — dead code carried only by passing tests. Removing them
> must not touch campaign grouping or the opt-in `ConnectsRule`. The triage
> prompt's `inbox_batches` wording (go/internal/phases/triage/triage.go:256)
> must name only the signals the triage path applies. `selectableBatchesNote`
> classifies with `Config{}` (DefaultRules: campaign, file-area), so the
> wording may not claim dependency or connects_to grouping unless triage opts
> `ConnectsRule` in. The historical ACS predicates that pinned the three-rule
> set (cycle1205 001/005, cycle1206 001, cycle1633 008) must be re-derived in
> the same change, not left RED outside every gate's scope.
>
> Source incidents: a scout-identified dead-but-green defect (design doc
> `docs/architecture/logic-first-delivery-design.md` W8), and the cycle-1724
> audit round 1 rejection. D1: the first eval's AC2 required the wording to
> name connects_to, a signal triage never applies. D2: removing depRule left
> three historical predicates RED, and TDD found a fourth (TestC1633_008).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dead-rule-removal | depRule gone; Deps-only items bind zero edges | 8/10 | `go test -tags acs -run TestC1724_001_...` |
| dead-ordering-removal | topoOrder ignores Deps; weight-desc ordering holds | 8/10 | `go test -tags acs -run TestC1724_002_...` |
| unaffected-signals | campaign + opt-in connects_to keep working | 6/10 | `go test -tags acs -run TestC1724_003_...` |
| prompt-wording | inbox_batches wording names only the signals triage applies; no dependency language | 6/10 | `go test -tags acs -run TestC1724_004_...` |
| historical-pins | cycle1205/1206/1633 rule-set predicates re-derived and PASS | 7/10 | `go test -tags acs -run TestC1724_005_...` |
| no-regression | package builds and its suite is green | 7/10 | `go build ./internal/inboxbatch/... && go test -count=1 ./internal/inboxbatch/...` |
