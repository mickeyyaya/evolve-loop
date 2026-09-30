# Comment history: `acs/cycle1638`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1638/predicates_test.go:3` — above `package cycle1638`

```text
// Package cycle1638 materializes the acceptance criteria for the single
// triage-committed task of this fleet lane:
//
//   - triage-unified-solution-synthesis → the operator directive that triage
//     synthesizes ACROSS backlog items and commits ONE unified general
//     solution, with PLANNING given teeth over that commitment.
//
// The second lane-scoped id, overlay-family-name-transport-ambiguity, was
// DEFERRED by triage as already shipped (797b8518, verified an ancestor of
// this tree: `git merge-base --is-ancestor 797b8518 HEAD` exits 0). R9.3
// forbids predicates for non-top_n work, so nothing below binds to it.
//
// # What is already built, and what this package adds
//
// Cycles 1633 and 1637 built the SELECTION half of the directive and it is
// preserved in this tree: inboxbatch.UnifiedCommitment.Validate (typed,
// evidence-cited, homogeneous-campaign, >=2 members), triage's
// processUnifiedCommitment (top_n binding, fail-open rejection, campaign-plan
// emission, claimed-item lifecycle resolution), router.Digest's
// UnifiedSize/UnifiedMemberCount projection, and transactional member closure
// at landing. go/acs/cycle1637 pins all of it and is GREEN here; 005 below
// re-runs that whole package as ONE named-package `go test` so the re-ship
// cannot regress it, rather than re-authoring 650 lines of the same contract.
//
// What is NOT built is how_to_apply step (2) — "PLANNING gets teeth: a unified
// commitment routes through buildplanner+plan-review AT DEEP TIER; plan-review
// verdict REVISE/ABORT if the design is a patch-bundle rather than a general
// abstraction". Verified absent by driving the production callers, not by
// reading code:
//
//  1. NO TIER TEETH. router.RoutingSignals.Triage.UnifiedSize is resolvable as
//     the routing field "triage.unified_size" (internal/router/condition.go:83)
//     and pins plan-review/build-planner to RUN via the registry's
//     conditional_mandatory block — but nothing anywhere reads it to raise a
//     model TIER. `grep -rn UnifiedSize` outside tests returns exactly three
//     sites (the field, the digest write, the condition read). So the deepest
//     design decision the loop makes is reviewed at whatever tier the advisor
//     happened to propose. 001 pins the raise at the production clamp; 002 is
//     the anti-no-op negative that forbids a blanket "always deep".
//  2. THE BIGGEST BUNDLES ESCAPE REVIEW ENTIRELY. The registry pins
//     plan-review and build-planner on `triage.unified_size==small` only, so a
//     LARGE commitment — the one that emits a multi-cycle ADR-0054 campaign
//     plan, the highest-blast-radius design the loop can commit — is the one
//     case that runs with no plan review at all. 003 drives the real
//     PhasePolicy over the real registry and requires both sizes to pin.
//  3. NO PATCH-BUNDLE RUBRIC. agents/plan-reviewer.md carries the four lenses
//     and the PROCEED/REVISE/ABORT aggregation, but says nothing about
//     rejecting a plan that is N patches wearing one commitment's clothes —
//     the exact failure mode step (2) names. 004 loads the persona through the
//     production loader and requires the rule to be reachable in the composed
//     prompt.
//
// Adversarial axes (skills/adversarial-testing §6): NEGATIVE — 002 (no
// commitment must not escalate), 003's no-commitment subtest, 001's
// clamp-recorded requirement (a silent raise fails). EDGE — the `large`
// commitment (003), a plan that proposes no tier at all (002). SEMANTIC — tier
// escalation (001/002), run-pinning (003), review rubric (004), prior-contract
// regression (005/006) are four distinct behaviors, not one restated.
//
// Flaky-shape hygiene: the two `go test` subprocesses each name ONE package
// (./acs/cycle1637, ./internal/router — measured 3.1s and 0.5s), no `/...`
// sweep, no ./internal/core or ./cmd/evolve, no wall-clock bounds, no literal
// PIDs, every git call is -C rooted, no load generators.
```

### `go/acs/cycle1638/predicates_test.go:339` — above `func TestC1638_007_CycleACSPackageIsGitTracked(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// 007: ship-tree tracking of this package (cycle-93 lesson)
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1638/predicates_test.go:354` — above `func declinedPlan() *router.PhasePlan {`

```text
// ---------------------------------------------------------------------------
// 008-011: the cycle-1638 audit-round-1 repair contract
//
// Round 1 REJECTED the build. Each predicate below encodes one auditor finding
// as a failing test, so the rebuild must address it rather than re-earn the
// verdict. The findings and their owners:
//
//   - H1 two contradictory executable predicates over the same production walk
//     (cycle1633:619 "large must NOT visit build-planner" vs cycle1638:236
//     "every size must pin build-planner"). RECONCILED at the source in this
//     same phase: how_to_apply step (2) is unqualified by size, step (3)'s
//     small/large split is about what EXECUTION emits. cycle1633's block now
//     asserts the reconciled direction; 008 pins the SHAPE of the defect so it
//     cannot re-fork -- the two production routing authorities (PhasePolicy and
//     the router walk) must AGREE on every size.
//   - H2 the Builder narrative framed H1 as inherited history. It is not:
//     all three acs packages are added by this diff. 011 makes that claim
//     mechanically false-able against the base tree.
//   - M1 buildplanner.ShouldSkip discards router.Digest's error AND
//     RoutingSignals.DigestDegraded, so a read failure zero-values the signals
//     and the phase silently self-skips -- disarming the very registry pin this
//     cycle installs. 009 drives the real ShouldSkip over a degraded workspace.
//   - M2 docs/architecture/phase-registry.json is the sole mechanism that makes
//     003 pass, yet the explanation document never names it. 010 derives the
//     required set from the real base-bound diff instead of hard-coding it.
//
// Not encoded here, because no production code is at fault: the audit's first
// gate reason ("Evidence must cite .evolve/inbox/2026-07-21T02-00-00Z-triage-
// unified-solution-synthesis.json with path:line evidence") is a defect in the
// AUDITOR's own artifact. internal/explanationdocs.ValidateReviewedHandoff
// requires the audit report's explanation-review Evidence to cite every host
// material path at a concrete line, and reportdoc.RequirePathLineEvidenceAt
// resolves a path the Build deleted against the BASE blob -- so the deleted
// root inbox copy must be cited at its base path, not at its consumed path.
// See test-report.md "Auditor checklist".
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1638/predicates_test.go:587` — above `func explanationDoc(t *testing.T) (string, string) {`

```text
// explanationDoc returns the path and body of the build explanation THIS TREE
// ships. It is deliberately NOT a cycle-1638-*.md glob: on a continuation the
// host archives every unshipped predecessor record under
// docs/private/research/archived-*/ (explanationdocs.
// ArchiveUnpublishedContinuationRecords), so the cycle-1638 draft is history
// and the deliverable is the ONE record the tree adds under
// docs/explain/builds/ — the index/working-tree addition on a pre-commit lane,
// or the record HEAD's own commit added once the cycle has landed. The former
// glob made 010/011 structurally unsatisfiable on ANY continuation tree
// (cycle-1647 audit H1); the intent — name every load-bearing path, no history
// attribution of this diff's own packages — binds to whichever record ships.
```

### `go/acs/cycle1638/predicates_test.go:757` — above `func TestC1638_011_ExplanationDoesNotAttributeThisDiffsOwnPredicatesToHistory(t *testing.T) {`

```text
// TestC1638_011 pins the H2 finding: the round-1 narrative framed the routing
// contradiction as inherited history ("a contradictory preserved cycle-1633
// assertion"), when `git cat-file -e <base>:go/acs/cycle1633/predicates_test.go`
// is ABSENT -- all three acs packages are added by this diff. The
// misattribution moved the fix to the wrong owner and cost the cycle a round.
```
