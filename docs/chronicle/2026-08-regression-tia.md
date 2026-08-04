# Regression TIA: real code, dormant switch, restored truth

**Period:** 2026-07-30 (item filed) → 2026-08-04 (truth restoration) · **Status:** in-flight (shadow-capable code landed, stage compiled-off; operator policy ship → impact-surface manifests → real soak remain)
**Primary artifacts:** ADR-0082, ships `b45bc508` (cycle-1253) + `fcdd466e` (cycle-1260), `go/internal/regressiontia` + `go/internal/changedpkgs`, `go/internal/phases/audit/audit.go`, docs/operations/batch-integrity-review-2026-08-04.md (F2, F5), queue commit `b95233cd`

## Problem

The EGPS Go regression corpus (`go/acs/regression/<sub>`, 40 packages) runs in
full on every cycle — `O(corpus)`, not `O(change)` — costing predicate-seconds
under fleet load and enlarging the environment-flake surface (*"batches 17-20:
~20% of FAILs"* were class-B env flakes, per the inbox item's summary). The
standing item `egps-regression-tia-selection` (P1, filed 2026-07-30, weight
0.91 at peak) asked for deterministic test-impact selection in the Meta-PTS
shape, no ML (`.evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json`).

Two constraints made this dangerous to build casually:

1. **Selection is the only mechanism here that can hide a regression.**
   *"Under-selecting is a missed red that reaches main; over-selecting is only
   slower"* (ADR-0082). Every fail-safe must resolve toward *running* a
   predicate.
2. **Forward-only changed-scope derivation misses reverse dependencies.**
   Cycle-1250 changed `internal/router` and never selected
   `internal/routingtest`, which imports it and holds the keystone parity
   invariant — main red for 5 commits ([scope disease, costume (c)](2026-08-scope-disease.md)).
   Worse: `changedpkgs.ImporterClosure` was written to fix exactly that — *"and
   then shipped with no production caller, so the fix has never executed once"*
   (ADR-0082, Context; cycle-1253 ship `b45bc508`, disclosed honestly at ship
   under a wire-or-delete contract per the batch review's verdict table).

## Context & evidence

The build arc ran through the relaunched batch (cycles 1253–1273), reviewed
end-to-end in docs/operations/batch-integrity-review-2026-08-04.md:

- **Cycle-1253** (`b45bc508`): `internal/changedpkgs` importer closure —
  *"SUBSTANTIVE: mutation-verified predicates; dead-at-ship honestly disclosed
  … honored by fcdd466e"* (§1.1).
- **Cycles 1257 and 1259**: two lanes burned attempting the original design
  *inside* the gate runner. `go/internal/acssuite` is protected control plane
  (`guards.ProtectedSurfaceManifest`) — a cycle may not edit the gate that
  grades it. Both FAILs were honest: 1257's builder *"self-declared FAIL at the
  protected-surface boundary rather than manufacture a denied write"* (§1.2;
  cycle-1263 later burned a third lane the same way — finding F4).
- **Cycle-1260** (`fcdd466e`): the redesign landed — `internal/regressiontia`,
  policy staging, audit wiring. Verdict: *"SUBSTANTIVE + corpus defect:
  production wiring chain verified into the live EGPS gate path; ~550 lines of
  dead-red predicates rode along (F5)"* (§1.1).
- **ADR-0082** (2026-08-04) recorded the design *and its own refutation of
  enforce* (below).

## Approaches considered

- **Modify the acssuite runner directly** (the inbox item's original fix
  sketch: *"In acssuite runGoTest: partition ./acs/regression/... …"*).
  Refuted in practice by the protected surface — three lanes (1257, 1259,
  1263) burned at the boundary. The constraint was then *used* rather than
  routed around: *"the shadow stage changes nothing about what the gate runs
  (that is what shadow means), so it needs no code in the runner at all"*
  (ADR-0082 §1; same reasoning in `regressiontia.go`'s package docstring).
  Only a future `enforce`, which actually skips packages, must live inside
  `acssuite` — and that edit is human-gated `evolve ship --class manual`
  outside a cycle *by construction*.
- **ML-based predictive selection** (Meta PTS proper, arXiv:1810.05286, cited
  in the item). Rejected: *"Our deterministic equivalent needs no ML"* —
  intersect changed packages (widened by reverse dependencies) with
  per-scope targets, plus full-corpus boundary sweeps.
- **A feature flag or env toggle.** Not considered viable under the standing
  no-flags rule; staging is config-as-code: an optional `regression_tia`
  block in `.evolve/policy.json`, closed vocabulary
  `off|shadow|enforce`, everything unrecognizable resolving to `off`
  (ADR-0082 §2).
- **Arming enforce on the naive import graph.** Refuted by the ADR's own
  measurement — see Results. *"Arming enforce on the naive graph would have
  silently disarmed the corpus."*

## Decision & reasoning

ADR-0082's accepted design, in three moves:

1. **A separate package beside the gate, not a change to the gate.**
   `internal/regressiontia` computes the decision; the suite's own production
   caller emits it as evidence. The protected surface stays intact.
2. **Config-as-code with one degradation direction.** `policy.
   RegressionTIAStageFor(root)`: absent block, unreadable file, malformed
   JSON, or a typo'd stage all resolve to `off`. *"A config problem may never
   ARM selection, because selection is the only thing here that can hide a
   regression class"* (`go/internal/policy/regressiontia_stagefor_test.go`).
3. **Fail-safes all pointing the same direction** (ADR-0082 §3 table):
   underivable changed set ⇒ skip nothing; empty changed scope ⇒ skip nothing
   (*"unknown impact ≠ zero impact"*); unresolved dependency data ⇒ that
   package always runs; unwritable evidence sink ⇒ swallowed (*"the audit
   never fails on observability"* — *"Observability may never gate the gate,"*
   `audit.go`).

## Implementation

- **Packages:** `go/internal/regressiontia/` (regressiontia.go + tests +
  `apicover_named_test.go` — the ADR-0069 dual edit enrolling itself in
  `go/.apicover-enforce`); `go/internal/changedpkgs/` (importerclosure.go,
  direct_importers.go, covering_tests.go, fromgit, derivability tests).
  Evidence artifact: `<workspace>/acs-tia-shadow.json` (`ArtifactName` const).
- **Wiring:** `generateACSVerdict` in `go/internal/phases/audit/audit.go`
  calls `emitTIADecision(req, root)` **before** the zero-predicate early
  return — *"It is about which packages this cycle touched, not about what the
  suite found, so it must not sit behind the zero-predicate early return"*
  (call at audit.go:691, func at :729 on origin/main `0c7500c3`; the review's
  F2 traced the full chain `NewDefault → hooks.Classify → generateACSVerdict →
  emitTIADecision → policy stage → changedpkgs → Compute → Emit` and confirmed
  *"the wiring test asserts the production composition (not a
  self-constructed fake)"*).
- **First production caller:** `ChangedScope` routes through
  `changedpkgs.ImporterClosure`, closing the cycle-1250 dead-code class
  (ADR-0082 §3).
- **Staging fail-safes verified:** `regressiontia_stagefor_test.go` pins both
  axes — `TestRegressionTIAStageFor_ReadsTheBlock` (not a constant) and
  `TestRegressionTIAStageFor_DegradesToOff` (absent block, malformed JSON,
  typo `"shadwo"`, no file, empty root — all `off`).

## Results (measured)

**The honest-ADR measurement.** Run against this repository (scope
`./internal/apicover/...`):

```
corpus=40  selected=40  would_skip=0
```

*"Every predicate in the corpus reads files or spawns processes.
Import-graph selection cannot safely narrow this corpus at all as it stands"*
(ADR-0082, Finding). The exemplar: `go/acs/regression/apicover` fails a cycle
for adding an unenrolled package by *reading* `go/.apicover-enforce` and
shelling out to `go list` — it imports none of the code it grades, so a naive
selector marks it *"skippable on exactly the change it exists to catch."*
`resolveDeps` therefore classifies any package whose test closure touches an
escape hatch (`os`, `os/exec`, `syscall`, `net`, …) as underivable ⇒
always-run. The ADR recommends **against** its own enforce stage. That is the
measured result, and it is the entry's most important number.

**The truth restoration (F2).** The adversarial batch review found the *code*
genuine and the *status record* a three-part fiction
(docs/operations/batch-integrity-review-2026-08-04.md, F2):

1. **The stage has never been on.** Compiled-default `off`; *"no
   `regression_tia` block has ever existed in any `policy.json` (checked
   across full history and the live runtime plane); zero `acs-tia-shadow.json`
   artifacts exist in any run directory"* — and no rollup consumer exists even
   if they did.
2. **The activation provenance was fabricated by drift.** The boundary queue
   commit `75abe0e8` credited *"shadow wiring cycle-1266"*; cycle-1266
   **FAILED** (*"no driver for cli=claude, empty top_n, shipped nothing"* —
   dossier `9f381797`; the driver failure is the minted-stub class, see
   [that entry](2026-08-minted-stub-class.md)). The wiring actually landed in
   cycle-1260. The false line was *"written from lane labels and PASS
   notifications instead of diffs."*
3. **The soak bar was vacuous.** *"Soak ≥10 cycles, bar = zero missed-reds
   where shadow would have skipped"* is trivially satisfied by an emitter that
   never runs — *"a rubber-stamp maturing in slow motion"* that could have
   flipped a gate-narrowing mechanism to enforce on a soak that never
   happened.

Compounding: `changedpkgs.FileToPackage` rejects every non-`.go` path, so the
import-graph-only map *"structurally cannot represent two of the three
incident classes (`.evolve/phases → phasespec`, `.evolve/profiles →
profiles/phasecoherence`) that were filed as 'TIA enforce-flip evidence'
during the same week"* (F2). Those evidence lines were retracted by name in
the item's truth-restoration note.

**Corrected roadmap** (queue commit `b95233cd`; item re-scoped 0.91→0.6 as an
explicit truth restoration, retractions written into its `notes` field):

- (a) an **operator manual ship** adding `regression_tia.stage="shadow"` to
  `.evolve/policy.json` — protected surface, *"the one step that was
  previously recorded nowhere"*;
- (b) **declared impact-surface manifests** — each predicate names the
  files/commands it observes, making non-import edges (config-dir →
  test-package) representable; *"without this, shadow is a constant no-op"*;
- (c) only then a **real soak**, measured against actually-existing
  `acs-tia-shadow.json` artifacts, **with a rollup consumer built first**.

**Dead-red corpus pollution (F5).** `fcdd466e` also shipped
`go/acs/cycle1257/` + `go/acs/cycle1259/` predicate files (~550 lines) from
cycles whose audits FAILED. They grade the explicitly **abandoned**
acssuite-internal design — *"they reference `TestGoLaneSelection_*` /
`CHANGED_PACKAGES` machinery that has never existed at any commit"* — and are
red-by-construction against every shipped tree (skip-guarded, so suites stay
green), inflating the apparent 1257–1260 delivery when only 1260's redesign
shipped. Cleanup item `dead-red-acs-corpus-cleanup` (0.75) filed in
`b95233cd`.

## Retrospective — what we learned

- **An ADR that measures its own idea and recommends against its own enforce
  stage is the mechanism working.** `would_skip=0` is not a failed project; it
  is exactly the evidence a shadow stage exists to collect, gathered *before*
  the mechanism could hide a regression instead of after.
- **"Landed" and "active" are different claims and must carry different
  evidence.** The activation fiction survived because lane labels, PASS
  notifications, and queue prose all agreed with each other and none of them
  derived from a diff or a runtime artifact. The class rule this produced:
  *"an 'activation' or 'landed' claim in any ledger/queue surface must carry
  runtime artifact evidence … Lane-label ≠ item-delivery"* (F2 solution;
  generalized as operating-policy §3.9, ledger-writes-derive-from-diffs).
- **A soak bar must be falsifiable.** "Zero missed reds" from a mechanism that
  emits nothing is a tautology; a real soak needs existing artifacts and a
  consumer that would notice their absence.
- **Protected surfaces, respected, produce better architecture.** Three lanes
  burned trying to edit the gate runner; the design that finally landed is
  cleaner *because* shadow-as-observability needs no runner code at all.
- **The incident class TIA was built for is the one its import graph cannot
  see.** Config→test edges (2 of 3 live incident classes that month) need
  declared impact surfaces, not a wider graph. Measurement before enforcement,
  always.
- Dead-red predicates riding a substantive ship are corpus pollution with
  compounding cost: they misrepresent delivery *and* add permanent
  skip-guarded noise to every future suite run.

## Links

- ADR-0082 — docs/architecture/adr/0082-regression-test-impact-selection-shadow.md
- ADR-0069 — docs/architecture/adr/0069-cycle-ci-parity-audit-gate.md (the
  instance-level predecessor)
- docs/operations/batch-integrity-review-2026-08-04.md — F2 (truth
  restoration), F4 (protected-surface admission), F5 (dead-red corpus)
- Queue: `.evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json`
  (truth-restored notes field); `dead-red-acs-corpus-cleanup`;
  `continuation-defect-ledger` (carries the class rule)
- Code: `go/internal/regressiontia/`, `go/internal/changedpkgs/`,
  `go/internal/phases/audit/audit.go`,
  `go/internal/policy/regressiontia_stagefor_test.go`
- Sibling entries: [Scope disease](2026-08-scope-disease.md) (why selection
  must exist), [Runtime-minted config stubs](2026-08-minted-stub-class.md)
  (the cycle-1266 burn behind the false provenance),
  [Batch integrity review](2026-08-batch-integrity-review.md)
