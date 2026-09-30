# Comment history: `internal/router`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/router/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage. Each test
// names a previously-uncovered exported symbol and exercises it through its
// REAL producer/consumer (no `_ = pkg.X` padding): funcs are invoked and
// asserted; types are bound via route assembly / the strategy engine / a real
// digest projection and then read back. Conventions match the existing suite
// (testCfg, writeFile, Digest fixtures, the StaticPreset/LLMProposal brains).
```

### `go/internal/router/assemble.go:5` — above `func AssembleHandoffs(workspace string, completed []string) (phaseio.Handoffs, error) {`

```text
// AssembleHandoffs reads the completed phases' handoff artifacts (via Digest)
// and projects the resulting RoutingSignals into a dependency-free
// phaseio.Handoffs — the typed Upstream view a phase consumes (ADR-0050
// Phase 3.3). It is the single router→phaseio bridge: it lives in router
// (which already owns Digest + RoutingSignals and may import the phaseio leaf,
// router→phaseio→phasespec→config, no cycle) precisely so phaseio stays a pure
// leaf with no router import. Built on Digest so there is one on-disk-shape
// authority, never a second reader.
```

### `go/internal/router/condition.go:51` — above `func evalCondRule(sig RoutingSignals, r config.CondRule) bool {`

```text
// evalCondRule evaluates a conditional-mandatory rule (string value form): the
// head clause AND every clause in r.And (ADR-0099). A single-clause rule is
// byte-identical to the legacy evaluation.
```

### `go/internal/router/condition.go:72` — above `return 0, false, sig.DeliverableKind(), true`

```text
// Projected (triage > scout > "code") and ALWAYS present: the absent
// default "code" is the conservative side, so `deliverable_kind !=
// document` holds pre-handoff and the tdd pin is released only by a
// digested document signal (ADR-0099).
```

### `go/internal/router/condition_test.go:106` — above `func TestEvalCondition_AbsentFieldIsAlwaysFalse(t *testing.T) {`

```text
// TestEvalCondition_AbsentFieldIsAlwaysFalse encodes the cycle-238 defect D2
// (missing-signal fail-open): an insert_when condition on a generic field that
// was NEVER EMITTED must evaluate false for EVERY operator. At the defective
// baseline, `ne` on an absent field returned true ("" != value) and `eq ""`
// returned true ("" == ""), so catalog phases with `goal_type != <other-goal>`
// triggers fired when scout.goal_type was simply not emitted.
```

### `go/internal/router/condition_test.go:120` — above `{"scout.goal_type", "ne", "growth"},`

```text
// the cycle-238 fail-open shape
```

### `go/internal/router/condition_test.go:177` — above `func TestTriggerFires_AbsentFieldFailsClosed(t *testing.T) {`

```text
// TestTriggerFires_AbsentFieldFailsClosed proves D2 at the trigger level — the
// exact cycle-238 mechanism end-to-end through triggerFires:
//  1. insert_when `ne` on an absent signal must NOT fire the phase, and
//  2. skip_when `ne` on an absent signal must NOT suppress an otherwise-firing
//     insert (fail-closed means absent CONDITIONS are false, in both polarities).
```

### `go/internal/router/deliverable_kind_test.go:3` — above `import (`

```text
// deliverable_kind_test.go — ADR-0099 (deliverable kinds: solution cycles on
// the same spine). RED contract:
//
//   - The kernel derives TWO new objective signals from the report headers the
//     phases already write (handoff JSON has been extinct since ~cycle 215, so
//     the header line is the trusted path, exactly as triage's
//     `cycle_size_estimate:` reaches RoutingSignals today):
//       scout-report.md  → `goal_type: <goal_recipes key>` + `deliverable_kind: code|document`
//       triage-report.md → `deliverable_kind: code|document` (authoritative, like cycle_size)
//   - RoutingSignals.DeliverableKind() projects triage > scout > "code". The
//     absent-default is "code" — the CONSERVATIVE side: a conditional rule
//     `deliverable_kind != document` evaluates TRUE pre-handoff, so the tdd pin
//     holds at plan time and is released only by a digested document signal
//     (post-scout RePlan). An unrecognised kind word is fail-safe: it never
//     releases anything.
//   - `scout.goal_type` finally has a producer, so the 15 shipped domain
//     phases' `insert_when` triggers (dormant since 2026-06-06) can fire.
```

### `go/internal/router/deliverable_kind_test.go:199` — above `func TestRoutingSignals_DeclaredDeliverableKind(t *testing.T) {`

```text
// TestRoutingSignals_DeclaredDeliverableKind — ADR-0099 slice 3: the kernel
// exposes whether a kind was DECLARED (triage > scout) separately from its
// conservative "code" default, so the dispatch-time projection can substitute
// the project default only when no report spoke, while the integrity floor
// keeps reading declarations alone.
```

### `go/internal/router/digest.go:72` — above `for _, phase := range completed {`

```text
// ADR-0039 §7: lift each completed phase's structured failure context
// (report-sentinel v2 failure block) onto the generic plane — this is
// what lets failure-phase insertion be DATA-driven via insert_when.
```

### `go/internal/router/digest.go:133` — above `func unwrapPayload(raw []byte) []byte {`

```text
// unwrapPayload returns the inner `payload` object bytes of the canonical
// ADR-0050 Phase-3 envelope (schema_version 2: a wrapper carrying the exact
// per-phase payload bytes plus promoted top-level verdict/signals/failure), or
// the input unchanged when there is no payload wrapper — the Postel-compatible
// flat fallback. This keeps Digest reading byte-identically whether a handoff is
// written flat (legacy/today) or payload-wrapped (the unified envelope), which
// is the golden-equivalence invariant the shadow stage relies on.
//
// AUTHORITY CONTRACT: the inner payload is the single source of truth. Digest
// (and foldGeneric) read the UNWRAPPED payload, so the wrapper's promoted
// top-level signals/verdict/failure are defined to be a COPY of the payload's,
// never an independent source — a wrapper that carried signals absent from its
// payload would have them ignored. The envelope writer (Phase 3.4+) must uphold
// this; TestDigest_PayloadWrapped_FoldsInnerSignals pins the read side.
```

### `go/internal/router/digest.go:226` — above `func buildFromGitFallback(workspace string, degraded *[]string) BuildSignals {`

```text
// buildFromGitFallback derives BuildSignals from git when neither
// handoff-build.json nor handoff-builder.json is present on disk — both have
// been extinct since ~cycle 215 (warnship_apicover_ci_gap, 3rd recurrence),
// which otherwise leaves sig.Build silently zero-value on every real cycle.
// It reuses changedpkgs.FromGitChecked — the same helper
// internal/phases/audit.changedPackagesForAudit already uses — rather than
// re-implementing git-diff logic. A git-underivable tree (no repo, git
// failure) degrades LOUDLY via DigestDegraded instead of silently returning
// Present:false with no trace, mirroring the read-miss vs genuine-gap
// distinction readFirstTracked already applies to handoff read errors.
```

### `go/internal/router/digest.go:254` — above `func scoutFromReportFallback(workspace string, degraded *[]string) ScoutSignals {`

```text
// scoutFromReportFallback derives ScoutSignals from the artifact scout
// actually writes every cycle — scout-report.md — when handoff-scout.json is
// absent (handoffs have been extinct since ~cycle 215; buildFromGitFallback
// closed the same gap for build). Presence is the only signal the spine floor
// gates on, so the richer handoff fields stay zero — the router's digest
// degrades in richness, never in floor truth. A missing report is a CLEAN
// absence (Present:false, no degrade entry — the enforce gate's fail-closed
// signal); a report that exists but cannot be read is a read-miss and degrades
// LOUDLY (R5), matching readFirstTracked's distinction.
```

### `go/internal/router/digest.go:275` — above `const (`

```text
// Report header keys the kernel READS (ADR-0099) and the scout/triage personas
// WRITE — exported so the persona templates are pinned to these exact words
// (TestPersonaTemplates_CarryTheHeaderLines) instead of carrying a second
// literal that could drift.
```

### `go/internal/router/digest.go:305` — above `func triageFromReportFallback(workspace string, degraded *[]string) TriageSignals {`

```text
// triageFromReportFallback derives TriageSignals from triage-report.md — the
// artifact triage actually writes every cycle — when handoff-triage.json is
// absent (handoffs extinct since ~cycle 215; same gap scoutFromReportFallback
// closed for scout). Richer than scout's: it extracts the report header's
// `cycle_size_estimate: <size>` line so RoutingSignals.CycleSize() carries a
// real value on the live path (ADR-0076 slice A's budget signal). No size
// vocabulary validation here — the multiplier lookup treats unknown sizes as
// 1.0, so tolerance is safe and single-sourced at the consumer. Absence and
// read-miss semantics mirror scoutFromReportFallback exactly.
```

### `go/internal/router/digest.go:358` — above `*degraded = append(*degraded, "audit: acs-verdict.json has no verdict field (schema drift?) — degraded, not clean")`

```text
// Parses but carries no verdict — a legacy/schema-drifted artifact
// (the 2c0559a5 e2e red: Present:true + Verdict:"" is an UNSATISFIABLE
// audit anchor that read as a clean absence and hard-blocked at
// enforce). Schema drift is a DEGRADED read, never a clean gap.
```

### `go/internal/router/digest.go:401` — above `if strings.HasPrefix(k, "item") && hasDigitAfterPrefix(k, "item") {`

```text
// itemN_* blocks measure scope breadth (cycle-56: item1_..item6_).
```

### `go/internal/router/digest_git_fallback_test.go:3` — above `import (`

```text
// digest_git_fallback_test.go — RED contract for cycle-589 Task
// changedpkgs-acssuite-router-git-fallback (scout-report.md Task 1, AC2;
// inbox builder-handoff-extinct-deterministic-changedpkgs weight 0.96, 3rd
// recurrence of warnship_apicover_ci_gap).
//
// TODAY: Digest's "build" branch (digest.go ~L43) reads ONLY
// handoff-build.json / handoff-builder.json via readFirstTracked. Both have
// been extinct since ~cycle 215, so on every real cycle sig.Build stays
// Present:false / FilesTouched:0 with an EMPTY DigestDegraded — a silent gap
// indistinguishable from "the build phase did nothing."
//
// FIX CONTRACT (undefined until Builder adds it, so this file's assertions
// fail today — that failure IS the RED evidence):
//
//   - When done["build"] is true but neither handoff file is present, Digest
//     falls back to a git-derived signal — reusing
//     changedpkgs.FromGitChecked(projectRoot, "HEAD") (same shared helper
//     internal/phases/audit.changedPackagesForAudit already uses; no
//     re-implemented git-diff logic), deriving projectRoot from workspace
//     via the standard <projectRoot>/.evolve/runs/cycle-<N> layout
//     (core.RunWorkspacePath's inverse).
//   - A git-derivable tree (even with a fallback, even if the handoff never
//     existed) yields sig.Build.Present == true with FilesTouched reflecting
//     the actually-changed files — never silently staying Present:false.
//   - A git-UNDERIVABLE tree (no git repo, git failure) must degrade LOUDLY:
//     sig.DigestDegraded gains an entry (mentioning "build") rather than the
//     current silent Present:false / empty-DigestDegraded combination — the
//     read-miss vs genuine-gap distinction (R5) the digest already applies to
//     handoff read errors must also cover an underivable git fallback.
```

### `go/internal/router/digest_git_fallback_test.go:73` — above `func TestDigest_Build_FallsBackToGitWhenHandoffAbsent(t *testing.T) {`

```text
// TestDigest_Build_FallsBackToGitWhenHandoffAbsent — AC2 (positive case): a
// git-derivable worktree with NO handoff-build.json/handoff-builder.json must
// still populate sig.Build (Present:true, FilesTouched > 0) via the git
// fallback, instead of silently staying Present:false — the same silent gap
// that let cycle-587 ship internal/ciwatch without the router ever seeing it
// as a "files touched" signal.
```

### `go/internal/router/digest_payload_test.go:8` — above `func TestDigest_PayloadWrapped_EquivalentToFlat(t *testing.T) {`

```text
// TestDigest_PayloadWrapped_EquivalentToFlat is the golden-equivalence anchor
// for ADR-0050 Phase 3.3: Digest must read the new payload-wrapped handoff
// envelope and yield byte-identical RoutingSignals to the legacy flat envelope
// (Postel-compatible). Until digest.go unwraps `payload`, the wrapped workspace
// extracts nothing and this fails RED.
```

### `go/internal/router/digest_report_fallback_test.go:3` — above `import (`

```text
// digest_report_fallback_test.go — RED contract for the L1 spine-floor fix
// (static/dynamic boundary review 2026-07-16, spine-floor-enforce-flip).
//
// TODAY: Digest's "scout" and "audit" branches read ONLY handoff-*.json via
// readFirstTracked. Handoff files have been extinct since ~cycle 215 (see
// buildFromGitFallback, which closed the same gap for build) — so on EVERY
// real cycle sig.Scout/sig.Audit stay Present:false, the spine floor
// (SpineSatisfiedUpTo) reports unsatisfied for next=triage/build/audit/ship,
// and the WARN clamp fires on healthy PASSing cycles (1914 clamped routing
// decisions across the surviving run history; 7 in the 2026-07-16 run alone).
// That WARN storm is the ONLY thing blocking the PhaseRecovery shadow→enforce
// flip — the floor cannot be armed while it cries wolf on every cycle.
//
// FIX CONTRACT (fails today — that failure is the RED evidence):
//   - scout: handoff absent → fall back to the artifact scout actually writes
//     every cycle: a non-empty scout-report.md ⇒ Present:true (signals beyond
//     presence stay zero — presence is what the floor gates on). No report at
//     all stays a CLEAN absence (Present:false, DigestDegraded empty).
//   - audit: handoff absent → fall back to acs-verdict.json — the
//     DETERMINISTIC, Go-generated verdict artifact (generateACSVerdict), whose
//     top-level verdict/red_count keys extractAudit already reads. A FAIL
//     verdict is carried through honestly (the audit anchor then correctly
//     refuses ship). A corrupt acs-verdict.json degrades LOUDLY via
//     DigestDegraded (fail-open at enforce), never silently Present:false.
```

### `go/internal/router/digest_report_fallback_test.go:131` — above `func TestDigest_Audit_VerdictlessACSVerdictDegradesLoudly(t *testing.T) {`

```text
// TestDigest_Audit_VerdictlessACSVerdictDegradesLoudly — the e2e-tier red at
// 2c0559a5 (cycles blocked at ship on CI): an acs-verdict.json that PARSES but
// carries no top-level verdict field (a legacy/schema-drifted artifact — the
// e2e fake emitted exactly this) yielded Present:true + Verdict:"" — an
// UNSATISFIABLE audit anchor (needs PASS|WARN) that read as a CLEAN absence,
// so the armed spine floor hard-blocked. A schema-drifted artifact is a
// DEGRADED read, not a clean gap: mark DigestDegraded (fail-open at enforce),
// never a silent block.
```

### `go/internal/router/digest_test.go:19` — above `const buildHandoff = '{`

```text
// buildHandoff mirrors the real cycle-55 handoff-build.json shape.
```

### `go/internal/router/digest_test.go:186` — above `func TestDigest_LiftsFailureSentinelSignals(t *testing.T) {`

```text
// --- ADR-0039 §7 item 3: failure-sentinel signal lifting ---
```

### `go/internal/router/floor.go:13` — above `func ClampPlanToFloor(in RouteInput, plan *PhasePlan) (*PhasePlan, []Clamp) {`

```text
// floor.go implements the ADR-0024 §1 conditional integrity floor: the SINGLE
// causal invariant that replaces the fixed mandatory-spine never-skip list when
// an advisor drives phase selection (Stage>=Advisory). It is a PURE plan-level
// prefilter — the caller (orchestrator) applies it only when routing is at
// Advisory or above; below that the legacy static path runs unchanged.
```

### `go/internal/router/floor.go:19` — above `func ClampPlanToFloor(in RouteInput, plan *PhasePlan) (*PhasePlan, []Clamp) {`

```text
// ClampPlanToFloor enforces the conditional integrity floor on an advisory
// whole-cycle plan. The floor is two causal implications:
//
//	reach(ship) ⇒ build ∧ audit ∧ (tdd, unless the cycle is trivial)
//	run(build)  ⇒ audit ∧ ship   (operator policy 2026-06-11)
//
// If the plan runs ship, the clamp forces build + audit on (and tdd unless the
// cycle is trivial per the configured TDD-pin), recording one Clamp per forced
// phase. If the plan BUILDS, review and ship are forced — built work may not
// strand unreviewed or unshipped. Only a no-build, no-ship plan is left fully
// unconstrained — such a cycle may legitimately end after scout
// (investigation/convergence). The clamp can only
// COMPLETE the set, never weaken it (sequencing across the set is the walk's job,
// not the floor's). Phase names must be canonical lowercase — the caller
// normalizes the advisor's parsed output before clamping.
//
// This is a plan-level PREFILTER, not the whole safety story: it forces audit to
// RUN, but the "audit must PASS bound to the built tree" guarantee remains with
// the ship phase's audit-binding (tree-SHA match + EGPS red_count==0) and the
// artifact-backed SpineSatisfiedUpTo gate. Defense in depth — never the sole gate.
//
// PURE: returns a NEW plan (input unmutated) plus the clamps applied.
//
// This is the back-compat entry point: it enforces the SAFE STRUCTURAL DEFAULT
// floor (DefaultShipFloor). Callers that honor a user-configured floor
// (.evolve/policy.json:ship_floor) call ClampPlanToFloorWith with the resolved
// set instead. Keeping this wrapper byte-identical to the historical behavior is
// what lets the existing floor_test.go suite stand as the default-preserving proof.
```

### `go/internal/router/floor.go:129` — above `if planRuns(out, "build") {`

```text
// Converse implication (operator policy 2026-06-11): a cycle that BUILDS
// must schedule review and ship — built work may not strand unreviewed or
// unshipped (the cycle-283 class: a completed build discarded with
// audit/ship unreached). Forcing ship here makes the building plan
// ship-bound, so the ship floor below then completes the set. No-build
// investigation cycles are untouched (the antecedent is false).
```

### `go/internal/router/floor.go:169` — above `func dropUnknownPhases(in RouteInput, plan *PhasePlan) ([]PhasePlanEntry, []Clamp) {`

```text
// dropUnknownPhases returns a NEW entry slice with every entry whose phase is
// not in knownPhaseSet removed, plus one Clamp per removal so the drop is never
// silent (floor.go's "no silent disposition" rule).
//
// Why here and not in ValidatePlan: ValidatePlan already flags these as
// "unknown-phase" but is documented PURE and REPORT-ONLY, so nothing removed
// them — an advisor-hallucinated phase reached dispatch and crashed the cycle
// with "profile not found" (cycles 1151, 1152). The clamp is the sole plan
// disposer, so enforcement belongs here. knownPhaseSet is REUSED rather than
// re-derived, keeping the drop mint-aware and in lockstep with the walk.
//
// Both run:true and run:false entries are dropped: a skipped unknown is still
// garbage the walk and the telemetry must not see. Removal, not Run=false —
// dispatch keys off the entry's presence.
```

### `go/internal/router/floor.go:237` — above `func tddPinned(in RouteInput) bool {`

```text
// tddPinned reports whether tdd is mandatory this cycle. It reuses the kernel's
// existing conditional-mandatory rule (EVOLVE_CONDITIONAL_MANDATORY; default
// config.DefaultTddRuleExpr — trivial OR a document deliverable releases, ADR-0099)
// so the floor's exemptions stay consistent with shouldRun's TDD-pin. Absent rule ⇒ pinned (the safer, more-mandatory side).
```

### `go/internal/router/floor_activation_scenarios_test.go:3` — above `import (`

```text
// Pure-kernel scenario catalog for the ADR-0024 §1 conditional integrity floor
// ACTIVATION (PR-5 live-wiring). Proves the clamped whole-cycle plan drives
// run/skip at Stage>=Advisory, that the configurable mandatory set is the
// never-skip floor, and that the non-configurable integrity floor (ship⇒build∧
// audit∧tdd) backstops a plan that ships without the chain. Built entirely from
// the configurable routingtest framework. See internal/routingtest.
```

### `go/internal/router/floor_build_requires_review_test.go:3` — above `import "testing"`

```text
// floor_build_requires_review_test.go — RED contract for the operator policy
// (2026-06-11): "any form of review phase must be selected for the pipeline;
// if the audit/verdict is passed, the code/changes must ship."
//
// Plan-level half of that guarantee, as the CONVERSE implication of the
// existing ship floor:
//
//	run(build) ⇒ run(audit) ∧ run(ship)
//
// A cycle that BUILDS must schedule review and ship — built work may not
// strand unreviewed or unshipped (the cycle-283 class: completed build
// discarded with audit/ship unreached). No-build investigation cycles stay
// unconstrained (the antecedent is false), preserving the documented
// scout-only legitimacy. The runtime half (audit must PASS bound to the built
// tree before ship commits) remains with audit-binding + EGPS — this clamp is
// the plan-level prefilter, defense in depth, never the sole gate.
```

### `go/internal/router/floor_intent_test.go:3` — above `import "testing"`

```text
// Cycle-238 advisory-soak defect D4: with EVOLVE_REQUIRE_INTENT=1 the static
// state machine starts at intent (NextFromStart), but the advisory plan —
// computed without the requirement — omits intent, so enforceNext's
// plan-honoring override silently drops the operator's required intent gate.
// The fix: the integrity-floor clamp honors RouteInput.IntentRequired and
// forces an intent Run:true entry (clamp rule "require-intent"), mirroring how
// the ship-chain phases are forced. Unlike the ship floor, the intent
// requirement is NOT gated on planRuns(ship): EVOLVE_REQUIRE_INTENT=1 demands
// the intent gate on every cycle shape, exactly as NextFromStart does on the
// static path — a no-ship investigation cycle with the flag set still starts
// at intent.
```

### `go/internal/router/floor_test.go:39` — above `func TestClampPlanToFloor_NoShipIsUnconstrained(t *testing.T) {`

```text
// TestClampPlanToFloor_NoShipIsUnconstrained proves a no-ship plan is left
// untouched — a scout-only investigation cycle is legitimate (ADR-0024 §1).
```

### `go/internal/router/floor_test.go:79` — above `if len(clamps) != 1 || clamps[0].Rule != "build-requires-audit" {`

```text
// Since the 2026-06-11 review-floor policy, the converse implication
// (build-requires-audit) fires first and claims the single clamp.
```

### `go/internal/router/floor_test.go:139` — above `func TestClampPlanToFloor_ShipFalseWithBuildOverridden(t *testing.T) {`

```text
// TestClampPlanToFloor_ShipFalseWithBuildOverridden: POLICY CHANGE
// (2026-06-11, repeals the former ShipFalseNotReached expectation): an
// explicit ship veto in a BUILDING plan is overridden — built work may not
// strand unshipped. Only no-build plans may decline ship.
```

### `go/internal/router/floor_test.go:155` — above `func TestClampPlanToFloor_ShipAbsentWithBuildAppended(t *testing.T) {`

```text
// TestClampPlanToFloor_ShipAbsentWithBuildAppended: POLICY CHANGE
// (2026-06-11, repeals the former ShipAbsentNotReached expectation): ship
// missing entirely from a BUILDING plan is appended run=true — same rationale
// as the explicit-veto override.
```

### `go/internal/router/floor_test.go:243` — above `func TestClampPlanToFloorWith_DropsUnknownPhaseEntry(t *testing.T) {`

```text
// TestClampPlanToFloorWith_DropsUnknownPhaseEntry is the in-package regression
// for the cycle-1151/1152 incident: the advisor hallucinated "gate-wiring-proof"
// out of policy prose, ValidatePlan flagged it unknown-phase (report-only), and
// the entry survived into dispatch ("profile not found"). The clamp must now
// remove it — recording the removal under DropUnknownPhaseRule — while leaving
// the known phases and the integrity floor intact.
```

### `go/internal/router/floor_unavailable_test.go:9` — above `func TestClampPlanToFloorWith_DropsUnavailablePhases(t *testing.T) {`

```text
// TestClampPlanToFloorWith_DropsUnavailablePhases — 2026-09-09 token-waste
// root cause #2: an advisor plan that schedules a phase whose persona doc is
// absent is clamped at the floor, so the dispatch/skip/retrospective spend
// never happens. The drop is recorded under its own rule token, distinct from
// the unknown-phase drop, so forensics can tell "not in the catalog" from "in
// the catalog, persona missing".
```

### `go/internal/router/header_persona_pin_test.go:10` — above `func TestPersonaTemplates_CarryTheHeaderLines(t *testing.T) {`

```text
// TestPersonaTemplates_CarryTheHeaderLines — ADR-0099 slice 3: the words the
// kernel READS (HeaderGoalType, HeaderDeliverableKind, HeaderCycleSize) are the
// words the scout and triage personas WRITE. Two pins per header: the
// DISPATCHED persona names it as a directive (agents/evolve-scout.md's
// operational body — the reference file is stripped from dispatched prompts),
// and the output template carries it as a line-start header.
```

### `go/internal/router/hybrid_cadence_test.go:3` — above `import (`

```text
// Unit proof of the ADR-0024 §2 hybrid cadence: LLMProposal.Decide invokes the
// Proposer on EVERY transition when no upfront plan drives (legacy/Shadow), but
// ONLY at branch transitions (post-build, post-audit) once a clamped plan is
// threaded in — removing the per-transition double-spend at Stage>=Advisory.
```

### `go/internal/router/mintspec_metadata_test.go:8` — above `func TestMintSpec_CarriesSelectMetadata(t *testing.T) {`

```text
// TestMintSpec_CarriesSelectMetadata pins the advisor-facing wire contract
// (cycle-1275): MintSpec must decode description/when_to_use under the SAME
// JSON keys phasespec.PhaseSpec already uses, so the minter can thread the
// advisor's SELECT metadata straight through without a vocabulary translation.
```

### `go/internal/router/mintspec_metadata_test.go:27` — above `func TestMintSpec_MetadataOmitEmpty(t *testing.T) {`

```text
// TestMintSpec_MetadataOmitEmpty is the negative case: a MintSpec with unset
// metadata must marshal byte-identically to the pre-cycle-1275 wire form, so
// today's advisor output and every recorded plan artifact round-trip unchanged.
```

### `go/internal/router/mismatch_test.go:9` — above `func TestPlanMismatch_TriggersOnlyOnMaterialDivergence(t *testing.T) {`

```text
// TestPlanMismatch_TriggersOnlyOnMaterialDivergence pins WS2-S4 (ADR-0052) with a
// boundary table: "tester" inserts when scout.item_count >= 5. A mismatch exists
// only when the trigger FIRES on the measured signals AND the plan omits that
// phase. N=4 (below threshold) is not a mismatch; N=5 (fires) with tester
// unscheduled is; N=5 with tester already scheduled is not (need covered, no churn).
```

### `go/internal/router/model_routing_clamp.go:10` — above `var universalTierFloor = &profiles.ModelTierEnvelope{Min: "balanced", Max: "top"}`

```text
// universalTierFloor is the compiled-default model-tier envelope applied when a
// profile declares no explicit model_tier_envelope (cycle-480,
// universal-envelope-floor). 72/91 profiles omit an envelope; without a default
// those phases skipped the clamp-up gate entirely and a below-floor advisor tier
// proposal fell through to policy.ValidatePin as a mere PREFERENCE (B2), never
// clamped. Single-sourcing the floor HERE — rather than editing every profile
// JSON — guarantees a below-floor tier is clamped UP to "balanced" for EVERY
// phase, while any profile that DOES declare an envelope still wins (its explicit
// Min is used verbatim). Min feeds the clamp-up gate; Max feeds the clamp-DOWN
// gate (L5, 2026-07-16): "top" is the HIGHEST rank in policy.TierRank's
// fast<balanced<deep<top ladder (a live advisor-proposable frontier tier —
// sanitizeAdvisorTier keeps it legal), so envelope-less profiles accept every
// tier unchanged — only a profile declaring a lower explicit Max (e.g. a
// memo-class balanced ceiling) gets a real ceiling. Pre-ceiling this field was
// documentation-only and said "deep"; activating the ceiling with "deep" would
// have silently foreclosed "top" for the 72/91 envelope-less profiles
// (go-reviewer HIGH, 2026-07-16).
```

### `go/internal/router/model_routing_clamp.go:44` — above `func ClampPlanModelRouting(plan *PhasePlan, profileFor func(phase string) *profiles.Profile, catalogLookup func(cli, tie…`

```text
// ClampPlanModelRouting is the cycle-436 MR2 guardrail: it re-validates every
// plan entry's advisor-proposed {CLI,Tier} against the phase's OWN profile
// guardrails (allowed_clis + model_tier_envelope, via the EXISTING
// policy.ValidatePin — reused, not forked) and the live model catalog
// (modelcatalog.Catalog.Lookup), clamping any out-of-bounds or
// catalog-unresolvable proposal back to the safe static default ({cli:"",
// tier:""}, which the resolver already treats as "use the profile's pinned
// default") rather than ever letting an illegal or unresolvable pair reach
// dispatch ("model proposes, kernel disposes"). profileFor resolves a
// phase's profile lazily (nil ⇒ nothing to validate ⇒ honored, matching
// ValidatePin's own nil-profile contract); a NIL profile is distinct from a
// profile with an empty AllowedCLIs (B2: no restriction configured is a
// PREFERENCE, not a violation — ValidatePin already encodes this). An entry
// that proposes neither CLI nor Tier is left untouched (nothing to clamp).
// catalogLookup resolves (cli,tier)→(model,ok); it is INJECTED (dependency
// inversion) so router stays a leaf and never imports modelcatalog — the
// caller passes modelcatalog.Catalog.Lookup. A nil catalogLookup skips the
// catalog-resolvability gate (guardrail validation still applies).
// PURE: returns a NEW plan (input unmutated) plus the clamps applied, so it
// composes with ClampPlanToFloorWith exactly like every other router clamp.
```

### `go/internal/router/model_routing_clamp.go:81` — above `if prof != nil && e.Tier != "" {`

```text
// Operator low-model floor (cycle-463 T4; universalized cycle-480): a tier
// proposal BELOW the phase's envelope minimum clamps UP to the floor rather
// than emptying the whole proposal — the CLI is left untouched since only
// the tier violated a bound. When the profile declares no envelope, the
// compiled universalTierFloor is substituted so the floor is UNIVERSAL
// across every phase (72/91 profiles omit an envelope). TierRank returns 0
// for an unclassifiable string, so this only fires when both ranks are real.
```

### `go/internal/router/model_routing_clamp.go:99` — above `if maxRank := policy.TierRank(env.Max); tierRank > 0 && maxRank > 0 && tierRank > maxRank {`

```text
// Ceiling (L5, 2026-07-16): a tier ABOVE the envelope maximum clamps
// DOWN to the ceiling — an over-provisioned proposal is cost/quota
// drift (a memo-class phase routed to deep), the same shape the floor
// closes from below. The universal envelope's Max is the top tier, so
// this only ever fires against an explicitly-declared lower Max.
```

### `go/internal/router/model_routing_clamp_ceiling_test.go:3` — above `import (`

```text
// model_routing_clamp_ceiling_test.go — L5 (static/dynamic boundary review
// 2026-07-16): the routing clamp enforced only a FLOOR (clamp-up to the
// envelope Min); a tier proposal ABOVE the envelope Max sailed through — an
// advisor could route a memo-class phase (max balanced) onto deep, a
// cost/quota leak (the same pressure class behind the quota storms). These
// tests pin the ceiling: above-Max clamps DOWN to Max, and the compiled
// universal envelope (Max "top" — the HIGHEST TierRank, above deep) keeps
// envelope-less profiles unaffected: no behavior change for the 72/91
// profiles without an explicit envelope, including advisor-proposed "top".
```

### `go/internal/router/model_routing_clamp_ceiling_test.go:66` — above `func TestClampPlanModelRouting_EnvelopelessTopStaysLegal(t *testing.T) {`

```text
// TestClampPlanModelRouting_EnvelopelessTopStaysLegal — go-reviewer HIGH
// regression pin (2026-07-16): "top" is TierRank's HIGHEST tier (above deep)
// and a live advisor-proposable value (sanitizeAdvisorTier keeps it). The
// universal envelope's Max MUST be "top", or activating the ceiling silently
// forecloses the frontier tier for every envelope-less profile (72/91) — the
// exact false-comfort this test would have caught the first time.
```

### `go/internal/router/model_routing_clamp_envelope_floor_test.go:10` — above `func TestClampPlanModelRouting_NilEnvelopeFloorClampsUp(t *testing.T) {`

```text
// Cycle-480 Task 1 (universal-envelope-floor) RED tests.
//
// Root cause (scout Key Finding 3): the operator low-model floor at
// model_routing_clamp.go:52 gates the clamp-up entirely on
// `prof.ModelTierEnvelope != nil`. 72/91 profiles declare NO envelope, so a
// `tier:fast` advisor proposal against a nil-envelope profile falls through to
// policy.ValidatePin, which (by design B2) treats "no envelope configured" as a
// PREFERENCE, not a violation — and is therefore NEVER clamped up to the
// balanced floor. The fix substitutes a compiled-default envelope
// {min:balanced, max:deep} at the clamp site when ModelTierEnvelope == nil, so
// every phase — regardless of whether its profile happens to declare an
// envelope — gets the same universal floor.
//
// These tests exercise the SUT (ClampPlanModelRouting) directly. Builder must
// NOT modify this file — implement the production fix in
// model_routing_clamp.go until they are GREEN.
```

### `go/internal/router/recipes.go:9` — above `func RenderRecipeProjection(recipes map[string][]string) string {`

```text
// RenderRecipeProjection renders the goal-type recipe table body from the recipe
// SSOT (config.RoutingConfig.GoalRecipes, passed as the bare map to keep router
// free of a config import). It is the generate side of the single-source recipe
// projection (ADR-0052 WS5, P10): one markdown row per goal type, sorted by goal
// type for determinism, recipe tokens joined with " → ". The persona's
// "Goal-Type Recipes" table is locked against this output (WS5-S2), and the
// RecipeVerifier reads the same source — ending the three-source recipe drift
// (the persona prose, the registry triggers, and per-phase metadata used to drift
// independently). A nil/empty map renders to the empty string (a clean no-op).
```

### `go/internal/router/recipes_drift_test.go:19` — above `func TestRouterPersonaRecipeTable_NoDrift(t *testing.T) {`

```text
// TestRouterPersonaRecipeTable_NoDrift locks the persona's "## Goal-Type Recipes"
// table body (agents/evolve-router.md, between the GENERATED markers) to the
// single source of truth — config.goal_recipes in phase-registry.json projected
// through RenderRecipeProjection (ADR-0052 WS5-S2). If a recipe is edited in the
// persona by hand, or in the registry without regenerating the table, this fails.
```

### `go/internal/router/recipes_test.go:8` — above `func TestRenderRecipeProjection_FromConfig(t *testing.T) {`

```text
// WS5-S1 (ADR-0052): RenderRecipeProjection is the generate side of the
// single-source goal-type recipe projection (P10). It renders the recipe SSOT
// (cfg.GoalRecipes) into the persona's "Goal-Type Recipes" table body — one row
// per goal type, sorted for determinism, tokens joined with " → ". The persona
// table is drift-locked against this output (WS5-S2) and the RecipeVerifier
// reads the same source, killing the three-source recipe drift (gap #3).
```

### `go/internal/router/recon.go:10` — above `type ReconDigest struct {`

```text
// ReconDigest is the deterministic pre-plan recon (ADR-0052 WS2-S0b): measured
// repo facts fed into the INITIAL whole-cycle Plan prompt so upfront phase
// selection is grounded in evidence, not goal-text inference alone (closing
// capability gap #1 for the initial plan, deterministically — no LLM, no
// subagent, per Core Rule 5). Every field is deterministic given repo state; the
// core gatherer FAILS OPEN — a git/fs error omits that fact and never errors —
// so a degraded environment silently narrows the digest rather than breaking
// planning. The floor still clamps whatever plan results, so more signal yields
// better plans, never unsafe ones.
```

### `go/internal/router/recon_test.go:10` — above `func TestPrePlanReconDigest_Deterministic(t *testing.T) {`

```text
// TestPrePlanReconDigest_Deterministic pins WS2-S0b (ADR-0052): the pre-plan
// recon digest is a deterministic, sorted, fail-open function of its inputs —
// same inputs ⇒ byte-identical digest, slice fields sorted+deduped, and a nil
// changedFiles slice (an upstream git error) simply omits the file-derived facts
// rather than erroring. This is the property that lets the recon feed measured
// repo facts into the INITIAL plan without becoming a flaky or fatal dependency.
```

### `go/internal/router/recovery.go:31` — above `var shipLocalCodes = map[string]bool{`

```text
// shipLocalCodes are ship-LOCAL preconditions a re-audit cannot re-establish:
// re-running audit re-verifies the same code while the blocking condition
// (merge divergence, prefix-gate scope, detached HEAD, unresolvable worktree)
// lives entirely on the ship side — the cycle-230 audit↔ship loop (3 PASS
// audits, 0 ships). Ship's in-Run repair ladder (ADR-0039 §8) already
// attempted the typed repair before this error surfaced, so the residue goes
// to the LLM debugger phase for triage, never back to audit.
```

### `go/internal/router/recovery.go:51` — above `name: "fleet-rebase-conflict-debugger",`

```text
// ADR-0049 G13a: a fleet rebase CONFLICT is genuinely overlapping work the
// advisor's disjoint-file partition should have separated. It carries the
// integrity class, but unlike a tamper/drift breach it is RECOVERABLE by
// triage — route to the LLM debugger (recommend sequential retry / partition
// split), NOT a blind block. Ordered FIRST so this specific code wins over
// the generic integrity-block below (the one integrity code that recovers).
```

### `go/internal/router/recovery.go:76` — above `name: "control-plane-rebuild",`

```text
// F37: a cycle diff touching the protected control plane (ADR-0064,
// ship's verifyNoControlPlaneEdits) is the BUILD's to reshape. A
// re-audit re-verifies the same diff — the cycle-230 audit↔ship loop —
// and the debugger cannot change what a cycle may write; the build
// handoff floor names the path on re-entry. Ordered BEFORE
// precondition-reaudit (it carries the precondition class).
```

### `go/internal/router/recovery.go:115` — above `name: "fleet-rebase-reaudit",`

```text
// ADR-0049 S5b: a fleet-mode ff-merge divergence (a peer cycle moved main
// mid-pipeline) is recovered by rebasing the cycle branch onto the new
// main and re-running AUDIT on the merged tree — the test-the-merged-tree
// / merge-queue pattern, which produces a FRESH audit binding for the
// rebased tree (re-pinning in place would be self-referential). NOT a
// retry-ship (it would just diverge again) and NOT the debugger. The
// rebase action itself runs in the orchestrator's recoverFromShipError
// before this re-audit. Ordered BEFORE transient-retry-ship because this
// transient code needs re-audit, not a blind ship retry.
```

### `go/internal/router/recovery_manifestgate_amplify_test.go:5` — above `func TestRecover_ManifestGate_RoutesToDebugger(t *testing.T) {`

```text
// TestRecover_ManifestGate_RoutesToDebugger amplifies cycle-1064's
// manifest-gate-policy-wiring change: recovery.go's shipLocalCodes now
// carries "MANIFEST_GATE" (mirroring COMMIT_PREFIX_GATE), but no test in the
// router package exercised the actual Recover() routing decision — the
// composed path a ledger/debugger consumer actually sees. A manifest-gate
// block is a ship-LOCAL precondition a re-audit can never re-establish, so it
// must route to the debugger, never back to audit (the cycle-230 loop).
```

### `go/internal/router/recovery_test.go:33` — above `{"ship-local ff-merge diverged", &Blocker{Code: "GIT_FF_MERGE_DIVERGED", Class: "precondition", Stage: "ship"}, "debugge…`

```text
// Ship-LOCAL preconditions: conditions a re-audit cannot re-establish
// (the cycle-230 audit↔ship loop). Ship's in-Run repair ladder already
// attempted the typed repair before this error surfaced, so the router
// hands the residue to the debugger phase — never back to audit.
```

### `go/internal/router/recovery_test.go:41` — above `{"control-plane violation → rebuild", &Blocker{Code: "CONTROL_PLANE_VIOLATION", Class: "precondition", Stage: "verify_cl…`

```text
// F37: a cycle diff touching the protected control plane is the BUILD's
// to reshape — a re-audit re-verifies the same diff and the debugger
// cannot change what a cycle may write.
```

### `go/internal/router/recovery_test.go:52` — above `{"fleet rebase needed → reaudit", &Blocker{Code: "GIT_FLEET_REBASE_NEEDED", Class: "transient", Stage: "ship"}, "audit",…`

```text
// ADR-0049 S5b: fleet ff-merge divergence is transient-classed but must
// route to RE-AUDIT (rebase + test-the-merged-tree), NOT the generic
// transient retry-ship — the dedicated handler is ordered before it.
```

### `go/internal/router/recovery_test.go:57` — above `{"fleet rebase conflict → debugger", &Blocker{Code: "GIT_FLEET_REBASE_CONFLICT", Class: "integrity", Stage: "ship"}, "de…`

```text
// ADR-0049 G13a: a fleet rebase CONFLICT (genuine overlapping work) cannot
// be re-audited away → route to the debugger, never back to audit/ship.
```

### `go/internal/router/route_scenarios_test.go:89` — above `Scenario("LLM justification captured when proposal agrees",`

```text
// Advisor-rationale capture (ADR-0024 problem #2): the justification is
// recorded on the decision whether the proposal is adopted or clamped.
```

### `go/internal/router/router.go:61` — above `UnavailablePhases []string`

```text
// UnavailablePhases is ENVIRONMENTAL context like BenchedCLIs: catalog-
// Optional phases whose persona doc does not exist (core probes every
// optional runner at plan time — 2026-09-09 token-waste root cause #2).
// The advisor is not offered them, the floor clamp drops them if proposed
// anyway, and the legacy trigger path never inserts them. Mandatory and
// floor phases are never listed here: their absence stays a loud dispatch
// failure.
```

### `go/internal/router/router.go:125` — above `type PhaseCard struct {`

```text
// PhaseCard is the advisor-facing projection of one pre-defined phase: enough
// for the planner to decide "select this" vs "mint a new one" — identity plus
// the spec's advisor-facing metadata (ADR-0038). The renderer (writeCatalog)
// caps how much of this reaches the prompt; relevance judgment stays with the
// advisor LLM, never a deterministic classifier.
```

### `go/internal/router/router.go:139` — above `AllowedCLIs       []string                    'json:"allowed_clis,omitempty"'`

```text
// AllowedCLIs + ModelTierEnvelope (cycle-436 MR1) project this phase's own
// profile guardrails into the plan prompt, so the advisor's per-phase
// {cli,tier} proposal is grounded in-bounds instead of guessing blind.
// Both nil/empty in the common case (no per-phase guardrail configured);
// the MR2 clamp re-validates regardless of whether the advisor honored
// the projection. Reuses profiles.ModelTierEnvelope's exact JSON shape
// rather than forking a router-local type.
```

### `go/internal/router/router.go:181` — above `Justification string 'json:"justification,omitempty"'`

```text
// Justification is the LLM advisor's one-sentence rationale (DynamicLLM
// mode only; empty in deterministic/static routing). Captured even when the
// proposal is clamped, so the shadow soak can diff advisor-rationale against
// the kernel's static path (ADR-0024 problem #2).
```

### `go/internal/router/router.go:204` — above `type PhasePlanEntry struct {`

```text
// PhasePlanEntry is one phase's whole-cycle run/skip decision plus the advisor's
// rationale. It is the building block of PhasePlan (ADR-0024 §2): the upfront,
// whole-cycle advisory deciding which phases run this cycle, computed once at
// cycle start. It is the cadence companion to Proposal — Proposal answers the
// per-branch "insert this optional phase?" question from post-phase signals the
// upfront plan cannot yet see.
```

### `go/internal/router/router.go:214` — above `CLI  string 'json:"cli,omitempty"'`

```text
// CLI/Tier mirror MintSpec{Tier,CLI} (cycle-436 MR1) onto an EXISTING
// (non-minted) phase's plan entry: the advisor's proposed dispatch CLI and
// abstract model TIER (fast|balanced|deep — never a raw model name). Both
// omitempty so a plan that never sets them (today's entire static-routing
// fleet) marshals BYTE-IDENTICAL to the pre-MR1 wire form — the H1
// regression floor. Advisory only: router.ClampPlanModelRouting
// re-validates against the phase's profile guardrails + the live model
// catalog before either ever reaches dispatch.
```

### `go/internal/router/router.go:240` — above `Description string 'json:"description,omitempty"'`

```text
// Description/WhenToUse are the advisor's SELECT metadata (ADR-0038),
// carrying the SAME json keys phasespec.PhaseSpec uses so the minter can
// thread them through without a vocabulary translation. Optional: omitted
// metadata mints exactly as before (cycle-1275 — the minter now satisfies
// the catalog metadata contract itself instead of metadataAllowlist padding).
```

### `go/internal/router/router.go:543` — above `if slices.Contains(in.UnavailablePhases, phase) {`

```text
// A phase whose persona doc is absent is never inserted — by the plan or by
// a trigger: the dispatch would only produce a skip (2026-09-09 token-waste
// root cause #2). Recorded as a clamp so the routing-plan artifact cites the
// exclusion; mandatory and floor phases never reach here (core lists only
// catalog-Optional, non-floor phases as unavailable).
```

### `go/internal/router/router.go:565` — above `if runs && !isFloorPhase(phase) && skipWhenFires(in.Signals, in.Cfg.Triggers[phase]) {`

```text
// A declarative skip_when (phase-catalog routing block / policy.json —
// CONFIG, never a Go literal about cycle class) gates the advisor's
// plan. Without this the plan won unconditionally for every
// non-mandatory phase, so a trivial-class cycle still burned the
// measured 0.83M-1.67M cache-read tokens per advisor-inserted optional
// (knowledge-base/research/token-usage-history-2026-07-20.md). Floor
// phases are excluded: `ship ⇒ build ∧ audit ∧ (tdd unless trivial)` is
// non-configurable, so a skip_when aimed at one must never become a
// floor bypass. The skip is RECORDED (returned optional==true → walk
// appends to SkipPhases) so the routing-plan artifact cites it rather
// than silently dropping the phase.
```

### `go/internal/router/router_memo_disabled_test.go:3` — above `import (`

```text
// router_memo_disabled_test.go — cycle-563 fix-memo-phase-dispatch, criterion 3
// (negative case): whatever fixes the silent post-ship memo-dispatch drop must
// not force memo to run unconditionally. With memo explicitly disabled via
// policy (PhaseEnable["memo"]==EnableOff), the exact same post-ship routing
// input that cycle-561's routing-decision-12.json shows resolving to "memo"
// must instead clamp to the next legal phase (here: "end", since retrospective
// is untriggered on a plain PASS cycle and memo is last in canonicalOrder) —
// and must NEVER return "memo". TestRoute_PostShip_MemoEnabled_RoutesToMemo is
// the contrasting positive case, proving the negative isn't vacuously true
// because memo was already unreachable from this input.
```

### `go/internal/router/router_psmas_test.go:3` — above `import (`

```text
// Cycle-252 task `psmas-phase-skip-wire-go-router` — TDD contract (RED first).
//
// The PSMAS gap (scout F1): digest.go extracts Triage.PhaseSkip from the
// triage handoff, but Route() never consumes it — the Go path has always
// been a no-op for PSMAS. These tests pin the wiring contract:
//
//   1. RouteInput gains a PSMASEnabled bool gate (orchestrator wires it
//      from EVOLVE_PSMAS_SKIP=1). Until the field exists this file does
//      not compile — that compile error IS the RED signal for the new API.
//   2. When enabled, Triage.PhaseSkip is unioned into the skip decision
//      ADDITIVELY: it can only skip phases that are genuinely optional
//      this cycle. Mandatory phases and the conditional tdd pin
//      (cycle_size != trivial) always win.
//   3. Triage emits persona vocabulary ("tdd-engineer", per
//      agents/evolve-triage.md §3a); the router order uses canonical
//      names ("tdd"). The wiring must normalize, or real triage output
//      silently never matches.
//   4. Gate off ⇒ byte-identical legacy behavior (PhaseSkip ignored).
```

### `go/internal/router/router_test.go:253` — above `func advisoryCfg() config.RoutingConfig {`

```text
// --- cycle-240 advisory-soak defect tests (D1 plan-veto, D3 insertion cap) ---
```

### `go/internal/router/router_test.go:273` — above `func TestRoute_AdvisoryTriggerCapEnforced(t *testing.T) {`

```text
// TestRoute_AdvisoryTriggerCapEnforced encodes cycle-238 defect D3: a
// trigger-class (EnableContent) phase scheduled by the advisory plan must
// still respect MaxInsertions. In cycle 238, 9 optional inserts ran against a
// cap of 6 because the plan path skipped the cap check wholesale.
```

### `go/internal/router/router_test.go:354` — above `func TestRoute_AdvisoryPlanRunFalse(t *testing.T) {`

```text
// TestRoute_AdvisoryPlanRunFalse encodes cycle-238 defect D1 at the kernel
// layer: an explicit plan run:false VETOES a phase whose insert_when trigger
// fires. The plan's veto outranks the content trigger.
```

### `go/internal/router/router_test.go:376` — above `func TestRoute_AdvisoryPlanVetoUserPhaseAbsentSignal(t *testing.T) {`

```text
// TestRoute_AdvisoryPlanVetoUserPhaseAbsentSignal composes D1+D2 in the exact
// cycle-238 shape: a catalog phase spliced into cfg.Order, with a
// `goal_type ne <other-goal>` trigger over a NEVER-EMITTED generic signal, and
// a plan run:false entry. Neither the fail-open trigger nor the plan path may
// run it.
```

### `go/internal/router/router_test.go:389` — above `in.Plan = spinePlan(pe("growth-loop", false))`

```text
// No Generic signals: scout.goal_type was never emitted (the cycle-238 state).
```

### `go/internal/router/signals.go:100` — above `DeliverableKind   string`

```text
// "code|document" as scout declared it; "" = undeclared (ADR-0099)
```

### `go/internal/router/signals.go:111` — above `DeliverableKind    string`

```text
// authoritative "code|document" after triage bounds top_n; "" = undeclared (ADR-0099)
```

### `go/internal/router/signals.go:160` — above `const (`

```text
// DeliverableKindCode and DeliverableKindDocument are the two deliverable
// kinds a cycle can declare (ADR-0099). Any other word is not a kind.
```

### `go/internal/router/signals.go:180` — above `func (s RoutingSignals) DeliverableKind() string {`

```text
// DeliverableKind returns the cycle's authoritative deliverable kind: triage's
// refinement when declared, else scout's, else "code". The absent default is
// the CONSERVATIVE side — with nothing digested (plan time) a rule of the form
// `deliverable_kind != document` holds, so the tdd integrity pin stays on and
// is released only by a digested document declaration (ADR-0099).
```

### `go/internal/router/signals.go:192` — above `func (s RoutingSignals) DeclaredDeliverableKind() (string, bool) {`

```text
// DeclaredDeliverableKind returns the kind a report DECLARED (triage is
// authoritative over scout) and whether one did. The integrity floor reads
// DeliverableKind (declaration or the conservative code default); a
// dispatch-time projection that may substitute the project's default kind
// reads this, so "nobody said" and "somebody said code" stay distinguishable
// (ADR-0099 slice 3).
```

### `go/internal/router/skip_when_gates_plan_test.go:9` — above `func routeC1140(t *testing.T, cycleSize, triggerPhase string, block config.RoutingBlock) RouterDecision {`

```text
// skip_when_gates_plan_test.go — cycle-1140, optional-phase-ev-gating-by-cycle-class.
// Before this gate, the Advisory+plan branch of shouldRun returned planRuns()
// directly, so an advisor-inserted optional ran on EVERY cycle class regardless
// of any configured skip_when. The ACS predicates live behind the `acs` build
// tag; these keep the behaviour in the default suite.
```

### `go/internal/router/strategy.go:34` — above `type Planner interface {`

```text
// Planner produces the advisory WHOLE-CYCLE plan (ADR-0024 §2): a run/skip
// decision + rationale for every phase, computed once at cycle start (the cheap,
// coherent half of the hybrid cadence). Segregated from Proposer so a consumer
// that only needs per-transition advice need not depend on whole-cycle planning,
// and vice versa. Like Proposer, the concrete implementation lives in package
// core; the plan is advisory and the kernel clamp remains the floor.
```

### `go/internal/router/strategy.go:65` — above `func shouldPropose(in RouteInput) bool {`

```text
// shouldPropose implements the ADR-0024 §2 hybrid cadence. When an upfront
// whole-cycle plan is driving (in.Plan != nil — set only when the orchestrator
// produced a clamped plan, i.e. Stage>=Advisory + DynamicLLM), the per-transition
// Proposer adds value ONLY at BRANCH transitions — post-build and
// post-audit, where new objective signals (acs_red, audit verdict) appear that
// the signal-poor start-of-cycle plan could not foresee. Every other transition
// is already decided by the cached plan, and a proposal can never change the
// kernel's NextPhase anyway (see applyProposal — it only annotates/clamps), so
// calling the LLM there is wasted spend. With NO upfront plan (Shadow, static
// mode, or a planner failure) the legacy per-transition cadence stands, so
// Shadow-soak forensics are unchanged.
```

### `go/internal/router/triage_report_fallback_test.go:5` — above `func TestDigest_TriageFromReportFallback_ExtractsSize(t *testing.T) {`

```text
// triage_report_fallback_test.go — ADR-0076 slice A (A1): handoff-triage.json
// has been extinct since ~cycle 215, so without a report fallback the triage
// size signal is dead and every size-conditioned budget silently multiplies by
// 1.0. triageFromReportFallback mirrors scoutFromReportFallback but is richer:
// it extracts `cycle_size_estimate: <size>` from triage-report.md so
// RoutingSignals.CycleSize() carries a real value on the live artifact path.
```

### `go/internal/router/validate.go:14` — above `func ValidatePlan(in RouteInput, plan *PhasePlan) []PlanRejection {`

```text
// ValidatePlan reports structural problems in an advisory whole-cycle plan
// (ADR-0052 WS2-S1; research principle P6 — validate before clamp). It is PURE
// and REPORT-ONLY: it never mutates the plan, never widens the run-set, and sits
// strictly ABOVE the integrity floor — ClampPlanToFloorWith still runs last,
// unconditionally, as the sole trust boundary. The orchestrator uses the
// rejections for telemetry (WS2-S2) and, once the re-plan is at advisory, to
// refuse a malformed re-plan back to the prior clamped plan rather than acting
// on garbage.
//
// Mint-aware (must-fix): a phase minted IN THIS PLAN counts as known, never
// "unknown". Checks, in order: empty plan; per entry — duplicate name, unknown
// name; and finally a run:true ship while audit is not scheduled (the floor will
// force audit, but surfacing the advisor's intent keeps the decision debuggable).
```

### `go/internal/router/validate.go:58` — above `func PlanMismatch(in RouteInput, plan *PhasePlan) bool {`

```text
// PlanMismatch reports whether the MEASURED signals materially diverge from what
// the plan scheduled (ADR-0052 WS2-S4; research P4 TAPE — mismatch-triggered
// replanning). It is true ONLY when an optional phase whose insert_when trigger
// now FIRES on the measured signals is NOT scheduled in the plan — i.e. the
// initial plan (composed with empty signals) missed a need the post-scout
// measurement reveals. It reuses triggerFires (the exact insert_when eval the
// kernel walks), so the mismatch threshold can never disagree with the trigger.
// A fired trigger the plan ALREADY covers is not a mismatch (re-planning would be
// churn); a nil plan ⇒ no mismatch. PURE.
```

### `go/internal/router/validate_test.go:11` — above `func TestValidatePlan_RejectsMalformedAndRegressive(t *testing.T) {`

```text
// TestValidatePlan_RejectsMalformedAndRegressive pins WS2-S1 (ADR-0052, research
// principle P6 "validate before clamp"): ValidatePlan is a pure, report-only
// pre-floor check. Case table — empty→reject, canonical→accept, unknown→reject,
// duplicate→reject, ship-while-skipping-audit→reject.
```
