# Build Explanation — Cycle 1724

## Build Binding
- Cycle: 1724
- Base SHA: c8bc27bbe07a5b12fb43608fa3c7e569f1a4eba1

## Summary
`depRule` and the deps-ordering half of `topoOrder` in `go/internal/inboxbatch` are removed: under ADR-0106 W3
`dependency_blocked` routing, a dependent is never offered in the same lane menu as its unlanded dependency, so a
batch grouped or ordered around a live `Deps` edge could no longer reach production dispatch — the code ran only
in tests. `Classify` now orders a cluster's members by weight-desc-then-id alone (`weightOrder`), and
`DefaultRules()` returns only `campaignRule` and `fileAreaRule`. The triage prompt's `inbox_batches` wording
now names only campaign and file-area, the two signals the triage path applies. The historical ACS predicates
that pinned the three-rule set are re-derived to the two-rule set in the same change.

## Rationale
The removal is the smallest change that matches the inbox acceptance criterion exactly: delete the two dead
consumers of `Item.Deps` inside `inboxbatch`, leave `Item.Deps` itself in place (it has a live consumer outside
this package — `internal/inboxmover`'s dispatch-state and dispatchability checks read it directly for the same
W3 routing that made `inboxbatch`'s dep grouping dead), and leave `campaignRule`, `fileAreaRule` and the opt-in
`ConnectsRule` untouched. Removing `topoOrder` entirely (rather than keeping a topological path with no live
input) avoids maintaining dead branching logic; `weightOrder` is the one ordering `Classify` already used for
its ready-set tie-break (`weightDescThenID`), so no new ordering concept is introduced.

## Changed Areas
- `go/internal/inboxbatch/rules.go` — deletes the `depRule` type and its `Edges` method, and drops it from
  `DefaultRules()`'s returned slice.
- `go/internal/inboxbatch/classify.go` — replaces `topoOrder` (Kahn deps-first traversal with a weight-desc
  fallback for cycles) with `weightOrder`, a direct weight-desc-then-id sort; updates the call site and a stale
  comment.
- `go/internal/inboxbatch/rules_rootcause_regression_test.go` — drops `depRule` from the pinned
  `wantDefaultRuleTypes` composition (now two rules, not three).
- `go/internal/inboxbatch/classify_test.go` — replaces the five dep-ordering/dep-grouping tests
  (`TestClassify_DepsOrderTopologically`, `TestClassify_DepOnMissingItemIsSatisfied`,
  `TestClassify_OversizedClusterChunksInTopoOrder`, `TestClassify_ContinuationNeverOutranksPredecessor`,
  `TestClassify_DepCycleFallsBackDeterministically`) with `TestClassify_DepsDoNotAffectGroupingOrOrdering` (pins
  that a Deps-only reference binds no edge and does not reorder a campaign-bound cluster) and
  `TestClassify_OversizedClusterChunksByWeight` (chunking/`DependsOnPrev` coverage without deps).
- `go/internal/phases/triage/triage.go` — `selectableBatchesNote`'s `inbox_batches` prompt line changes from
  "pre-grouped by campaign/file-area/links" to "pre-grouped by campaign/file-area". `selectableBatchesNote`
  classifies with `Config{}`, so only `DefaultRules()` binds (campaign, file-area). `ConnectsRule` is opt-in
  and has no production caller, so both "links" and "connects_to" would tell the triage LLM that linked items
  were co-batched when they were not. A dependent never reaches a batch either: W3 routes it to
  `dependency_blocked`.
- `docs/architecture/packages/internal-inboxbatch.md` — updates the Design/Invariants sections to drop the
  dep-topological ordering description and the three-rule default set, records the cycle-1724 removal, states
  that the triage wording names only the signals `Config{}` applies, and records the re-derived `go/acs/cycle1205`
  mutant literal and the predicates that pin the rule count.
- `go/acs/cycle1724/predicates_test.go` — the cycle's ACS predicates, authored by the TDD-engineer phase.
  Predicate 004 checks the wording against the batches the rendered prompt really forms. Predicate 005 runs the
  historical rule-set predicates. Build did not modify this file.
- `go/acs/cycle1205/predicates_test.go` — TDD-engineer re-derivation: `wantRuleTypes` (001) drops
  `inboxbatch.depRule`, and the overlay mutant target (005) becomes `return []Rule{campaignRule{}, fileAreaRule{}}`.
  Without it, TestC1205_001 and TestC1205_005 fail on every `-tags acs` run.
- `go/acs/cycle1206/predicates_test.go` — TDD-engineer re-derivation: TestC1206_001 drops the dep probe and
  compares the rule count to the probe count, two instead of three.
- `go/acs/cycle1633/predicates_test.go` — TDD-engineer re-derivation: TestC1633_008 expects two default rules
  instead of three.
- `.evolve/evals/inboxbatch-dormant-dependency-grouping.md` — the cycle's eval. After audit round 1 it requires
  wording that names only the applied signals, and it requires the historical pins to pass.

## Design Decisions
`Item.Deps` (the JSON field, `go/internal/inboxbatch/item.go:29`) is kept. It is out of this cycle's scope per
the inbox acceptance text ("delete the deps half of depRule/topoOrder", not the field), and it is not dead: a
grep across the module confirms `internal/inboxmover/dispatchstate.go:74` and
`internal/inboxmover/dispatchability.go:14,58` still read `.Deps` directly for W3 `dependency_blocked` gating.
Removing the field would break that live consumer; that removal is explicitly deferred to a follow-up cycle
(`inboxbatch-deps-field-removal` in `triage-report.md`'s deferred section) contingent on Build confirming zero
remaining consumers, which this cycle disproves.

## Verification
- `go test -tags acs -count=1 -v ./acs/cycle1724/`: 5/5 PASS
  (`TestC1724_001_DefaultRulesDropsDepRuleAndDepsAloneBindsNothing`,
  `TestC1724_002_ClassifyIgnoresDepsForOrdering`,
  `TestC1724_003_ConnectsToAndCampaignGroupingUnaffected`,
  `TestC1724_004_InboxBatchesWordingNamesOnlySignalsTriageApplies`,
  `TestC1724_005_HistoricalRuleSetPredicatesRederived`).
- `go test -tags acs -count=1 -v ./acs/cycle1205/ ./acs/cycle1206/ ./acs/cycle1633/`: 25/25 PASS (5 + 3 + 17).
- `go vet` over the changed packages: clean. `gofmt -l`: no output.
- `go test -count=1 ./...` (full module) and `evolve acs suite --cycle 1724`: results are recorded in
  `build-report.md`.

## Compatibility
`Item.Deps`'s JSON shape (`"deps": [...]`) is unchanged; only its consumption inside `inboxbatch` is removed.
`Rule` remains a 1-method interface; `ConnectsRule` and `campaignRule`/`fileAreaRule` keep their existing
behavior and signatures. `Batch.DependsOnPrev` keeps its meaning (a later chunk of the same cluster) — it no
longer additionally signals a crossed dependency chain, since no such chain is tracked any more. The triage
prompt loses the word "links" from one line; nothing parses that line.

## Limitations
This change does not remove `Item.Deps` or its JSON schema entry — that is deferred to a follow-up cycle
pending confirmation that `internal/inboxmover` is (or is made) its sole remaining consumer. It does not opt
`ConnectsRule` into the triage call site; whether connects_to should drive triage batching is a separate
decision. The historical packages `go/acs/cycle1205`, `cycle1206` and `cycle1633` are outside CI's ACS scope,
which runs only `./acs/regression`. `go/acs/cycle1724` predicate 005 is now the only gate that runs them.
