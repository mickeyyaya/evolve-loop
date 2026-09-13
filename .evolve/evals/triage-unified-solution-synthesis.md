---
score_cap:
  - criterion: "A complete, evidence-cited unified commitment over known inbox items validates and sizes small at <= inboxbatch.DefaultMaxItems members"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_00[17]_' ./acs/cycle1633/"
  - criterion: "Incomplete, evidence-less, unknown-member, duplicate-member and heterogeneous (distinct campaigns / mixed deliverable kinds) claims are rejected"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_00[23456]_' ./acs/cycle1633/"
  - criterion: "The triage phase fails OPEN on an invalid claim (verdict PASS, loud diagnostic, no unified signal, independent top_n preserved) and rejects members outside top_n"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_0(09|11)_' ./acs/cycle1633/"
  - criterion: "A validated small commitment is projected by router.Digest and pins plan-review + build-planner through the real phase registry, surviving an advisor plan that declined them; build-planner's own ShouldSkip agrees; the no-commitment baseline is unchanged"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_01[0234]_' ./acs/cycle1633/"
  - criterion: "A validated large commitment projects to a Verify()-clean campaign plan whose waves honor member deps and whose cycles each carry that member's own acceptance; the triage phase emits campaign-plan.json for large claims only"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_01[56]_' ./acs/cycle1633/"
  - criterion: "Deterministic batch classification is untouched — DefaultRules stays the three structural signals and unrelated items stay independent batches"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_008_' ./acs/cycle1633/"
  - criterion: "The cycle's ACS predicate package is git-tracked so the audit's predicate tree and the ship tree agree"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1633_017_|TestC1637_007_' ./acs/cycle1633/ ./acs/cycle1637/"
  - criterion: "A commitment mixing an unscoped member with a campaign-scoped member is heterogeneous: the typed seam rejects it for the campaign reason and the triage phase fails OPEN (PASS, loud, no unified signal, independent top_n intact)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1637_00[12]_' ./acs/cycle1637/"
  - criterion: "Members the triage persona already claimed into processing/cycle-<N>/ (Step 0a.4) are still known inbox items and the claim projects; a member in processed/ or in another lane's processing/cycle-<M>/ is rejected by name"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1637_003_' ./acs/cycle1637/"
  - criterion: "Members close transactionally with landing through the one lifecycle seam ship uses: the runner-emitted decision keeps every member in inboxmover.CommittedIDs, ApplyCycleOutcome PASS promotes all of them and FAIL promotes none"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1637_004_' ./acs/cycle1637/"
  - criterion: "router.Digest routes only on the projection the triage phase computed: a forged projection without a commitment is cleared, a forged one beside a valid claim is recomputed, a raw unvalidated claim never routes, and nothing routes before triage completes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1637_005_' ./acs/cycle1637/"
  - criterion: "A validated unified commitment (either size) raises plan-review and build-planner to the deep model tier through the production integrity-floor clamp, and records the raise as a Clamp; an absent or rejected claim leaves the advisor's proposed tier untouched"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_00[12]_' ./acs/cycle1638/"
  - criterion: "A LARGE unified commitment pins plan-review and build-planner to run, exactly as a small one does — the multi-cycle campaign path cannot outrun the design review; no commitment pins neither"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_003_' ./acs/cycle1638/"
  - criterion: "The plan-review persona, as the production prompt loader resolves it, names the patch-bundle failure mode, binds it to a REVISE/ABORT verdict, and states the general-abstraction rubric (single-source-with-projection, immutability, KISS floor)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_004_' ./acs/cycle1638/"
  - criterion: "The two production routing authorities agree on every commitment size: router.PhasePolicy.Enabled and the router.Route walk over the same live registry both pin plan-review and build-planner for small AND large, and both decline to pin them with no commitment"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_008_' ./acs/cycle1638/"
  - criterion: "build-planner's ShouldSkip surfaces a degraded routing digest as a diagnostic instead of self-skipping silently on zero-valued signals; a clean no-commitment cycle stays silent and a validated commitment runs the phase"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_009_' ./acs/cycle1638/"
  - criterion: "The build explanation names every load-bearing file the base-bound diff changed (production Go plus docs/architecture routing config), derived from git rather than from any report"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_010_' ./acs/cycle1638/"
  - criterion: "The build explanation does not attribute a go/acs package this diff ADDS to inherited history (preserved / pre-existing / inherited / older / prior cycle), checked against the base blob"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1638_011_' ./acs/cycle1638/"
---

# Eval: triage-unified-solution-synthesis — validated, evidence-cited unified commitment with fail-open routing

> Pins the contract for the operator directive of 2026-07-21 ("triage should
> pick multiple tasks and build ONE unified general solution"): triage MAY
> declare a typed `unified_commitment` (`inboxbatch.UnifiedCommitment` —
> root-cause hypothesis, shared seam, design requirements, evidence-cited
> members) in `triage-decision.json`; the triage phase validates it against
> the live inbox and the decision's own `top_n`, fails OPEN to independent
> selection when the claim is incomplete, unknown, duplicated, heterogeneous
> or over-reaching, and only a VALID claim is projected by `router.Digest`
> (`TriageSignals.UnifiedSize`) — a small one pins plan-review + build-planner
> via the phase registry's config-first `conditional_mandatory` rules, a large
> one emits an ADR-0054 `campaign-plan.json` whose cycles each keep their
> member's own acceptance contract. Deterministic `inboxbatch` grouping is
> unchanged (the cycle-1204 root_cause rule reversal stands: synthesis is a
> triage-declared, validated artifact, not an inferred edge). Source
> incidents: `triage-decision.json` top_n averaging ~1.5 items (cycles
> 974–981), campaign_retrospective_215_231 (12 defects = one disease), the
> cycle-1602 fleet conflict in which the item was selected but never
> materialised, and cycle 1629 — 20/20 predicates GREEN on this exact
> contract, FAILed only by the explanation-documentation citation gate
> (`.evolve/runs/cycle-1629/audit-fail-reason.json`); cycle 1633 re-pins the
> contract for the salvage (17/17 GREEN, FAILed by the same documentation
> gate); cycle 1637 continues that salvage and adds the two holes found by
> driving the production callers: (a) `Validate` accepted an unscoped member
> beside a campaign-scoped one (the mechanical `campaignRule` never binds
> those — bug-reproduction phase, cycle 1637), and (b) the persona's claim
> step (`evolve inbox-mover claim`, agents/evolve-triage.md Step 0a.4) moves
> every selected item to `processing/cycle-N/` BEFORE `hooks.Classify` runs
> `processUnifiedCommitment`, which loaded only the inbox ROOT — so no live
> commitment could ever validate. The 1637 rows also pin transactional
> member closure at the `inboxmover.ApplyCycleOutcome` seam ship uses and the
> router's trust boundary (only the validated projection routes).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| typed-contract-positive | complete evidence-cited commitment validates; size boundary = DefaultMaxItems | 6/10 | `go test -tags acs -run 'TestC1633_00[17]_' ./acs/cycle1633/` |
| typed-contract-negatives | missing evidence / unknown / duplicate / incomplete / heterogeneous rejected | 7/10 | `go test -tags acs -run 'TestC1633_00[23456]_' ./acs/cycle1633/` |
| triage-fail-open | invalid claim → PASS + diagnostic + no unified signal; member ∉ top_n rejected | 8/10 | `go test -tags acs -run 'TestC1633_0(09\|11)_' ./acs/cycle1633/` |
| routing-projection | Digest projects small; plan-review + build-planner pinned via real registry; ShouldSkip agrees; baseline unchanged | 8/10 | `go test -tags acs -run 'TestC1633_01[0234]_' ./acs/cycle1633/` |
| campaign-route | large → Verify()-clean plan, dep-ordered waves, per-member acceptance retained; triage emits campaign-plan.json; small does not | 7/10 | `go test -tags acs -run 'TestC1633_01[56]_' ./acs/cycle1633/` |
| batching-unchanged | DefaultRules == 3 signals; unrelated items stay independent | 5/10 | `go test -tags acs -run 'TestC1633_008_' ./acs/cycle1633/` |
| ship-tree-tracking | predicate packages are git-tracked (cycle-93 / cycle-1623 M1) | 5/10 | `go test -tags acs -run 'TestC1633_017_\|TestC1637_007_' ./acs/cycle1633/ ./acs/cycle1637/` |
| heterogeneity-campaign-partition | unscoped + campaign-scoped members rejected (typed seam + REAL runner fail-open) | 7/10 | `go test -tags acs -run 'TestC1637_00[12]_' ./acs/cycle1637/` |
| claim-state-resolution | members in processing/cycle-<N>/ validate; processed/ and other-lane claims rejected by name | 8/10 | `go test -tags acs -run 'TestC1637_003_' ./acs/cycle1637/` |
| transactional-closure | CommittedIDs keeps every member; PASS promotes all, FAIL promotes none | 7/10 | `go test -tags acs -run 'TestC1637_004_' ./acs/cycle1637/` |
| routing-trust-boundary | only the triage-computed projection routes; forged/raw/premature claims do not | 6/10 | `go test -tags acs -run 'TestC1637_005_' ./acs/cycle1637/` |
| planning-tier-teeth | validated commitment (small AND large) raises plan-review + build-planner to `deep` at the production floor clamp and records a Clamp; no commitment → advisor tier stands | 8/10 | `go test -tags acs -run 'TestC1638_00[12]_' ./acs/cycle1638/` |
| large-cannot-skip-review | `triage.unified_size==large` pins both planning phases, not only `==small`; no commitment pins neither | 8/10 | `go test -tags acs -run 'TestC1638_003_' ./acs/cycle1638/` |
| patch-bundle-rubric | production-resolved plan-reviewer persona names patch-bundle → REVISE/ABORT + the general-abstraction rubric | 7/10 | `go test -tags acs -run 'TestC1638_004_' ./acs/cycle1638/` |
| routing-authority-agreement | PhasePolicy and the router walk agree per size; both pin on a commitment, neither pins without one | 9/10 | `go test -tags acs -run 'TestC1638_008_' ./acs/cycle1638/` |
| degraded-digest-is-loud | build-planner reports a degraded digest instead of a silent self-skip; clean absence stays silent; validated commitment runs | 7/10 | `go test -tags acs -run 'TestC1638_009_' ./acs/cycle1638/` |
| explanation-names-the-mechanism | every load-bearing file in the base-bound diff appears in `## Changed Areas` | 6/10 | `go test -tags acs -run 'TestC1638_010_' ./acs/cycle1638/` |
| explanation-provenance-honesty | no history language about an acs package this diff adds (checked at the base blob) | 6/10 | `go test -tags acs -run 'TestC1638_011_' ./acs/cycle1638/` |

## Cycle-1638 addendum — planning teeth (how_to_apply step 2)

Cycles 1629/1633/1637 closed the SELECTION half of the 2026-07-21 operator
directive; the caps above them pin it and stay green in this tree
(`go test -tags acs ./acs/cycle1637` — 7/7). What stayed unbuilt is step (2):
`router.RoutingSignals.Triage.UnifiedSize` reached the routing vocabulary
(`triage.unified_size`, `internal/router/condition.go:83`) and the registry's
`conditional_mandatory` block, but nothing ever read it to raise a model TIER,
and the registry pinned the planning phases on `==small` only — so the
multi-cycle campaign, the largest design the loop commits, was the one case
that ran with no plan review at all. `agents/plan-reviewer.md` carried the four
lenses and the PROCEED/REVISE/ABORT aggregation but no rule for rejecting a
plan that is N patches wearing one commitment's clothes. The three caps added
here pin that gap closed.


## Cycle-1638 audit round 1 — the repair contract (008-011)

Round 1 REJECTED the build on two gate reasons and four findings. Three of the
four are pinned by the caps above; the fourth is an auditor-artifact defect with
no production-code half.

**H1 (HIGH) — two contradictory executable predicates over the same production
walk.** `go/acs/cycle1633/predicates_test.go` asserted "only SMALL commitments
route through build-planner" while `go/acs/cycle1638/predicates_test.go`
asserted every size must pin it; both drive the real registry, so no tree could
satisfy both and `TestC1638_005` (which re-runs the salvaged package) stayed
RED, producing the `EGPS red_count=1` gate reason. TDD owns this: all three acs
packages are ADDED by this diff (`git cat-file -e 4c58eb6d:<path>` → absent for
cycle1633, cycle1637 and cycle1638), so the conflict was authored wholly inside
this cycle. Reconciled at the source in favour of cycle-1638: `how_to_apply`
step (2) — "a unified commitment routes through buildplanner+plan-review at deep
tier" — carries no size qualifier, and step (3)'s small/large split governs what
EXECUTION emits (one cycle vs an ADR-0054 campaign plan), which
`TestC1633_015/016` still pin. `TestC1633_013` now asserts the reconciled
direction and `TestC1633_012` gained the matching plan-review case, so the two
planning phases cannot fork on size. The `routing-authority-agreement` cap pins
the SHAPE of the defect (two production authorities disagreeing) rather than
only its one instance.

**M1 (MEDIUM) — swallowed error disarms the cycle's own pin.**
`internal/phases/buildplanner/buildplanner.go:63` discards `router.Digest`'s
error and never consults `RoutingSignals.DigestDegraded`, while `ShouldSkip`
leaves its `[]core.Diagnostic` channel nil — so an unreadable
`triage-decision.json` zero-values the signals and the phase silently skips
itself, defeating the `conditional_mandatory` pin this cycle installs. Pinned by
`degraded-digest-is-loud`, whose anti-no-op half requires a clean cycle to stay
silent.

**M2 (MEDIUM) / H2 (HIGH) — the explanation must be findable and true.**
`docs/architecture/phase-registry.json` is the sole mechanism behind the size
pins yet appears in no `## Changed Areas` entry, and the round-1 narrative framed
the H1 conflict as "a contradictory **preserved** cycle-1633 assertion" when that
file does not exist at the base SHA. Both caps derive their expectations from
git (the base-bound diff and the base blob), never from a report that could agree
with a wrong answer.

**Not pinned here (no production defect):** the gate reason "explanation review
Evidence must cite `.evolve/inbox/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json`
with path:line evidence" is a defect in the AUDITOR's own artifact.
`internal/explanationdocs.ValidateReviewedHandoff` requires the audit report's
explanation-review Evidence to cite the document plus every host material path
at a concrete line, and `reportdoc.RequirePathLineEvidenceAt` resolves a path the
Build DELETED against the base blob — so the deleted root inbox copy must be
cited at its base path (e.g. `…/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json:30`),
not at its `consumed/` path.

## Cycle-1647 continuation — how 010/011 find the record (audit H1)

The `explanation-names-the-mechanism` and `explanation-provenance-honesty`
caps originally resolved "this cycle's explanation" with a
`docs/explain/builds/cycle-1638-*.md` glob. A continuation never ships that
record: `explanationdocs.ArchiveUnpublishedContinuationRecords` moves every
unshipped predecessor to `docs/private/research/archived-*/`, and the record
the tree ships is the continuing cycle's own — so both caps were RED on every
continuation tree by construction (cycle-1647 audit H1), while the shipping
1647 document repeated exactly the two defects they pin. `explanationDoc` now
resolves the ONE `docs/explain/builds/cycle-*.md` the tree adds (the index /
working-tree addition on a pre-commit lane; the record HEAD's latest
record-adding commit introduced once landed) — from git, never from a cycle
number — so the two caps grade whichever record ships. Their evidence
commands are unchanged; `.evolve/evals/overlay-family-name-transport-ambiguity.md`'s
`inherited-packages-green` cap makes the harness lane reach this package.
