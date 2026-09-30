# Comment history: `acs/cycle1633`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1633/predicates_test.go:3` — above `package cycle1633`

```text
// Package cycle1633 materializes the acceptance criteria for the single
// triage-committed task of this fleet lane:
//
//   - triage-unified-solution-synthesis → a validated, evidence-cited UNIFIED
//     commitment at the inboxbatch/triage seam that fails OPEN to independent
//     top_n selection, and is projected ONLY when valid: a small commitment
//     pins plan-review + build-planner through the phase registry, a large one
//     emits an ADR-0054 campaign plan. Per-member acceptance stays separate
//     (members ⊆ top_n; every member keeps its own acceptance contract).
//
// The second lane-scoped id, overlay-family-name-transport-ambiguity, was
// dropped by triage as already shipped (797b8518 — verified an ancestor of
// this tree; its five llmroute tests pass here). R9.3 forbids predicates for
// non-top_n work, so nothing below binds to it.
//
// Provenance: cycle 1629 built this exact contract to 20/20 predicates GREEN
// and an auditor-narrative PASS; the deterministic explanation-documentation
// gate forced FAIL on a path:line citation gap (audit-fail-reason.json), not
// on the code. Its snapshot (b5b2b669, branch cycle-cd3ae73e-1629) is the
// salvage source. This package re-pins the SAME wire contract so the salvaged
// implementation satisfies it, with one deliberate narrowing: 015 asserts
// wave membership through the existing fleet.CycleSpec.Scope, so the fix
// needs NO edit under go/internal/fleet (the file the 1629 gate tripped on).
//
// The contract pinned here (every symbol below is RED — the package does not
// compile until Builder provides it):
//
//	inboxbatch.UnifiedMember{ID, Evidence string}            json: id, evidence
//	inboxbatch.UnifiedCommitment{RootCauseHypothesis string   json: root_cause_hypothesis
//	                             SharedSeam string            json: shared_seam
//	                             DesignRequirements []string  json: design_requirements
//	                             Members []UnifiedMember}     json: members
//	func (c UnifiedCommitment) Validate(items []Item) error   nil ⇔ credible
//	func (c UnifiedCommitment) Size() string                  "small" | "large"
//	                                                          (len(Members) <= DefaultMaxItems ⇒ small)
//	router.TriageSignals.UnifiedSize string                   "" when absent/invalid
//	router.TriageSignals.UnifiedMemberCount int
//	routing field "triage.unified_size" (registry conditional_mandatory)
//	campaign.PlanFromUnifiedCommitment(c, items) (*campaign.Plan, error)
//
// triage-decision.json carries the agent's claim under the top-level key
// "unified_commitment". The triage PHASE (hooks.Classify, reached through the
// real runner) is the production validator: an invalid claim keeps verdict
// PASS (fail-open to the independent top_n), surfaces a diagnostic, and leaves
// NO unified signal for the router; a valid claim is projected by
// router.Digest.
//
// Predicate strategy — every predicate exercises the system under test (a
// direct call on the typed seam, the REAL triage runner driven by a fake
// core.Bridge, router.Digest/Route over the REAL phase registry, or a real
// emitted campaign-plan.json), never a source grep (the cycle-85 ban):
//
//   - 001–007: the typed contract — accept the complete/evidence-cited case;
//     reject missing evidence, unknown member, duplicate member, incomplete
//     shape, heterogeneous members (distinct campaigns / mixed deliverable
//     kinds — the forced-unification failure mode); size boundary pinned to
//     inboxbatch.DefaultMaxItems.
//   - 008: regression — the deterministic batch rules are untouched (a
//     commitment is a triage-declared artifact, not an inferred grouping rule).
//   - 009–011: the triage seam through the production runner — fail-open on an
//     invalid claim, projection of a valid small claim, rejection of a member
//     outside top_n (per-member acceptance stays separate).
//   - 012–014: the routing projection over the REAL phase registry — a small
//     commitment pins plan-review and build-planner even against an advisor
//     plan that declined them (Plan != nil bypasses insert_when; only
//     conditional_mandatory survives — router.shouldRun), and build-planner's
//     own ShouldSkip agrees; the no-commitment baseline is unchanged.
//   - 015–016: the campaign route — a large commitment projects to a Verify()-
//     clean campaign.Plan honoring member deps AND each member's own
//     acceptance contract; an invalid one is refused; the triage runner EMITS
//     campaign-plan.json in the workspace for large claims only.
//   - 017: this package is git-tracked (cycle-93: untracked predicates are
//     dropped at ship; cycle-1623 audit M1 recurrence).
//
// Adversarial axes (skills/adversarial-testing §6): NEGATIVE — 002/003/004/
// 005/006/009/011 and the invalid-claim half of 015 reject; a no-op that
// accepts everything fails them. EDGE — blank strings, zero/one member, exactly
// DefaultMaxItems vs DefaultMaxItems+1, a member outside top_n. SEMANTIC —
// validation (001–007), classifier isolation (008), phase fail-open (009–011),
// routing (012–014) and campaign projection (015–016) are five distinct
// behaviors, not one restated.
//
// Flaky-shape hygiene: no whole-package `go test` subprocesses, no wall-clock
// bounds, no literal PIDs, no bare `git` (every git call is -C rooted), no
// load generators.
```

### `go/acs/cycle1633/predicates_test.go:455` — above `t.Run("mixed deliverable kinds", func(t *testing.T) {`

```text
// A cycle has ONE authoritative deliverable kind (ADR-0099); a code item
// and a document item cannot be one solution.
```

### `go/acs/cycle1633/predicates_test.go:493` — above `if got := len(inboxbatch.DefaultRules()); got != 2 {`

```text
// A unified commitment is a triage-declared artifact validated AFTER
// classification; it must not become a grouping Rule. Three items with no
// campaign, disjoint areas and no deps stay three batches under the default
// rule set, and the rule set is still the two structural signals, campaign
// and file-area (rules_rootcause_regression_test.go forbids a prose
// root-cause rule; the dep rule was removed in cycle 1724 as unreachable
// under ADR-0106 W3).
```

### `go/acs/cycle1633/predicates_test.go:553` — above `items := threeItems()`

```text
// The Task Contract (ADR-0098) projects acceptance per top_n id. A member
// the decision did not commit has no separate acceptance reference, so the
// ONE solution could not be graded against it — reject, fail open.
```

### `go/acs/cycle1633/predicates_test.go:594` — above `large := chainItems(inboxbatch.DefaultMaxItems + 1)`

```text
// Symmetry lock (cycle-1638 audit H1): the plan-review half must never fork
// from the build-planner half on size. A LARGE commitment pins plan-review
// for the same reason 013 now pins build-planner.
```

### `go/acs/cycle1633/predicates_test.go:621` — above `large := chainItems(inboxbatch.DefaultMaxItems + 1)`

```text
// RECONCILED by TDD in cycle-1638 (audit round 1, H1). This block formerly
// asserted the INVERSE — "only SMALL commitments route through
// build-planner" — which projected how_to_apply step (3)'s EXECUTION split
// ("small unified fix = one cycle; large = emit an ADR-0054 campaign plan")
// onto ROUTING, a distinction the directive does not make there. Step (2) is
// unqualified by size: "a unified commitment routes through
// buildplanner+plan-review at deep tier"
// (.evolve/inbox/consumed/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json:30).
// The large commitment is the highest-blast-radius design the loop commits,
// so it is the LAST one that may escape planning. The size split stays where
// the directive puts it — in what execution EMITS — and 015/016 pin that
// campaign-plan half. Both ACS packages are added by this same diff
// (`git cat-file -e 4c58eb6d:go/acs/cycle1633/predicates_test.go` → absent),
// so this is one author reconciling their own contract, not a superseded
// inheritance.
```

### `go/acs/cycle1633/predicates_test.go:773` — above `func TestC1633_017_CycleACSPackageIsGitTracked(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// 017: ship-tree tracking of this package (cycle-93 / cycle-1623 M1)
// ---------------------------------------------------------------------------
```
