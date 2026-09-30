# Comment history: `internal/evalgate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/evalgate/flakyshape.go:15` — above `type flakyShapeGate struct{}`

```text
// flakyshape.go — Gate D (acs-metapredicate-suite-scope): THE production caller
// of the authoring-time flaky-shape lint.
//
// Why here and nowhere else. The lint reads go/acs/cycle<N>/predicates_test.go,
// which does not exist until the tdd phase has authored it — so the discover-time
// `evolve eval quality-check` invocation in skills/loop/phase2-discover.md can
// never see a predicate source, and a seam wired only into a CLI --help string is
// dead code. Gate D mounts on the SAME per-phase DeliverableReviewer seam as
// Gate B/C (core.WithReviewer, composed in NewReviewer, dispatched by the
// orchestrator at cyclerun_review.go), fires at the END of the tdd phase, and
// therefore inspects the predicates in the same transition the four-layer
// predicate-quality defense already owns (docs/architecture/
// acs-predicate-quality-gate.md) — one phase BEFORE the build tokens that a
// flaky predicate would later waste, and before acssuite's run-time scope-lint
// has to demote anything.
//
// ADVISORY, structurally. check returns block=false as a CONSTANT, not as a
// computed conjunct: a flaky SHAPE is a strong smell, not proof that the
// predicate is wrong, and calibration against the historical corpus (64 of 282
// acs dirs flagged, 2026-07-30) is not yet broken down per class. So a finding is
// surfaced by reviewer.Review's log line and never rejects a deliverable at any
// stage. That makes "this gate cannot fail a cycle" checkable by reading one
// line, which is the property the promotion path (advisory → enforce after a
// per-class false-positive breakdown) will need to deliberately revoke.
//
// KNOWN LIMIT on the surface, not the rule: reviewer.Review writes the reason to
// stderr (the phase log) only. Nothing persists it to an artifact, cycle state,
// or the Auditor's handoff, so the findings are operator-visible but not yet
// agent-consumable — and the per-class audit the promotion path needs cannot be
// fed from live runs until they are. Queued, deliberately out of scope here:
// wiring an advisory-finding channel is a reviewer-seam change, not a lint change.
```

### `go/internal/evalgate/floorbinding.go:19` — above `type floorBindingGate struct{}`

```text
// floorbinding.go — Gate C (R9.3, triage capacity): EGPS floor predicates
// must bind ONLY floors triage committed this cycle. The cycle-280 failure
// mode: TDD authored coverage-floor predicates for tasks triage had
// DEFERRED, and the builder starved the committed task clearing gates that
// were never this cycle's work. The check is fully deterministic:
//
//  1. extract the target packages of coverage-floor predicates from this
//     cycle's acs package (go/ast over predicates_test.go — floor predicates
//     name their target as a path literal, e.g. "./internal/core/");
//  2. ask the triage artifact which of those packages appear in
//     floor-bearing ## deferred / ## dropped items;
//  3. any overlap is a CERTAIN violation → block at enforce.
//
// Fail-open on every ambiguity: missing predicates file, unparseable Go,
// missing triage artifact, or no floor predicates at all.
```

### `go/internal/evalgate/floorbinding.go:55` — above `_, deferredDeclared, _ := triagecap.ReadDeferredFloors(companionPath)`

```text
// Committed-wins subtraction: a package floor-committed this cycle may
// carry predicates even if more of its work was ALSO deferred for later
// (cycle 310: the gate blocked the committed package's own predicates).
// Provenance rule (declarations outrank prose, the Layer-1 contract): a
// DECLARED deferred_floors entry yields only to a DECLARED committed
// floor; prose-derived deferral yields to committed evidence of any rank.
```

### `go/internal/evalgate/floorbinding.go:88` — above `var cycleDirRE = regexp.MustCompile('^cycle-(\d+)$')`

```text
// cycleNumFromWorkspace parses N from the run-dir basename "cycle-<N>".
// Sub-paths of the workspace (e.g. cycle-300/artifacts) return 0 → fail
// open; the orchestrator always passes the workspace root.
```

### `go/internal/evalgate/floorbinding_test.go:20` — above `const deferredCorePredicates = '//go:build acs`

```text
// floorbinding_test.go — R9.3 (triage capacity): EGPS floor predicates must
// bind ONLY floors triage committed this cycle. The pin the plan names:
// deferred tasks ⇒ zero binding predicates (cycle-280: TDD authored floor
// predicates for deferred tasks; the builder starved the committed task
// clearing gates that were never this cycle's work).
```

### `go/internal/evalgate/floorbinding_test.go:67` — above `func buildFloorBindingFixture(t *testing.T, predicates string) core.ReviewInput {`

```text
// buildFloorBindingFixture lays out workspace (triage artifact) + worktree
// (predicates file) for cycle 300.
```

### `go/internal/evalgate/floorbinding_test.go:103` — above `const triageWithDualListedCore = '## top_n (commit to THIS cycle)`

```text
// triageWithDualListedCore commits a core floor AND defers more core work —
// the committed listing must win (a floor predicate on this cycle's own
// committed package is a legitimate ratchet, never a cycle-280 starvation).
```

### `go/internal/evalgate/floorbinding_test.go:252` — above `const committedCoreNoDeferred = '## top_n (commit to THIS cycle)`

```text
// ----------------------------------------------------------------------------
// ADR-0046 Layer 1 (cycle 305): the floor-binding gate consumes the triage
// companion's deferred_floors[] declaration instead of scraping ## deferred /
// ## dropped prose. Builder rewires floorBindingGate.check() to read
// <workspace>/triage-decision.json via the declaration-primary
// triagecap.DeferredFloorPackagesDecl wrapper.
//
// RED guarantee: TestFloorBinding_DeclaredDivergenceMessage references the
// not-yet-existing symbol triagecap.DeferredFloorDivergence, so this whole test
// package fails to compile until Builder lands the Layer-1 functions — every
// pin below is RED in the unbuilt tree. The behavioral pins
// (DeferredFromCompanion, ProseIgnoredWithCompanion) additionally fail on
// behavior once the package compiles but the gate still reads prose.
// ----------------------------------------------------------------------------
```

### `go/internal/evalgate/floorbinding_test.go:336` — above `func TestFloorBinding_ProseIgnoredWithCompanion(t *testing.T) {`

```text
// N1: a companion declaring a DIFFERENT deferred package makes prose-deferred
// core non-authoritative — the gate must NOT block core. This is the cycle-280
// class retirement: prose can no longer over-bind a floor the agent committed.
```

### `go/internal/evalgate/gate_wiring_registry_test.go:1` — above `package evalgate`

```text
// gate_wiring_registry_test.go — cycle-996 root-cause guard for the
// gate-wiring-binding-tests inbox item.
//
// The per-gate wiring pins (TestQualityGate_WiredIntoReviewer,
// TestFloorBindingGate_WiredIntoReviewer) bind two named gates into
// NewReviewer's composed slice (reviewer.go:39), but nothing FORCES a NEW gate
// added to that slice to carry such a pin — the exact class the item targets
// ("nothing forces cross-package/wiring binding tests for gate enforce paths").
// materializationGate ("evals-materialized") is currently composed yet has no
// wiring pin at all, proving the gap is live.
//
// These two default-suite meta-tests turn the per-gate discipline into a
// forcing function: every gate name returned by newGatesForTest() (the REAL
// production slice) must be registered in pinnedGateWirings, and the registry
// may not carry an entry for a gate no longer composed. Adding a gate to the
// production slice without registering its wiring pin fails the build loudly,
// naming the offender.
```

### `go/internal/evalgate/gates_test.go:34` — above `body := "# Eval " + slug + "\n\n- [code] '" + bashBody + "'\n\n'''bash\n" + bashBody + "\n'''\n"`

```text
// A [code] grader bullet: the form the materialization gate requires of a
// scout-written eval (cycle 1679) — the bash fence below is the legacy form.
```

### `go/internal/evalgate/materialization.go:18` — above `type materializationGate struct{}`

```text
// materializationGate (Gate A) enforces the scout contract: every slug scout
// SELECTED must have a real .evolve/evals/<slug>.md file. It fires after the
// scout phase, before triage/tdd/build spend tokens (cycle-166).
```

### `go/internal/evalgate/materialization.go:63` — above `scope := "Gate A therefore checked NOTHING this cycle: a selected task with no eval file " +`

```text
// What Gate A actually checked depends on whether the "## Decision Trace"
// still yielded slugs. On a parse-miss the Selected Tasks body contributes
// none, so a non-empty union here came from the trace alone and the gate did
// check those — measured 2026-09-15, 3 of the 59 real cycle-16* fires. Saying
// it checked NOTHING there is simply false (audit L1, cycle 1685).
```

### `go/internal/evalgate/materialization_graders_test.go:12` — above `func TestMaterializationGate_RequiresACodeGraderPerEval(t *testing.T) {`

```text
// TestMaterializationGate_RequiresACodeGraderPerEval pins the rule this
// gate's own remediation has always promised ("Each must contain at least
// one `[code]` grader") but never checked: an eval that exists with no
// [code] grader passed here, reached the build, and was refused by the
// lane's own durability test at the SHIP gate (cycle 1679, 2026-09-14) —
// after which the builder could not fix it (its sandbox denies .evolve/evals)
// and two repair rounds burned. The scout owns the eval; the gate must fail
// the scout while the scout still holds the pen.
```

### `go/internal/evalgate/materialization_remediation_test.go:27` — above `func scoutWorkspaceSelecting(t *testing.T, slugs ...string) (projectRoot, workspace string) {`

```text
// scoutWorkspaceSelecting builds a workspace whose scout-report selects slugs,
// with no eval files anywhere — the exact cycle-1531 shape.
```

### `go/internal/evalgate/materialization_remediation_test.go:32` — above `var b strings.Builder`

```text
// Mirrors the REAL scout-report shape (cycle-1531): slugs reach the gate via
// the "## Decision Trace" JSON, which is the path production actually uses.
// Note the prose fallback (slugLineRE) does NOT match the persona's own
// backticked "- **Slug:** `x`" form, so the trace is the only live source.
```

### `go/internal/evalgate/materialization_wiring_test.go:1` — above `package evalgate`

```text
// materialization_wiring_test.go — cycle-996 wiring pin for materializationGate
// ("evals-materialized") in NewReviewer's composed gate list (reviewer.go:39).
//
// materialization_test.go exercises materializationGate{}.check() DIRECTLY but
// never through NewReviewer(...).Review(...), so deleting materializationGate{}
// from the composition slice passes the direct-.check() tests while silently
// dropping the scout-phase evals-materialized enforcement. This DEFAULT-SUITE
// test closes that blind spot, mirroring TestQualityGate_WiredIntoReviewer and
// TestFloorBindingGate_WiredIntoReviewer, and is the pin registered for
// "evals-materialized" in pinnedGateWirings (gate_wiring_registry_test.go).
```

### `go/internal/evalgate/monotonic.go:13` — above `var binaryAbsoluteTargetRe = regexp.MustCompile('(?i)\bto\s+(?:<=|≤|=<|at most|no more than|fewer than|less than|under|b…`

```text
// binaryAbsoluteTargetRe matches an acceptance criterion that states where a
// count must END UP ("to <=25", "to at most 25", "to under 25") rather than how
// far it must MOVE. On a monotonic task that phrasing is all-or-nothing: a
// cycle that verifiably pruned 101 of 140 items scored zero and the work was
// discarded (cycle-992). Deliberately prose-tolerant — the defect is the
// phrasing pattern, not one operator spelling.
```

### `go/internal/evalgate/parsemiss_test.go:1` — above `package evalgate`

```text
// parsemiss_test.go — cycle-1685 regression pins for the parse-miss vs
// convergence distinction (inbox id `evalgate-selectedslugs-nil-blindness`).
//
// SelectedSlugs returns nil for two categorically different scout-report shapes,
// and until this cycle nothing downstream could tell them apart:
//
//  1. genuine convergence — no "## Selected Tasks" section at all; fail-open is
//     CORRECT, the cycle claimed no work;
//  2. format drift — the section IS present with real task prose whose slug the
//     parser cannot read, so it yields zero slugs and Gate A's fail-open path
//     checks NOTHING while believing it checked everything.
//
// Shape 2 is cycle-1570: scout selected `config-gate-default-policy-authority`,
// authored no eval, the section parsed to nil, Gate A approved, and the missing
// eval surfaced three phases later as an audit H1.
```

### `go/internal/evalgate/parsemiss_test.go:32` — above `const cycle1570ReportShape = "# Scout Report\n\n## Selected Tasks\n\n" +`

```text
// cycle1570ReportShape is the REAL incident shape, not a synthetic stand-in: a
// "## Selected Tasks" section carrying genuine task prose for
// config-gate-default-policy-authority whose slug is stated as free prose
// ("Task slug: ...") instead of the "- **Slug:**" bullet slugLineRE requires.
// Neither empty nor absent — and it parses to zero slugs.
```

### `go/internal/evalgate/parsemiss_test.go:43` — above `func TestCycle1570ReportShape(t *testing.T) {`

```text
// TestCycle1570ReportShape pins the incident itself. The first assertion is the
// fixture PREMISE: this shape must still parse to zero slugs, or the predicate
// below is exercising nothing. The second is the fix.
```

### `go/internal/evalgate/parsemiss_test.go:192` — above `t.Run("claims nothing-was-checked only when the union is empty", func(t *testing.T) {`

```text
// The advisory must not claim more than it knows. Gate A checked NOTHING only
// when the union is genuinely empty; when the "## Decision Trace" still
// supplied slugs the gate DID check those, and on 3 of the 59 real cycle-16*
// fires it had (audit L1, cycle 1685).
```

### `go/internal/evalgate/parsemiss_test.go:267` — above `const cycle1664ReportShape = "# Scout Report — Cycle 1664\n\n" +`

```text
// --- the slugLineRE widening's measured effect ---------------------------------
//
// Round 1 of this cycle shipped a NEUTRALITY CLAIM about the backtick widening —
// "the union SelectedSlugs returns is unchanged and no report becomes newly
// blockable" — asserted from a count of "## Decision Trace" HEADINGS. Heading
// presence is not union equality, and the claim was false: re-measured
// 2026-09-15 over .evolve/runs/cycle-16*/scout-report.md, the widening changes
// the union on 6 of 84 reports and makes 2 of them newly blockable at Gate A.
//
// The corpus is gitignored and absent in CI, so the correction cannot live only
// in prose citing it. These fixtures reproduce the two falsifying reports so the
// claim is RE-RUN on every `go test ./internal/evalgate`, which is the half
// round 1 was missing.
```

### `go/internal/evalgate/parsemiss_test.go:320` — above `func TestSlugLineWideningIsNotBlockingNeutral(t *testing.T) {`

```text
// TestSlugLineWideningIsNotBlockingNeutral executes the corrected claim.
//
// Union half: on both reports the "## Decision Trace" is present but yields
// nothing and the pre-widening pattern matches nothing, while the shipped
// SelectedSlugs returns the slug — so the union the widening produces differs.
//
// Blocking half: driving the production reviewer at StageEnforce over an A/B
// pair that differs ONLY by that bullet, Gate A BLOCKS with it and fail-opens
// without it. That delta is what "newly blockable" means, and it is the sentence
// round 1 got wrong.
```

### `go/internal/evalgate/parsemiss_test.go:336` — above `if !strings.Contains(tc.report, "## Decision Trace") {`

```text
// Premise: these are reports that DO carry the heading round 1
// counted, and whose trace nonetheless supplies no slug.
```

### `go/internal/evalgate/quality.go:10` — above `type qualityGate struct{}`

```text
// qualityGate (Gate B) enforces that the selected slugs' eval predicates are
// behavioral, not tautological no-ops (cycle-204). It fires after the tdd
// phase, by which point the eval files exist, and reuses the working
// evalqualitycheck classifier (LevelPass/Warn/Halt). A definite tautology
// (LevelHalt) blocks at enforce; a weak predicate (LevelWarn) is advisory only
// (CLAUDE.md item-7's "block persistent WARN after a soak" is left as a TODO).
```

### `go/internal/evalgate/quality_wiring_amplified_test.go:1` — above `package evalgate`

```text
// quality_wiring_amplified_test.go — Test Amplifier (cycle 987).
//
// The TDD contract flags the exact blind spot this closes: gates_test.go
// exercises qualityGate{}.check() directly, never through
// NewReviewer(...).Review() — so a severed wire (qualityGate deleted from
// reviewer.go's composition slice) is invisible to the existing suite. The
// Builder's two binding tests (TestQualityGate_WiredIntoReviewer,
// TestNewReviewer_TautologyEvalBlocksAtEnforce) close that gap for the
// tautology-blocks-at-enforce case. These adversarial additions exercise the
// SAME wire from angles a stub or over-eager gate would fail even while
// passing the two canonical tests: stage-conditional gating (shadow must
// never block), the advisory/never-block contract for weak evals, a positive
// control (clean eval must not be blocked), and multi-eval aggregation.
// Written black-box against tdd-contract.md / build-report.md +
// gates_test.go's already-established qualityGate{} unit contract
// (tautology=block, weak/echo=advisory-never-block, behavioral=pass,
// missing-eval=fail-open-here) — reviewer.go/quality.go bodies and the
// Builder's new quality_wiring_test.go were deliberately not read.
```

### `go/internal/evalgate/quality_wiring_test.go:1` — above `package evalgate`

```text
// quality_wiring_test.go — cycle-987 gate-wiring binding tests for the quality
// gate's presence in NewReviewer's composed gate list (reviewer.go:39).
//
// gates_test.go exercises qualityGate{}.check() DIRECTLY but never through
// NewReviewer(...).Review(...), so deleting qualityGate{} from the composition
// slice passes 100% of the existing suite while silently re-admitting
// tautological evals. These two DEFAULT-SUITE tests close that blind spot,
// mirroring TestFloorBindingGate_WiredIntoReviewer (floorbinding_test.go).
```

### `go/internal/evalgate/slugs.go:1` — above `package evalgate`

```text
// Package evalgate implements the structural inter-phase gates that replace
// prose/trust contracts with verified checks, mounted at the orchestrator's
// existing per-phase DeliverableReviewer seam (core.WithReviewer):
//
//   - Gate A (materialization): after scout, every slug it SELECTED must have a
//     real .evolve/evals/<slug>.md file on disk (cycle-166: selected slugs with
//     no eval files → audit FAIL after build tokens were already spent).
//   - Gate B (predicate quality): after tdd, the selected slugs' eval predicates
//     must not be tautological no-ops (cycle-204), via evalqualitycheck.
//   - Gate C (floor binding): after tdd, EGPS floor predicates must bind only
//     floors triage COMMITTED this cycle (cycle-280), via triagecap.
//   - Gate D (flaky predicate shape): after tdd, the authored predicates are
//     linted for shapes that flake under fleet load (cycles 1173/1175/1178).
//     ADVISORY-ONLY — see flakyshape.go for why it can never block.
//
// The BLOCKING gates (A/B/C) gate ONLY on CERTAIN violations (a stat'd-missing
// file, a definite tautology, a proven deferred-floor binding) and fail OPEN on
// any ambiguity (parse failure, zero slugs, advisory WARN), so enforce-by-default
// never false-blocks a healthy cycle. Gate D never blocks at all.
```

### `go/internal/evalgate/slugs.go:95` — above `var slugLineRE = regexp.MustCompile('(?m)^[*\-]\s*\*\*Slug:\*\*\s*' + "'" + '?([a-z0-9][a-z0-9-]*)')`

```text
// slugLineRE matches a "- **Slug:** <kebab>" bullet (bullet char * or -). The
// slug may be wrapped in backticks: the persona template at
// agents/evolve-scout-reference.md declares the bare form, but real reports emit
// the backticked one about as often (12 of the 84 cycle-16xx reports carrying a
// "## Selected Tasks" section, measured 2026-09-15). Accepting both is what
// keeps SelectedTasksParseMiss a signal instead of a WARN on one cycle in seven.
//
// The widening is not blocking-neutral, and saying so cost this cycle an audit
// round: the first attempt counted "## Decision Trace" HEADINGS and reported the
// result as union equality, which is a different claim. Re-measured 2026-09-15
// by running this parser over the same 84-report corpus and diffing against the
// pre-widening pattern: the union SelectedSlugs returns differs on 6 of the 84
// reports — cycle-1605, -1610, -1634, -1664, -1665 and -1669, each going from
// empty to non-empty. All six do carry a "## Decision Trace" heading, but each
// states its selection as a "selected_tasks" string array, a shape
// decisionTraceSelected does not read, so the trace contributes nothing and the
// bullet is the only source of the slug.
//
// On 2 of those 6 the newly parsed slug has no eval file at either resolution
// evalFilePath checks, so Gate A blocks at StageEnforce where it previously
// fail-opened: cycle-1664 (settle-wait-stability-shortcircuit) and cycle-1669
// (verdict-tool-call-claudep). That is a deliberate capability increase, not a
// regression — catching a selected slug whose eval was never written is this
// gate's cycle-166 job, and on both of those cycles the eval really is absent.
// TestSlugLineWideningIsNotBlockingNeutral pins the effect by running it, so the
// next reader re-derives this paragraph instead of trusting it.
```

### `go/internal/evalgate/slugs.go:167` — above `func SelectedTasksParseMiss(report string) bool {`

```text
// SelectedTasksParseMiss reports whether a scout report's "## Selected Tasks"
// section is present and carries real content from which ZERO slugs parsed —
// format drift the parser could not read, as distinct from genuine convergence.
//
// SelectedSlugs returns nil for both shapes, so its callers cannot tell "nothing
// was claimed" (where fail-open is correct) from "something was claimed in a
// form we failed to read" (where fail-open silently drops the claim). That is
// cycle-1570: a section naming config-gate-default-policy-authority in prose
// rather than in a "- **Slug:**" bullet parsed to nil, Gate A's fail-open path
// let it through, and the missing eval surfaced three phases later as an audit
// H1. This function is that missing distinction.
//
// It is deliberately ADVISORY. Blocking every zero-slug report would false-block
// every converged cycle, which is precisely what this package's documented
// fail-open-on-ambiguity contract exists to prevent.
//
// True requires all three: the heading is present, the bounded body still holds
// non-whitespace content once HTML comments are stripped, and no slug bullet
// parses out of that body. Content after the next "## " heading belongs to
// another section and never counts, and a malformed "## Decision Trace" is out
// of scope by construction — this reads the Selected Tasks body only.
```
