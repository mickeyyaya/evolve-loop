# Comment history: `internal/policy`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/policy/acs_paths.go:40` — above `const MaxContractCorrectionRetries = maxContractCorrectionRetries`

```text
// MaxContractCorrectionRetries is the hard ceiling on build correction
// re-dispatches — exported for the ADR-0076 size-budget scaler, which must
// clamp its scaled limit to the same bound the resolver enforces.
```

### `go/internal/policy/advisor_skill_overlay_test.go:3` — above `import (`

```text
// advisor_skill_overlay_test.go — RED tests for the cycle-613 task
// advisor-skill-selection (.evolve/inbox/2026-07-07T18-30-00Z-advisor-skill-selection.json,
// triage top_n, weight 0.92): the advisor gains authority to PROPOSE per-phase
// skill sets, following the EXACT {cli,tier} soft-overlay precedent
// (phases/runner/runner.go:429-447, router.ClampPlanModelRouting) — "advisor
// proposes, kernel disposes". This file pins the policy-layer clamp/merge/
// registry contract; production types (AdvisorOverlayPolicy,
// AdvisorSkillRejection, Policy.ClampAdvisorSkills,
// Policy.ResolveOverlaysWithAdvisor, SkillRegistryFromFS) do not exist yet —
// RED now: internal/policy fails to compile.
//
// Scope note: this file covers the policy-layer clamp/merge/registry
// contract only (AC1 additive-merge, AC2 clamp matrix, AC3 injection guard,
// AC5 filesystem-sourced registry). The dispatch wiring into Engine.Launch /
// PhaseRequest / the advisor prompt's registry section, the
// advisor-rejections.json artifact plumbing, and the StaticPrefix
// cache-contract round-trip (AC4) are dispositioned manual+checklist in
// test-report.md — they compose EXISTING shipped mechanisms
// (RejectionsFromClamps, StaticPrefix, BaseCycleContext) verbatim once this
// clamp/merge/registry contract lands, so no new predicate is needed for
// them; Auditor verifies the wiring by inspection per the checklist.
```

### `go/internal/policy/advisor_skill_rejection_test.go:3` — above `import (`

```text
// advisor_skill_rejection_test.go — cycle-613 advisor-skill-selection.
// AdvisorSkillRejection values are logged to advisor-rejections.json, so its
// wire shape (json tags) is part of the contract the rejection-plumbing
// checklist item depends on. This pins that shape and names the type for
// apicover. Separate from advisor_skill_overlay_test.go (the RED clamp/merge
// contract) so that protected file is not modified.
```

### `go/internal/policy/artifact_budget_deeptier_test.go:3` — above `import (`

```text
// artifact_budget_deeptier_test.go — the compiled deep-tier artifact budgets.
//
// Incident (inbox item deep-phase-artifact-budget-too-small, P1/0.95): SIX
// phases died with codes=[missing_artifact] — report + acs absent, a
// content-free infra FAIL that burns the whole cycle — across FOUR phase types
// in one day: audit ×2 (cycle-1201), retro ×1 (1201), adversarial-review ×1
// (1217), tdd ×2 (1218, 1219). The last landed on a QUIET host (load 3.0, zero
// leaked processes), which rules out contention as the sole cause: load made it
// worse, the BUDGET is the floor.
//
// The arithmetic: the bridge artifact-wait base is 300s
// (bridge.tmuxArtifactTimeoutS) and the deterministic reviewer grants at most 6
// extends (bridge.defaultArtifactMaxExtends) ⇒ ~650s of effective wall clock
// before exit 81. Before this fix, defaultPhaseArtifactTimeoutS raised ONLY
// retrospective/retro, so every deep-tier (opus) phase doing real analysis on
// this repo was one slow turn from a lost cycle at ~11 minutes.
//
// An operator-side .evolve/policy.json block mitigates a configured checkout.
// These predicates pin the COMPILED defaults, i.e. the contract for a FRESH
// CLONE, which the operator block cannot reach.
//
// The two invariants that must survive together:
//  1. the four deep-tier phase labels carry 1200s;
//  2. every OTHER phase still resolves 0 — the bridge's "use the builtin"
//     sentinel — because global hang detection must not be broadly weakened to
//     fix the phases that legitimately think for a long time.
```

### `go/internal/policy/artifact_budget_deeptier_test.go:41` — above `func TestBridgePolicy_DeepTierArtifactBudgets(t *testing.T) {`

```text
// TestBridgePolicy_DeepTierArtifactBudgets pins requirement (1): a zero-value
// BridgePolicy — the REAL state of a fresh clone, whose checked-in
// .evolve/policy.json carries no bridge block — must already resolve the
// deep-tier budgets. Both vocabularies are asserted for the same reason the
// retro entry carries two keys (the cycle-1054 unit-green/live-dead defect): the
// runner dispatches Agent = the core phase NAME ("tdd", "build", "audit"), while
// the persona/agent vocabulary spells the same phases "tdd-engineer", "builder",
// "auditor". Carrying both makes a rename on either side harmless instead of
// silently restoring the 300s cliff.
```

### `go/internal/policy/artifact_budget_deeptier_test.go:64` — above `for _, label := range []string{"retrospective", "retro"} {`

```text
// retro keeps its own, smaller budget: its contract grew, but it is not a
// deep-tier analysis phase and 900s has held since cycle-1048.
```

### `go/internal/policy/basecli_test.go:5` — above `func TestBaseCLI_StripsKnownDriverSuffixes(t *testing.T) {`

```text
// TestBaseCLI_StripsKnownDriverSuffixes (mr4b AC2/AC3): policy.BaseCLI is the
// single exported base-name normalizer — this test binds the exported symbol
// name directly (apicover -enforce naming floor) and pins its documented
// semantics (cycle-440 api-contract §1): sequential-strip of "-tmux" then
// "-p", with a leading TrimSpace, adopted verbatim from the pre-existing
// unexported policy.baseCLI (the more-exercised of the two duplicated
// implementations this task consolidates — see api-contract "Resolved
// ambiguity").
```

### `go/internal/policy/bridge_config_param_test.go:60` — above `func TestBridgePolicy_PhaseArtifactTimeouts(t *testing.T) {`

```text
// TestBridgePolicy_PhaseArtifactTimeouts pins the per-phase artifact-wait
// budget resolver: compiled defaults keyed on the bridge AGENT LABEL, a
// positive-only operator merge, and a fresh map per call. The 900s retro entry
// exists because the grown retro contract does not fit the 300s builtin
// (cycle-1048's retro was ctx-canceled at ~608s); the 1200s deep-tier entries
// exist because six deep-tier phases died at ~650s with no artifact in a single
// day (see TestBridgePolicy_DeepTierArtifactBudgets for the arithmetic).
```

### `go/internal/policy/budgets_recovery.go:133` — above `type RecoveryPolicy struct {`

```text
// RecoveryPolicy is the .evolve/policy.json "recovery" block.
// It surfaces the ADR-0044 Unified Phase Recovery rollout stage so operators
// can set phase_recovery = "enforce" in policy.json without an env var, and
// (R8.5, 2026-07-16) the artifact-backed spine floor's OWN dial — split from
// phase_recovery because that stage ALSO arms the bidirectional channel
// (ADR-0045 I6) and the failure-adviser promotion path (see
// config.RolloutStages.SpineFloor for the full decoupling rationale), and
// (F27, 2026-09-26) the ADR-0044 C2 fatal-pane fast-fail's OWN dial, split
// the same way (see config.RolloutStages.FatalPane).
```

### `go/internal/policy/budgets_recovery.go:157` — above `func (p Policy) RecoveryConfig() RecoveryPolicy {`

```text
// RecoveryConfig returns recovery configuration with built-in defaults resolved.
// Empty/absent PhaseRecovery ⇒ "shadow" (behavior-neutral first-ship default);
// empty/absent SpineFloor ⇒ "enforce" (the R8.5 flip — replay-evidenced; see
// config.defaults()); empty/absent FatalPane ⇒ "enforce" (the F27 flip —
// soak-evidenced: every shadow match was a dead pane; see config.defaults()).
```

### `go/internal/policy/budgets_recovery.go:179` — above `func (p Policy) BridgeRecoveryStages() (recovery, fatalPane string) {`

```text
// BridgeRecoveryStages returns the two ADR-0044 recovery dials as the canonical
// stage words every production bridge Deps builder that does not go through the
// Loader injects (adapters/bridge.NewDefault, the subagent root). Each word is
// parsed by config.GateStage — the Loader's own trichotomy parser — so a policy
// word resolves to the SAME stage on every root (F27 architecture review: one
// parser, no root-specific case-folding; an unknown word is off, never enforce).
```

### `go/internal/policy/budgets_recovery.go:192` — above `type DocsFloorPolicy struct {`

```text
// DocsFloorPolicy is the .evolve/policy.json "docs_floor" block — the
// config-as-code dial (no flag) for the ADR-0077 documentation floor, shaped
// exactly like the SpineFloor dial it is modeled on.
```

### `go/internal/policy/ciwatch_config_param_test.go:25` — above `func TestCIWatchPolicy_KnobsFromPolicyJSON(t *testing.T) {`

```text
// TestCIWatchPolicy_KnobsFromPolicyJSON pins the AC4 contract for
// push-ci-watch-remote-parity (cycle-748): every knob (enabled, timeout, poll
// interval) resolves from a policy.json ci_watch block; an ABSENT block yields
// the compiled defaults (watch enabled — gates default ON as compiled Go
// defaults, the observer/fleet pattern); malformed values are rejected
// explicitly rather than silently zeroed.
```

### `go/internal/policy/context_fill_config.go:3` — above `const defaultContextFillWarnPct = 60`

```text
// context_fill_config.go — the .evolve/policy.json "context_fill" block
// (cycle-1444, task `context-fill-warn-threshold`). Shape mirrors
// ParallelEvaluatePolicy, the canonical sub-block resolution pattern in this
// package: a pointer block on Policy plus a pure resolver that applies
// built-in defaults. Config-as-code, no flags — see
// [[phase_settings_from_config_not_code]].
```

### `go/internal/policy/context_fill_config_test.go:3` — above `import (`

```text
// context_fill_config_test.go — RED contract for cycle-1444 task
// `context-fill-warn-threshold`. Mirrors parallel_evaluate_config_test.go, the
// canonical sub-block resolution shape in this package.
//
// RED: policy.ContextFillPolicy, policy.ContextFillConfig and
// Policy.ContextFillConfig() do not exist yet; this file fails to COMPILE until
// Builder adds them (compile-fail = RED evidence).
//
// The invariant every case below defends: operator input is never accepted
// verbatim. Absent, empty, and out-of-range all resolve to the built-in 60%
// default — a typo must never silence the WARN (0) nor arm it on every launch.
```

### `go/internal/policy/core.go:44` — above `type IntegrityPolicy struct {`

```text
// IntegrityPolicy configures the per-phase binary-integrity model (ADR-0065).
```

### `go/internal/policy/core.go:232` — above `func BaseCLI(cli string) string {`

```text
// BaseCLI is the single exported base-name normalizer for driver-qualified CLI
// names: claude-tmux/claude-p → claude, codex-tmux → codex, agy-tmux → agy.
// It strips "-tmux" then "-p" repeatedly until neither suffix matches, so a
// (never-occurring-in-practice) doubly-qualified name like "codex-tmux-p"
// still resolves to its bare family "codex". This is the ONE exported source
// consolidating the formerly-duplicated policy.baseCLI and
// bridge.baseCLIName (cycle-440 MR4b, F2/F3).
```

### `go/internal/policy/dispatch.go:138` — above `var defaultPhaseArtifactTimeoutS = map[string]int{`

```text
// defaultPhaseArtifactTimeoutS is the compiled per-phase artifact-wait budget
// (seconds), keyed on the bridge AGENT LABEL — the vocabulary
// core.BridgeRequest.Agent carries and Engine.Launch dispatches with.
// internal/phases/retro launches Agent:"retrospective", so a map keyed only on
// the core phase name "retro" would be unit-green and live-dead; both
// vocabularies carry the budget because the phase-name/agent-label skew is
// permanent.
//
// retro=900s: the grown retro contract (report + preventive_actions +
// disposition.json) no longer fits the 300s builtin — cycle-1048's retro was
// ctx-canceled at ~608s mid-authoring, losing the artifact entirely.
//
// The deep-tier analysis phases (tdd, build, audit, adversarial-review) carry
// 1200s: the bridge's base artifact-wait is 300s (bridge.tmuxArtifactTimeoutS)
// and the deterministic reviewer grants at most 6 extends
// (bridge.defaultArtifactMaxExtends), so ~650s of wall clock was the effective
// ceiling for every phase this map did not name. SIX cycles died there in one
// day with codes=[missing_artifact] — report + acs absent, a content-free infra
// FAIL — across FOUR phase types (audit ×2 and retro on cycle-1201,
// adversarial-review on 1217, tdd on 1218/1219), the last on a QUIET host, which
// rules out contention as the sole cause: load made it worse, the budget was the
// floor. An opus-tier agent doing real analysis on this repo legitimately needs
// more than 11 minutes. Both vocabularies are listed for the same reason retro
// carries two keys (the cycle-1054 unit-green/live-dead defect): the runner
// dispatches Agent = the core phase NAME ("tdd"/"build"/"audit"), while the
// persona vocabulary spells the same phases "tdd-engineer"/"builder"/"auditor",
// and the skew is permanent.
//
// This list stays NARROW by design. Every phase NOT named here deliberately
// keeps the 300s builtin, so global hang detection is not weakened across the
// board to fix the phases that legitimately think for a long time — a wedged
// scout/intent/triage/ship is still surfaced in ~11 minutes, not ~2.3 hours.
```

### `go/internal/policy/docsfloor_config_param_test.go:11` — above `func TestDocsFloorConfigDefaults(t *testing.T) {`

```text
// TestDocsFloorConfigDefaults pins the compiled default and every override
// path: the ADR-0077 floor is armed without a policy block, and the operator
// dial in .evolve/policy.json reaches it (config-injected, no flag).
```

### `go/internal/policy/fleet.go:73` — above `BinaryRefresh string 'json:"binary_refresh,omitempty"'`

```text
// BinaryRefresh selects the boot-time binary staleness self-heal
// (cmd_loop_boot_refresh.go; docs/chronicle/2026-08-binary-lag.md):
// "auto" (default) rebuilds + re-execs when the running binary's build
// stamp is behind HEAD with a go/ source delta; "off" disables it for
// deliberate old-binary pins (incident bisects). Unknown words resolve to
// "auto" — the self-heal is integrity posture, so a typo must not
// silently disable it.
```

### `go/internal/policy/fleet.go:93` — above `type WorktreePolicy struct {`

```text
// WorktreePolicy is the .evolve/policy.json "worktree" block. Replaces the
// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
```

### `go/internal/policy/fleet.go:277` — above `const (`

```text
// defaultStarvationK / defaultStarvationWeight are the compiled fleet-starvation
// defaults, surfaced even on the p.Fleet==nil path (the cycle-542 C542_005
// lesson: seed struct-literal defaults BEFORE the nil early return).
```

### `go/internal/policy/fleet_config_param_test.go:3` — above `import (`

```text
// FleetPolicy/FleetConfig — the .evolve/policy.json "fleet" block (S1 of the
// FLEET-AS-POLICY operator-priority goal, cycle 464), mirroring the
// SwarmPolicy/SwarmConfig precedent exactly (policy.go:779-845): a raw
// *FleetPolicy block on Policy, a resolved FleetConfig struct, and a
// FleetConfig() getter with fail-safe defaults. Absent block ⇒ Count=1
// (byte-identical sequential execution); Concurrency<=0 ⇒ follows the
// resolved Count; PlanSource is closed-vocabulary ("triage"|"manual") with
// an unknown value failing safe to "manual" PLUS a surfaced warning (unlike
// the swarm/parallel_evaluate precedents, which fail back to their DEFAULT
// value — this block's spec explicitly calls for the non-default fail-safe
// branch, so the getter is not I/O: it returns the warning as data on the
// resolved config rather than logging, matching the package's no-I/O-in-
// getters style).
//
// Black-box: drives only the exported Policy/FleetPolicy/FleetConfig
// surface, zero env.
```

### `go/internal/policy/fleet_config_param_test.go:80` — above `func TestFleetConfig_MinLanesResolution(t *testing.T) {`

```text
// TestFleetConfig_MinLanesResolution pins the min_lanes floor resolution
// (2026-07-03): absent/≤1 ⇒ 1 (historical min-1 shrink); a positive override
// raises the floor but clamps to ≤ Count (a floor above the lane count is
// meaningless). This floor is what lets the quota-aware wave shrink keep the
// operator's asserted concurrent-lane budget through a transient CLI bench.
```

### `go/internal/policy/fleet_config_scheduling_test.go:3` — above `import (`

```text
// fleet_config_scheduling_test.go — RED contract for cycle-550's
// supervisor-continuous-lane-keeping task (L5, the fleet-width architecture's
// "ceiling-keeper").
//
// PROBLEM (inbox 2026-07-06T16-00-00Z-supervisor-continuous-lane-keeping.json,
// operator directive verbatim: "the supervisor must try its best to honor the
// setting"): today's wave scheduler is synchronized on a barrier — plan wave,
// dispatch N lanes, WAIT for every lane to finish, THEN plan the next wave. A
// lane that exits early (PASS or FAIL) cannot be replaced until every sibling
// lane in its wave also finishes, so realized concurrency is min-over-time,
// not the operator's configured fleet.count. The fix is a rolling lane pool
// (fleet.RunPool, see pool_test.go) that backfills a replacement lane
// immediately on any lane exit. Rollout is config-gated per repo precedent
// (mirrors SwarmPolicy.Stage / FleetBudgetPolicy.Stage's shadow-first idiom):
// a NEW `policy.fleet.scheduling` closed-vocab knob, "wave" (today's
// behavior, the default) or "pool" (the rolling lane pool).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence, mirroring the cycle-465/507/547 precedent):
//
//	FleetPolicy gains a `Scheduling string `json:"scheduling,omitempty"``
//	field (raw .evolve/policy.json value) and FleetConfig gains a resolved
//	`Scheduling string` field. FleetConfig() resolves it as a closed
//	vocabulary: empty/absent AND explicit "wave" -> "wave" (byte-identical
//	regression: every OTHER resolved field on FleetConfig is untouched by
//	this knob); explicit "pool" -> "pool"; any OTHER value (operator typo)
//	fails safe to "wave" -- NOT "pool" -- plus a surfaced warning naming the
//	rejected value, because "wave" is the current/safe default this knob
//	must never silently escalate away from (unlike PlanSource, whose unknown
//	value fails to "manual" per its own spec — see
//	TestFleetConfig_PlanSourceClosedVocab in fleet_config_param_test.go for
//	the contrasting precedent this test intentionally does NOT mirror).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/Semantic : empty/"wave"/"pool" all resolve to their own value,
//     no warning.
//   - Negative           : an unknown value ("yolo") must NOT pass through
//     unchanged (a naive passthrough getter fails this) and must NOT resolve
//     to "pool" (a fail-open-to-the-new-mode bug would be worse than a
//     passthrough — it would silently opt an operator into the unsoaked
//     rolling pool). It must fail safe to "wave" WITH a warning.
//   - Regression         : an absent/empty fleet.scheduling must leave
//     Count/Concurrency/PlanSource/MinLanes resolution completely unaffected
//     (byte-identical wave-mode config), independent of this new field.
```

### `go/internal/policy/floorgate_test.go:8` — above `func TestFloorGate_ParsedFromFloorKey(t *testing.T) {`

```text
// TestFloorGate_ParsedFromFloorKey verifies the policy.json `floor` array (the
// ADR-0055 D3 closeout-gate enrollment, e.g. "dossier-closeout") is unmarshaled
// into Policy.Floor. Before the 2026-06-22 doc↔impl audit this key was declared
// in the checked-in policy.json but silently dropped — Policy had no `floor`
// field — so the gate it named enforced nothing (Potemkin enforcement).
```

### `go/internal/policy/integrity_config_param_test.go:5` — above `func TestIntegrityMode(t *testing.T) {`

```text
// IntegrityMode resolves the integrity sub-policy (ADR-0065) with safe
// defaults: absent/partial/unknown ⇒ pipeline + shadow + provenance-required.
// The default is byte-neutral with today's behavior (pipeline mode = the
// existing single-pin check; shadow stage = log-only).
```

### `go/internal/policy/memo_envelope_config_test.go:11` — above `func shippedMemoPinAndProfile(t *testing.T) (policy.Pin, *profiles.Profile) {`

```text
// memo_envelope_config_test.go — RED contract for cycle-573 Task 1
// (memo-phase-tier-envelope, inbox weight 0.95 critical). The shipped config
// carries a split: .evolve/policy.json pins the memo phase to model "fast"
// (tier rank 1) while .evolve/profiles/memo.json declares a
// model_tier_envelope of [balanced..balanced] (rank 2). Every PASS cycle where
// memo actually dispatches records the abnormal
//
//	policy: pin for phase "memo": model "fast" (tier rank 1) outside envelope [balanced..balanced]
//
// Per phase_settings_from_config_not_code, the fix is config-only: align the
// pin's tier to the profile envelope (or vice-versa) in the shipped JSON — no
// Go literal changes. These tests read the SHIPPED files (same locator the
// profile routing tests use: filepath.Join("..","..","..",".evolve",...)) so
// they pin the on-disk contract, not a fixture.
//
// RED today: TestMemoPin_WithinShippedEnvelope fails because ValidatePin
// returns the "outside envelope" error for fast-vs-balanced. GREEN once the
// config drift is resolved.
```

### `go/internal/policy/memo_envelope_config_test.go:30` — above `func shippedMemoPinAndProfile(t *testing.T) (policy.Pin, *profiles.Profile) {`

```text
// shippedMemoPinAndProfile loads the shipped memo pin when one exists.
// 2026-08-14: the checked-in policy no longer pins memo — `setup apply
// --preset recommended` (operator-chosen) superseded the old agy/fast pin, so
// memo rides its profile default (balanced). The envelope-coherence contract
// below stays armed for ANY future pin; absence is legal and skips.
```

### `go/internal/policy/observation_mask_param_test.go:3` — above `import (`

```text
// ObservationMaskPolicy / ObservationMaskConfig — the .evolve/policy.json
// "observation_mask" block (cycle-530, research-backed #1 token lever). Mirrors
// the RouterPolicy raw==resolved idiom: a raw *ObservationMaskPolicy block on
// Policy that doubles as the ObservationMaskConfig() getter's return type, with
// a single WindowTurns knob defaulting to 10. Black-box: drives only the
// exported Policy/ObservationMaskPolicy surface, zero env.
```

### `go/internal/policy/overlays.go:1` — above `package policy`

```text
// overlays.go — skill-overlay resolver (cycle-609 skill-overlays-bridge-layer).
//
// The nil-able-pointer + resolver idiom mirrors ObserverPolicy/ObserverConfig
// (policy.go): Policy.Overlays == nil ⇒ the compiled default applies; a non-nil
// block with an empty Rules slice is an explicit operator opt-out (zero
// overlays, NOT the default); a non-nil block with rules resolves the UNION of
// every matching rule's skills, deduped, in stable (first-seen) order.
//
// The core is the policy-layer resolver — a pure, side-effect-free mapping from
// a dispatch descriptor to an ordered skill list (ResolveOverlays). The
// producers that construct an OverlayDispatch from a live launch already landed:
// the phase runner threads ResolveOverlays onto BridgeRequest.Skills per tiered
// dispatch (phases/runner/runner.go), and the non-phase dispatch sites
// (subagent.Run, retro.Phase.Run, swarmrunner's launcher) resolve through
// ResolveLaunchOverlaysFailOpen below. skills/fable/ is already covered by the
// ProtectedSurfaceManifest (guards/integrity_surface.go), so the F1 surface is
// live, not dormant.
```

### `go/internal/policy/overlays.go:67` — above `When   []config.Condition 'json:"when,omitempty"'`

```text
// When keys the rule on the CYCLE's objective signals (ADR-0099 slice 3):
// every clause must hold against OverlayDispatch.Signals. An absent
// signal never matches (fail-closed, the D2 discipline), so a rule keyed
// on `deliverable_kind == document` is inert until core has projected a
// document kind. The clause type is the kernel's own (config.Condition),
// string-valued here: {"field": "deliverable_kind", "op": "eq", "value": "document"}.
```

### `go/internal/policy/overlays.go:97` — above `{Phases: []string{"scout"}, When: document, Skills: []string{"solution-scout"}},`

```text
// ADR-0099 slice 3: a document cycle's discovery, build and audit run
// under the solution-skill personas — deterministic, signal-keyed, and
// inert for every code cycle (absent/`code` signal ⇒ no match).
```

### `go/internal/policy/overlays_apicover_salvage_test.go:3` — above `import (`

```text
// overlays_apicover_salvage_test.go — apicover Phase-5 naming coverage for the
// salvaged cycle-943/950 export (false-RED salvage, post-v22.4.2): NAMES +
// EXERCISES ResolveLaunchOverlaysFailOpen. Behavioral (Rule 9): the fail-open
// contract is asserted — a missing/malformed policy.json degrades to the
// compiled-default overlays instead of aborting the launch.
```

### `go/internal/policy/overlays_producer_test.go:3` — above `import (`

```text
// Overlay dispatch producer — cycle-867 task overlay-dispatch-producer.
//
// overlays.go's resolver (ResolveOverlays/ResolveOverlaysWithAdvisor) has been
// fully implemented since cycle-609 but has zero non-test callers: nothing
// constructs an OverlayDispatch from live cycle dispatch data. This is RED —
// policy.DispatchFromPhaseRequest does not exist yet. It is a pure,
// side-effect-free field mapping (phase/cli/model/tier -> OverlayDispatch);
// the caller resolves the routing-mode tier logic BEFORE calling (empty tier
// for the non-auto degrade floor, populated tier only under
// model_routing=auto with a non-nil clamped plan — mirrors
// core.PhaseRequest.ModelRoutingCLI/ModelRoutingTier, cyclerun_dispatch.go).
// This function does not touch bridge.Engine.Launch or
// guards/integrity_surface.go's ProtectedSurfaceManifest — that wiring is an
// explicit out-of-cycle manual-ship carryover per the overlays.go header.
```

### `go/internal/policy/overlays_test.go:3` — above `import (`

```text
// Overlays resolver — cycle-609 task skill-overlays-bridge-layer.
//
// Nil-able-pointer + resolver config idiom (ObserverPolicy exemplar,
// policy.go:483-539): Policy.Overlays == nil ⇒ the compiled default
// {tiers:[deep,top]}->[fable] applies; a non-nil block with an empty
// Rules slice is an explicit operator opt-out (zero overlays, not the
// default); a non-nil block with rules resolves the UNION of every
// matching rule's skills, deduped, in stable (first-seen) order.
//
// This is RED: internal/policy has no OverlaysPolicy/OverlayRule type and
// no ResolveOverlays method yet — Builder implements per the requeued spec
// (.evolve/inbox/2026-07-07T19-30-00Z-skill-overlays-bridge-layer-requeue.json).
```

### `go/internal/policy/overlays_when_test.go:13` — above `func TestResolveOverlays_WhenSelector(t *testing.T) {`

```text
// ADR-0099 slice 3 — an overlay rule may key on a cycle SIGNAL (`when`), not
// only on phase/cli/model/tier: a document cycle preloads the solution skills
// into the scout, build and audit dispatches. Absent signal ⇒ the rule does
// not match (fail-closed, the D2 discipline); the selector is deterministic
// config, the advisor may still ADD skills through its clamp.
```

### `go/internal/policy/policy.go:31` — above `type FloorGate struct {`

```text
// FloorGate is one entry in the policy `floor` array (ADR-0055 D3): a named
// closeout gate every completed cycle must satisfy before a batch is considered
// clean. The canonical entry is "dossier-closeout" — every cycle must write a
// dossier to knowledge-base/cycles/cycle-N.json, enforced by `evolve dossier
// verify`. NOTE: this is the closeout-gate array (`floor`), distinct from
// ShipFloor (`ship_floor`, the per-plan integrity floor of PHASES). Before the
// 2026-06-22 doc↔impl audit the `floor` key was present in the checked-in
// policy.json but had NO struct field, so json.Unmarshal silently dropped it and
// the gate it declared enforced nothing.
```

### `go/internal/policy/policy.go:62` — above `Floor []FloorGate 'json:"floor,omitempty"'`

```text
// Floor is the closeout-gate array (ADR-0055 D3): named gates every
// completed cycle must satisfy (e.g. "dossier-closeout"). Distinct from
// ShipFloor above (which lists PHASES). Absent ⇒ no closeout gates. Read by
// `evolve dossier verify` to decide whether a missing dossier fails the
// batch. (See FloorGate for the Potemkin-enforcement bug this field fixes.)
```

### `go/internal/policy/policy.go:74` — above `SystemFailurePolicy *SystemFailurePolicy 'json:"failure_policy,omitempty"'`

```text
// FailurePolicy is the declarative system-failure DECISION policy
// (ADR-0072): the category→action mapping the orchestrator classifies
// against, the non-negotiable Go-enforced floor categories
// (verdict-incoherence, infra-systemic — always halt), and the
// non-progress/retry thresholds. Distinct from FailureFloor (which tunes
// the LLM-learning layer) and Classify (the hang reclassifier). Absent ⇒
// compiled defaults (DefaultFailurePolicy). See policy_failure.go.
```

### `go/internal/policy/policy.go:139` — above `ReportBudget *ReportBudgetPolicy 'json:"report_budget,omitempty"'`

```text
// ReportBudget configures the report-size gate's per-artifact token budgets
// (cycle-565 Slice S1). Absent ⇒ built-in defaults apply (HandoffTokens=2000).
```

### `go/internal/policy/policy.go:142` — above `RetroAutofile *RetroAutofilePolicy 'json:"retro_autofile,omitempty"'`

```text
// RetroAutofile configures the retro→inbox preventive-actions autofiler
// (internal/retrofile, cycle-657). Absent ⇒ built-in safe default applies
// (DefaultWeight=0.75).
```

### `go/internal/policy/policy.go:174` — above `Recovery *RecoveryPolicy 'json:"recovery,omitempty"'`

```text
// Recovery configures the ADR-0044 Unified Phase Recovery rollout stage.
// Absent ⇒ built-in default applies (PhaseRecovery="shadow" — behavior-neutral).
```

### `go/internal/policy/policy.go:177` — above `DocsFloor *DocsFloorPolicy 'json:"docs_floor,omitempty"'`

```text
// DocsFloor configures the ADR-0077 documentation floor for
// architecture-labeled changes. Absent ⇒ built-in default applies
// (Stage="enforce" — the floor only WARNs, so arming it by default is
// byte-neutral to every verdict).
```

### `go/internal/policy/policy.go:188` — above `Worktree *WorktreePolicy 'json:"worktree,omitempty"'`

```text
// Worktree configures the per-cycle worktree base path. Absent ⇒ built-in
// default (<root>/.evolve/worktrees). Replaces the EVOLVE_WORKTREE_BASE env
// read (flag-reduction, ADR-0064): the operator override now flows from this
// config block, loaded once, rather than a process env dial.
```

### `go/internal/policy/policy.go:193` — above `Integrity *IntegrityPolicy 'json:"integrity,omitempty"'`

```text
// Integrity configures the binary self-SHA integrity model (ADR-0065).
// Absent ⇒ Mode="pipeline", Stage="shadow", ProvenanceRequired=true —
// byte-neutral with the legacy single-pin ship check. Mode="phase" verifies
// the per-phase agent-block chain; Stage="enforce" blocks (shadow logs only).
```

### `go/internal/policy/policy.go:206` — above `Chain *ChainPolicy 'json:"chain,omitempty"'`

```text
// Chain configures batch chaining for `evolve loop --until-inbox-empty`
// (cycle 1075). Absent ⇒ chaining off with the compiled max_batches cap.
// See policy_chain.go.
```

### `go/internal/policy/policy_chain.go:3` — above `const DefaultChainMaxBatches = 20`

```text
// policy_chain.go — the `chain` block: batch-chaining defaults for
// `evolve loop --until-inbox-empty` (cycle 1075, inbox item
// loop-batch-chaining). Chaining is an operator CAPABILITY, not a feature
// flag: the CLI parameter is the opt-in and this block supplies its bound, in
// the same compiled-default-then-policy-override shape as `workflow`/`gates`.
```

### `go/internal/policy/policy_chronicle_test.go:3` — above `import (`

```text
// TDD contract for chronicle-s2-digest-writer (cycle 702): the "chronicle"
// policy block, mirroring the GatesPolicy default-then-override idiom.
// RED until policy_chronicle.go defines ChroniclePolicy + Policy.ChronicleConfig().
//
// Compiled defaults (per the approved chronicle plan): digest=shadow,
// digest_tokens=1200, digest_cycles=10, escalation=shadow, historian=off.
// An absent block resolves the defaults; a present block overrides only the
// fields it sets (no feature flags — policy-driven stages only).
```

### `go/internal/policy/policy_ciwatch.go:5` — above `type CIWatchPolicy struct {`

```text
// CIWatchPolicy configures the post-push GitHub CI watch and the release
// preflight CI hard-gate (cycle-748, push-ci-watch-remote-parity). All knobs
// live in the policy.json `ci_watch` block — zero env flags by design.
// Pointer fields preserve the distinction between an omitted value and an
// explicit zero/false override.
```

### `go/internal/policy/policy_failure.go:5` — above `const (`

```text
// ADR-0072 — the system-failure DECISION policy. This is the declarative
// surface the orchestrator classifies each failure against; Go enforces the
// floor (verdict-incoherence, infra-systemic ALWAYS halt) and provides the
// deterministic fallback. See docs/architecture/adr/0072-system-failure-policy-and-halt.md.
//
// NOTE: the type is SystemFailurePolicy (not FailurePolicy) because
// (Policy).FailurePolicy() already resolves the distinct failure_floor
// LLM-learning block — this is the failure DECISION policy, a separate surface.
```

### `go/internal/policy/policy_failure.go:50` — above `type FailureThresholds struct {`

```text
// FailureThresholds are the non-progress / retry counters (ADR-0072 S2/S5).
```

### `go/internal/policy/policy_failure.go:69` — above `UnexplainedFailuresHaltCeiling int 'json:"unexplained_failures_halt_ceiling,omitempty"'`

```text
// UnexplainedFailuresHaltCeiling: failures with NO machine-readable reason
// reaching this in one batch ⇒ diagnosability halt (batch-6 first-firing:
// three distinct reason-less failures shared one degenerate fingerprint).
```

### `go/internal/policy/policy_failure.go:73` — above `ConsecutiveFailuresHaltCeiling int 'json:"consecutive_failures_halt_ceiling,omitempty"'`

```text
// ConsecutiveFailuresHaltCeiling: this many cycles failing back-to-back
// (any fingerprints) ⇒ pipeline-blocker halt + deep-dive before further
// dispatch (operator directive 2026-08-10: the 2026-08-09 batch burned 10
// failed cycles / 0 ships before an identity-keyed rule tripped).
```

### `go/internal/policy/policy_failure.go:78` — above `BuildDeepEscalateAtFailures int 'json:"build_deep_escalate_at_failures,omitempty"'`

```text
// BuildDeepEscalateAtFailures: an item whose failure_count has reached
// this routes its NEXT build to the deep tier (ADR-0076 D — retrying a
// hard item at the same tier re-fails identically). Raise-only; envelope
// Max still clamps. 0 keeps the compiled default (per-threshold merge
// convention); disable via the core escalation seam, not this knob.
```

### `go/internal/policy/policy_failure.go:94` — above `func DefaultSystemFailurePolicy() SystemFailurePolicy {`

```text
// DefaultSystemFailurePolicy is the compiled default surfaced when the
// failure_policy block is absent. It matches ADR-0072's table so behavior is
// correct without editing the checked-in policy.json (mirrors the
// gates/observer default pattern).
```

### `go/internal/policy/policy_failure.go:187` — above `func (fp SystemFailurePolicy) RetryPolicyFor(category string) (FailureCategory, bool) {`

```text
// RetryPolicyFor returns the declarative retry policy for a failure category —
// the Action / MaxRetries / FixType fields that ADR-0072 has always declared and
// that nothing consumed. It completes the accessor set beside IsFloor and
// IsSystemLevel so the retry decision reads the SAME table the floor does,
// instead of a parallel knob (the max_audit_repair_attempts duplication).
//
// ok is false for an unrecognised category; callers must treat that as
// "no retry", never as a default-allow.
```

### `go/internal/policy/policy_failure_consecutive_test.go:5` — above `func TestDefaultConsecutiveFailuresHaltCeilingIsThree(t *testing.T) {`

```text
// Pins the compiled default for the consecutive-failures halt (operator
// directive 2026-08-10) and its per-threshold merge override.
```

### `go/internal/policy/policy_failure_named_test.go:5` — above `func TestSystemFailurePolicy_FloorAndLevelPredicates(t *testing.T) {`

```text
// Names the ADR-0072 failure-policy exported surface (apicover graduation) by
// exercising the floor/level predicates and the category/level/action vocab —
// meaningful assertions, not bare symbol references.
```

### `go/internal/policy/policy_failure_test.go:8` — above `func TestFailurePolicyConfig_AbsentBlock_CompiledDefaults(t *testing.T) {`

```text
// ADR-0072 S1: the failure_policy decision block. Absent ⇒ compiled defaults;
// the two floor categories (verdict-incoherence, infra-systemic) are
// non-negotiable — the resolver forces floor=true even if operator policy tries
// to unset them (mirrors ShipFloor always re-appending "audit").
```

### `go/internal/policy/policy_manifestgate_test.go:8` — above `func TestGatesConfig_ManifestGateDefaultsToShadow(t *testing.T) {`

```text
// TestGatesConfig_ManifestGateDefaultsToShadow pins the behavior-preserving
// default for the new dial (cycle-1064, task manifest-gate-policy-wiring). The
// manifest gate is shadow-first, exactly like ReportSizeGate: observed before it
// can block a cycle. An absent `gates` block and a present-but-silent one must
// both resolve to "shadow".
```

### `go/internal/policy/policy_reportsize_test.go:8` — above `func TestGatesConfig_ReportSizeGate_DefaultsShadow(t *testing.T) {`

```text
// policy_reportsize_test.go — RED contract for cycle-565 Slice S1
// (report-size-contracts-jit-artifacts, this fleet lane's sole triage-committed
// top_n task). The new report-size gate is its own rollout dial in GatesPolicy/
// GatesConfig (mirrors ContractGate/EvalGate/TriageCapGate) but defaults to
// "shadow" — the inbox item explicitly calls for shadow/warn BEFORE enforce,
// unlike the existing gates which already default to enforce. The token
// budget itself is a separate policy-configured value (~2K default, per
// phase_settings_from_config_not_code — zero Go literals reachable only
// through code, always overridable via .evolve/policy.json).
//
// RED today: GatesPolicy has no ReportSizeGate field and Policy has no
// ReportBudgetConfig()/ReportBudgetPolicy (compile failure).
```

### `go/internal/policy/policy_test.go:89` — above `{"top", 4},`

```text
// cycle-516: "top" is the 4th canonical tier (modelcatalog.CanonicalTiers),
// must rank strictly above deep/opus (3) so an envelope ceiling of "deep"
// still excludes it.
```

### `go/internal/policy/policy_test.go:213` — above `func TestValidatePin_TopTierOutsideDeepEnvelope(t *testing.T) {`

```text
// TestValidatePin_TopTierOutsideDeepEnvelope pins the cycle-516 wiring
// contract: once "top" ranks above "deep" (4 > 3), a profile whose ceiling is
// still the pre-4-tier "deep" must keep rejecting a "top" pin. Before TierRank
// learns "top", it classifies as rank 0 (unclassifiable) and
// TestValidatePin_UnclassifiableModelSkipsEnvelope's own rule silently exempts
// it from the envelope check — the vocabulary addition must not leave a
// deep-ceilinged profile suddenly wide open to the frontier tier.
```

### `go/internal/policy/policy_test.go:397` — above `func TestWorkflowConfig_UniversalFallbackExcludeDefaultsToAgy(t *testing.T) {`

```text
// TestWorkflowConfig_UniversalFallbackExcludeDefaultsToAgy — the last-resort
// tail every launch walks (2026-09-14 policy: try every available CLI before
// giving up) never contains the agy family unless the operator lifts the
// 2026-06-07 ban explicitly with workflow.universal_fallback_exclude=[].
```

### `go/internal/policy/recovery_config_param_test.go:3` — above `import (`

```text
// RecoveryPolicy — the ADR-0044 Unified Phase Recovery config (cycle-12 flag retirement).
// PhaseRecovery defaults "shadow" (behavior-neutral); absent block is safe.
// R8.5 (2026-07-16): SpineFloor — the spine floor's OWN dial — defaults
// "enforce" (replay-evidenced flip); "shadow" is the policy escape hatch.
// F27 (2026-09-26): FatalPane — the ADR-0044 C2 fatal-pane fast-fail's OWN
// dial — defaults "enforce" (soak-evidenced flip); "shadow" is its hatch.
```

### `go/internal/policy/recovery_config_param_test.go:88` — above `"fatal-pane-shadow-escape-hatch",`

```text
// F27 escape hatch: dial the fatal-pane fast-fail back to shadow
// via policy.json, no recompile, without touching phase_recovery.
```

### `go/internal/policy/recovery_config_param_test.go:95` — above `"phase-recovery-off-leaves-fatal-pane",`

```text
// The dials are independent: silencing the whole ADR-0044 program
// does not silence the fatal-pane fast-fail (its own dial).
```

### `go/internal/policy/recovery_config_param_test.go:115` — above `func TestBridgeRecoveryStages_ParsesThroughTheLoaderTrichotomy(t *testing.T) {`

```text
// TestBridgeRecoveryStages_ParsesThroughTheLoaderTrichotomy (F27 review fold):
// the accessor every setter-less bridge root seeds from parses each word with
// config.GateStage, the Loader's own parser — so "Enforce" is off here exactly
// as at the cycle root (it was enforce under the bridge's case-folding
// normalizer), and a typo never arms the kill path.
```

### `go/internal/policy/regressiontia_test.go:5` — above `func TestRegressionTIAConfig_DefaultsOff(t *testing.T) {`

```text
// regressiontia_test.go — RED contract for cycle-1260 Task 1
// (`egps-regression-tia-shadow-wiring`, inbox item
// .evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json,
// P1 weight 0.91, 3rd live instance).
//
// The config surface. Test-impact selection over the EGPS regression corpus is
// a staged rollout (off → shadow → enforce), and per the standing rules it is
// config-as-code in .evolve/policy.json — never a flag, never a Go literal
// ([[phase_settings_from_config_not_code]], [[no_feature_flags_use_design_patterns]]).
// The shape is copied verbatim from the sibling ParallelEvaluate/Disposition
// blocks: a `{"stage": "..."}` object, resolved through an accessor that
// applies defaults and fail-safes.
//
// The contract these tests freeze:
//
//	type RegressionTIAPolicy struct{ Stage string `json:"stage,omitempty"` }
//	func (p Policy) RegressionTIAConfig() RegressionTIAConfig   // {Stage string}
//
//   - absent block (nil pointer) / zero-value Policy{} ⇒ Stage "off". The
//     checked-in .evolve/policy.json carries NO regression_tia block, so "off"
//     is the live production default and must be a byte-identical no-op.
//   - the closed vocabulary {"off","shadow","enforce"} is honored verbatim.
//   - ANY unknown value (typo, empty-after-trim, "on", "true") maps to "off" —
//     fail-safe, so a misspelling can never silently arm test selection and
//     hide a regression class. This is the negative axis and the whole point:
//     the failure mode this item exists to prevent is selection that skips a
//     predicate which would have caught a real red.
//
// RED today: RegressionTIAConfig/RegressionTIAPolicy are undefined, so package
// policy's tests fail to COMPILE — a hard non-zero exit, never a silent pass.
```

### `go/internal/policy/remediation_policy_test.go:5` — above `func TestWorkflowConfig_RemediationDefaults(t *testing.T) {`

```text
// TestWorkflowConfig_RemediationDefaults pins the graduated-remediation
// compiled defaults (2026-07-21): ON at 1 round for coverage-gate only, with
// policy.json workflow overrides honored — including explicit 0 to disable.
```

### `go/internal/policy/research_config.go:3` — above `const (`

```text
// research_config.go — the .evolve/policy.json "research" block (cycle-1494,
// task `sleep-time-kb-consolidation`). Shape mirrors ContextFillPolicy, the
// canonical sub-block resolution pattern in this package: a pointer block on
// Policy plus a pure resolver that applies built-in defaults. Config-as-code,
// no flags — see [[phase_settings_from_config_not_code]].
```

### `go/internal/policy/retro_autofile_test.go:5` — above `func TestRetroAutofileDefaultWeight_DefaultAndOverride(t *testing.T) {`

```text
// TestRetroAutofileDefaultWeight_DefaultAndOverride encodes AC3's "weight
// defaults from policy.json" half for the retro→inbox autofiler
// (retro-preventive-actions-autofile-inbox, cycle 657): the default weight for
// auto-filed preventive-action items is sourced from policy, not a Go literal
// (feedback_phase_settings_from_config_not_code), with a safe built-in fallback
// when the policy block is absent.
//
// RED until the Builder adds a RetroAutofile policy block + the
// RetroAutofileDefaultWeight() accessor. DO NOT modify this test — implement
// production code to green it.
```

### `go/internal/policy/retry_escalation_policy_test.go:3` — above `import "testing"`

```text
// retry_escalation_policy_test.go — ADR-0076 slice D policy knob: the
// failure-count threshold at which a retried item's build escalates to deep.
// Compiled default 1 (first retry escalates); positive-override merge (the
// TaskRetryCeiling idiom — 0 keeps the default, documented convention).
```

### `go/internal/policy/retry_policy_for_test.go:3` — above `import "testing"`

```text
// retry_policy_for_test.go — the accessor that finally consumes ADR-0072's
// declarative retry policy. Action, MaxRetries and FixType have been declared in
// the category table since the policy shipped and were read by nothing; the retry
// decision used a parallel knob instead. This pins the table as the one authority.
```

### `go/internal/policy/routing_gates.go:72` — above `ReportSizeGate string 'json:"report_size_gate,omitempty"'`

```text
// ReportSizeGate is the report-size (handoff-summary token budget) gate's own
// rollout dial (cycle-565 Slice S1). Unlike the other gates it defaults to
// "shadow", not "enforce": the inbox spec calls for shadow/warn BEFORE
// enforce so the budget is observed before it can block a cycle.
```

### `go/internal/policy/routing_gates.go:77` — above `ManifestGate string 'json:"manifest_gate,omitempty"'`

```text
// ManifestGate is the ship-bind tree-manifest reconciliation gate's rollout
// dial (internal/phases/ship/manifest.go, cycle-1064). Like ReportSizeGate it
// defaults to "shadow", not "enforce": out-of-manifest paths (the cross-lane
// untracked-leak shape) are LOGGED before the gate is allowed to block a
// cycle. "enforce" fails the ship closed with core.CodeManifestGate.
```

### `go/internal/policy/routing_gates.go:84` — above `RepoContractGate string 'json:"repo_contract_gate,omitempty"'`

```text
// RepoContractGate is the ship-time repo-contract scanner pack's dial
// (internal/phases/ship/repocontract.go). Unlike ManifestGate it defaults
// to "enforce": the scanners are the deterministic repo-wide guard suites
// (phasespec, profiles, phasecoherence, routingtest) whose breakage IS a
// red main — four lane landings redded main in the week of 2026-08-04
// (artifact-bytes, phase metadata, profile stubs, incident-postmortem),
// each a CI-email storm; FP≈0 because a failing scanner here fails on
// main's next run by construction. "off" disables.
```

### `go/internal/policy/routing_gates.go:116` — above `ManifestGate:   "shadow",`

```text
// shadow-first, mirroring ReportSizeGate (cycle-1064)
```

### `go/internal/policy/routing_gates.go:117` — above `RepoContractGate: "enforce",`

```text
// enforce-default deviation from shadow-first, justified: the pack is
// existing deterministic repo tests (FP≈0), and every failure mode it
// gates was a LIVE red-main incident in the preceding week.
```

### `go/internal/policy/routing_gates.go:152`

```text
// ReportBudgetPolicy is the .evolve/policy.json "report_budget" block: the
// per-artifact token budgets the report-size gate enforces (cycle-565 Slice S1).
// Separate from GatesPolicy so the budget VALUE and the gate STAGE are dialed
// independently. Absent ⇒ built-in defaults apply.
```

### `go/internal/policy/scan_fast_tier_envelope_test.go:11` — above `var inScopeScanProfiles = []string{`

```text
// scan_fast_tier_envelope_test.go — durable regression contract for cycle-980
// Task `scan-phase-fast-tier-envelopes` (inbox weight 0.94). The 12 in-scope
// judgment-light scan profiles must carry an explicit model_tier_envelope of
// {min:"fast", max:"balanced"} so their handoff work can run at the fast tier
// while capping escalation at balanced (overriding the universal {balanced,deep}
// floor). `secret-leak-scan`/`flake-rerun-scan` are OUT of scope (owned by
// mechanical-scans-to-native) and must be left on their existing floor.
//
// This is the PERMANENT sibling of the cycle-scoped ACS predicates in
// go/acs/cycle980 (AC3: "New Go regression test in internal/policy"). It reads
// the SHIPPED profiles via the same locator the memo/driver-agnostic tests use
// (filepath.Join("..","..","..",".evolve","profiles")) and exercises the live
// policy.ValidatePin resolver, so it pins the on-disk contract beyond this cycle.
//
// RED today: TestScanProfiles_CarryFastTierEnvelope and
// TestScanProfiles_EnvelopeClampsDeepPin fail because every in-scope profile's
// ModelTierEnvelope is nil. GREEN once the 12 profiles carry {fast,balanced}.
```

### `go/internal/policy/size_budget_policy_test.go:3` — above `import "testing"`

```text
// size_budget_policy_test.go — ADR-0076 slice A: cycle-size → budget
// multipliers (compiled defaults, per-key positive-override merge — the
// PhaseArtifactTimeouts idiom). Consumed by the correction-limit and build
// artifact-timeout scaling.
```

### `go/internal/policy/strict_audit_test.go:9` — above `func TestStrictAudit_DefaultAndOverride(t *testing.T) {`

```text
// TestStrictAudit_DefaultAndOverride locks the policy.json "workflow.strict_audit"
// field that replaces the EVOLVE_STRICT_AUDIT env read (flag-reduction, ADR-0064).
// Absent ⇒ false (fluent-by-default: ship on WARN, failure-adapter awareness-only);
// a present true flows the operator's strict (legacy-blocking) posture through.
```

### `go/internal/policy/workflow.go:20` — above `StrictAudit bool 'json:"strict_audit,omitempty"'`

```text
// StrictAudit selects the strict (legacy-blocking) audit posture. Absent/false
// = fluent-by-default (ship on a WARN audit verdict; the failure-adapter is
// awareness-only on recurring failures). True restores legacy blocking: WARN is
// promoted to FAIL in both the audit phase and the ship audit-binding, and the
// failure-adapter's first matching rule BLOCKs. Replaces the EVOLVE_STRICT_AUDIT
// env read (flag-reduction, ADR-0064). A plain bool (not *bool): false is the
// product default, so an absent block and an explicit false are the same posture.
```

### `go/internal/policy/workflow.go:40` — above `SizeBudgetMultipliers map[string]float64 'json:"size_budget_multipliers,omitempty"'`

```text
// SizeBudgetMultipliers scales per-cycle budgets (correction rounds, build
// artifact timeout) by the triage/scout cycle_size_estimate (ADR-0076 A).
// Per-key positive override; unmentioned keys keep compiled defaults.
```

### `go/internal/policy/workflow.go:68` — above `BuildFloorEnforced bool`

```text
// BuildFloorEnforced (default true): the build deliverable is REJECTED
// while the changed packages' deterministic self-check fails (shift-left
// half of the 2026-07-21 directive) — the E2 correction ladder then fixes
// it in-phase. false restores the advisory-only selfcheck.
```

### `go/internal/policy/workflow.go:73` — above `SizeBudgetMultipliers map[string]float64`

```text
// SizeBudgetMultipliers maps cycle_size_estimate → budget multiplier
// (ADR-0076 A). Compiled defaults: trivial/small 1.0, medium 1.25,
// large 1.5. A survivorship-hard backlog starves the verification tail
// under uniform budgets (batch-8: ~10-minute build windows incl.
// corrections on structural items).
```

### `go/internal/policy/workflow.go:79` — above `RemediationRounds int`

```text
// RemediationRounds bounds the graduated fix-forward ladder (operator
// directive 2026-07-21): when a phase listed in RemediablePhases returns a
// FAIL verdict, the orchestrator re-dispatches the builder ONCE per round
// with the gate's report as a correction directive, then re-runs the SAME
// gate. 0 disables. Default 1.
```

### `go/internal/policy/workflow.go:96` — above `UniversalFallbackExclude []string`

```text
// UniversalFallbackExclude (default ["agy"]): families the last-resort tail
// never contains — the 2026-06-07 operator judgment that gemini-3.5-flash is
// error-prone, and fallbacks fire exactly when things are already going
// wrong. A banned family may still be a profile's configured primary.
// workflow.universal_fallback_exclude=[] lifts it.
```

### `go/internal/policy/workflow.go:122` — above `RemediationRounds:     1,`

```text
// Graduated remediation (2026-07-21): default ON at 1 round for the
// coverage gate — the measured waste class (983/992/1007/1019/1020).
```

### `go/internal/policy/worktree_config_param_test.go:5` — above `func TestWorktreeBase_DefaultAndOverride(t *testing.T) {`

```text
// TestWorktreeBase_DefaultAndOverride locks the policy.json "worktree" block
// that replaces the EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
// Absent block ⇒ "" (caller applies its built-in <root>/.evolve/worktrees
// default); a present block flows the operator override through.
```
