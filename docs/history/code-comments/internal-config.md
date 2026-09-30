# Comment history: `internal/config`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/config/apicover_named_test.go:84` — above `func TestModelRouting_ParsesAndStringifies(t *testing.T) {`

```text
// TestModelRouting_ParsesAndStringifies binds the cycle-436 ModelRouting axis
// (type + its three consts + String) to its real producer, Load, exercising
// all three values through the registry's model_routing key and asserting the
// human-readable String() form each renders to.
```

### `go/internal/config/commit_evidence_test.go:5` — above `func TestCommitEvidence_Default(t *testing.T) {`

```text
// commit_evidence_test.go — EVOLVE_COMMIT_EVIDENCE (ADR-0027) flag parsing.
// Default off (byte-identical legacy path-poll); shadow/enforce recognized;
// "advisory" and typos default to off with a warning (a typo must never
// silently enable phase commits).
```

### `go/internal/config/compact_prompts_test.go:8` — above `func TestCompactPrompts_DefaultTrue_WhenKeyAbsent(t *testing.T) {`

```text
// compact_prompts_test.go — Adversarial amplification for CompactPrompts config behavior.
//
// Probes the config pipeline for the cycle-413 Task B CompactPrompts field:
// - Default (true) when compact_prompts absent from registry
// - Explicit false overrides the default
// - Whole workflow block absent → default still applies
// - Registry entirely absent → default applies (from defaults())
//
// These are distinct from ACS C413_004/C413_005 which test field-presence
// and the explicit-true case. These tests target the silent-false regression
// path: a field that defaults true can silently become false if parse errors
// or missing keys are not handled correctly.
```

### `go/internal/config/condrule.go:13` — above `const DefaultTddRuleExpr = "cycle_size!=trivial && deliverable_kind!=document"`

```text
// DefaultTddRuleExpr is the ONE expression of the compiled tdd conditional —
// the same words docs/architecture/phase-registry.json:config.conditional_mandatory
// carries (TestDefaults_TddRuleMatchesRegistry pins them equal). tdd is the
// code-specific phase: pinned for every non-trivial CODE cycle, released for a
// trivial cycle or a document deliverable (ADR-0099). Registry-less projects
// (no docs/architecture/phase-registry.json, or EVOLVE_USE_PHASE_REGISTRY=0)
// run on this default, so it must never lag the registry.
```

### `go/internal/config/condrule.go:37` — above `type CondRule struct {`

```text
// CondRule is a parsed conditional-mandatory predicate, e.g. cycle_size != trivial.
// A rule may AND further clauses (`a!=b && c!=d`, ADR-0099): the head clause
// keeps the single-clause shape (And nil) so every existing consumer reads it
// unchanged; the extra clauses ride in And and the rule holds only when ALL
// clauses hold.
```

### `go/internal/config/condrule.go:58` — above `func parseCondRule(expr string) (CondRule, error) {`

```text
// parseCondRule parses one or more `&&`-joined clauses of the form
// "field<op>value" (op one of != == >= <= > <). The first clause is the head;
// the rest land in And (ADR-0099). Any malformed clause fails the WHOLE rule —
// a rule can never silently shrink to fewer conditions than the operator wrote.
```

### `go/internal/config/condrule_and_test.go:9` — above `func TestParseCondRule_AndClauses(t *testing.T) {`

```text
// TestParseCondRule_AndClauses — ADR-0099: a conditional-mandatory rule may
// AND several clauses (`a!=b && c!=d`). The head clause keeps the legacy
// single-clause shape byte-identical (And nil) so every existing consumer is
// untouched; the extra clauses ride in And. A malformed clause fails the whole
// rule (never a silently-shorter rule).
```

### `go/internal/config/condrule_and_test.go:72` — above `func TestDefaults_TddRuleMatchesRegistry(t *testing.T) {`

```text
// TestDefaults_TddRuleMatchesRegistry pins the compiled default (the rule
// registry-less projects run on) to the registry's parsed rule — one belief,
// two homes, bound by a test since the ADR-0099 release lived only in the
// registry for one review round.
```

### `go/internal/config/config.go:1` — above `package config`

```text
// Package config is the single composition-root loader for evolve-loop's
// dynamic-routing configuration: the ONLY place that reads the routing env
// vars and the central phase registry. Every downstream consumer receives the
// resolved RoutingConfig by injection and never calls os.Getenv.
//
// The Loader (ADR-0103 unit 08) resolves it in two steps. Load: the compiled
// defaults → docs/architecture/phase-registry.json → the contained env
// overrides → the spine and inert-enable validators (precedence env >
// registry > default). ApplyPolicyStages: the composition root's projection
// of .evolve/policy.json's gate, recovery, router and parallel-evaluate dials
// over that value. Both report every non-fatal diagnostic as a config.warning
// WARN under module config through the injected Signal Center; the
// package-level Load facade is the Center-less loader the per-phase callers
// keep (same value, same Warning slice, nothing emitted).
//
// Leaf package by design: stdlib plus internal/signalcenter and
// internal/paths (importgraph_test.go pins the allowlist). It must never
// import internal/core or internal/policy — both import it. Phase identifiers
// cross the boundary as plain strings; core converts to/from core.Phase at the
// call site. Design: docs/architecture/decomposition/08-config.md.
```

### `go/internal/config/config.go:132` — above `func defaults() RoutingConfig {`

```text
// defaults is the compiled baseline every resolution starts from. Dynamic
// routing is DEFAULT-ON (Component #7): the advisor drives phase selection
// every cycle, with the integrity floor (ClampPlanToFloor + SpineSatisfiedUpTo)
// — not a flag — protecting the ship guarantee. EVOLVE_DYNAMIC_ROUTING still
// overrides (e.g. =off for the legacy static path). Flipped from StageOff
// after the advisory mode soaked since cycle-108. CompactPrompts defaults ON
// (strips ~23 KB/cycle of on-demand reference tails); the concurrency and
// re-plan bounds are the soak sweet spots their field docs record.
```

### `go/internal/config/config.go:157` — above `func defaultRollout() RolloutStages {`

```text
// defaultRollout is the compiled rollout-dial baseline. SpineFloor=StageEnforce
// (the R8.5 flip, landed 2026-07-16 as its OWN dial): the artifact-backed
// spine floor ABORTS on a clean-absence handoff gap instead of
// WARN-and-proceed. Evidence: after the scout/audit digest fallbacks
// (router/digest.go), a 536-run-dir replay showed 0 would-block transitions
// on every cycle shape since ~cycle-480 (the only misses were pre-convention
// dirs from the 361-479 era). Degraded reads stay fail-open (the cleanAbsence
// guard in cyclerun_select.go), an enforce block records to failure-learning,
// and policy.json `recovery.spine_floor: "shadow"` is the no-recompile escape
// hatch. Deliberately DECOUPLED from PhaseRecovery: that dial is overloaded
// (ADR-0045 I6 folded the bidirectional channel into it, and the
// failure-adviser promotion path also keys on it), so arming the floor via
// PhaseRecovery would have armed two unsoaked subsystems. PhaseRecovery itself
// stays shadow. FatalPane=StageEnforce (the F27 flip, 2026-09-26, split out
// the same way): the stop-review checkpoint's fatal-pane fast-fail ACTS —
// every shadow match on record was a dead pane that then idled 900-1200s;
// `recovery.fatal_pane: "shadow"` is its escape hatch. The gate dials
// (EvalGate, ContractGate, TriageCapGate, TopNGate, PhaseRecovery,
// SpineFloor, FatalPane, RouterReplan, ParallelEvaluate) are
// re-resolved from policy's compiled accessors by ApplyPolicyStages at the
// composition root — a two-source belief TestDefaults_GateDialsMatchPolicyCompiledDefaults
// pins equal (F6 in the unit doc unifies them).
```

### `go/internal/config/config.go:198` — above `func defaultMandatory() []string { return []string{"scout", "build", "audit", "ship"} }`

```text
// defaultMandatory is the compiled mandatory spine. NOTE: this built-in
// baseline intentionally omits triage; the real registry
// (docs/architecture/phase-registry.json) adds it via applyRegistry (cycles
// 263/264: the advisory router skipped the scope-clamp). Tests constructing
// RoutingConfig directly keep this 4-phase baseline.
//
// Mandatory DELIBERATELY diverges from phasecontract.RequiredRoles() —
// audited cycle-1141, kept separate on purpose. The two lists answer
// different questions and must not be merged:
//   - phasecontract.RequiredRoles()/RequiredArtifacts() = "what must a
//     COMPLETED cycle have PRODUCED" (report-bearing spine phases;
//     consumed by cyclehealth/redteamcheck/ledgerverify).
//   - this list = "what must the ROUTER always PLAN" — a superset that
//     includes "ship", a phase that writes no report and therefore
//     cannot appear in an artifact-derived registry vocabulary.
//
// Deriving this from the registry would silently drop "ship" from every
// plan; hardcoding the registry's half is the drift risk. The invariant
// that actually binds them — Mandatory ⊇ registry-required phases, plus
// "ship", and nothing the registry does not know — is asserted by the
// cycle-1141 ACS predicate rather than by shared code. That predicate reads
// THIS file by path (acs/cycle1141), so the defaults family stays in
// config.go.
```

### `go/internal/config/config_legacyflags_test.go:5` — above `func TestLoad_LegacyPhaseFlags(t *testing.T) {`

```text
// Locks the default per-phase enable values from defaults() when no env overrides apply.
// Env-based overrides (EVOLVE_TRIAGE_DISABLE, EVOLVE_TEST_PHASE_ENABLED, EVOLVE_BUILD_PLANNER)
// were removed in cycle-39: phase enables are now configured via WorkflowPolicy.PhaseEnables.
```

### `go/internal/config/config_realregistry_test.go:15` — above `if cfg.Stage != StageAdvisory {`

```text
// Default posture since 2026-06-06 (retro migration steps 1-3 landed):
// dynamic_routing=advisory ⇒ the advisor drives the optional surface while
// the spine stays static and ClampPlanToFloor protects the ship guarantee.
// EVOLVE_DYNAMIC_ROUTING=off remains the operator escape hatch.
```

### `go/internal/config/config_realregistry_test.go:25` — above `wantSpine := []string{"scout", "triage", "build", "audit", "ship"}`

```text
// triage joined the mandatory spine 2026-06-10 (cycles 263/264 post-mortem):
// the advisory router skipped triage on the premise "scout picks ONE item"
// while the scout authored THREE tasks — with the scope-clamp gone, the
// builder under-delivered and the all-or-nothing audit failed the cycle.
// Dispatch-level mandatory restores the pre-advisory equilibrium; triage's
// own runner-level auto-skip (EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL) still
// short-circuits genuinely trivial cycles, so the router may not remove
// the clamp but the clamp stays cheap.
```

### `go/internal/config/config_realregistry_test.go:42` — above `if cfg.MaxInsertions != 6 {`

```text
// 6 since cycle 217 (micro-phase catalog §4.2): the refactor recipe needs
// six optional insertions; registry config.max_optional_insertions raised 4→6.
```

### `go/internal/config/config_test.go:216` — above `if cfg, _ := Load(absent, map[string]string{}); cfg.PhaseRecovery != StageShadow {`

```text
// Default (no env): SHADOW — ADR-0044's behavior-neutral first ship. Note
// the R8.5 spine-floor flip deliberately did NOT move this dial: it is
// overloaded (bidirectional channel + failure-adviser promotion), so the
// floor got its own SpineFloor dial instead (TestLoad_SpineFloorStage).
```

### `go/internal/config/config_test.go:223` — above `for _, v := range []string{"off", "0", "shadow", "enforce", "banana"} {`

```text
// EVOLVE_PHASE_RECOVERY is retired (cycle-12 flag retirement). The env var
// is ignored by applyEnv; the dial is now policy-driven (policy.RecoveryConfig).
// Passing the env var has no effect — PhaseRecovery stays at the default StageShadow.
```

### `go/internal/config/config_test.go:252` — above `func TestLoad_FatalPaneStage(t *testing.T) {`

```text
// TestLoad_FatalPaneStage pins the F27 flip the same way: the ADR-0044 C2
// fatal-pane fast-fail defaults ENFORCE on its OWN dial, with no env-var
// override (policy-only: `recovery.fatal_pane`) — neither a would-be
// EVOLVE_FATAL_PANE nor the program dial's old env name moves it.
```

### `go/internal/config/config_test.go:271` — above `func TestPhaseIOStage(t *testing.T) {`

```text
// TestPhaseIOStage pins the EVOLVE_PHASE_IO dial (ADR-0050 Phase 3): the unified
// phase I/O rollout uses the FULL off→shadow→advisory→enforce ladder (4-value,
// unlike the 3-value gate dials), defaults ENFORCE as of the 3.10 cutover (the
// typed envelope is now authoritative; set EVOLVE_PHASE_IO=off to roll back), and
// a typo falls back to off with a warning (never silently leaving the dial in an
// unintended state). Covers DefaultEnforce / Off / Shadow / Advisory / Enforce / TypoDefaultsOff.
```

### `go/internal/config/defaults_parity_test.go:3` — above `import (`

```text
// defaults_parity_test.go — the two-source gate defaults (config.defaults()
// vs policy's compiled accessors, applied unconditionally over config's at the
// root) are a replicated belief acs/cycle34 records in prose; this is the
// consumer pin that turns it into a tested one (ADR-0103 unit 08 test 36; F6
// unifies them once the operator picks the winner). External test package so
// it can import policy (policy imports config).
```

### `go/internal/config/defaults_parity_test.go:30` — above `func TestPolicyStages_KeysMatchPolicyJSONTags(t *testing.T) {`

```text
// TestPolicyStages_KeysMatchPolicyJSONTags — the nine policy keys
// ApplyPolicyStages names in CONFIG_UNKNOWN_VALUE (fields.key and the message)
// are a SECOND home of policy.json's spellings; this is their consumer pin:
// each must equal <section tag on policy.Policy>.<field tag on the block>
// (architecture-review fold, ADR-0103 unit 08). A policy rename that leaves
// config's label behind would otherwise point a triage at a key that no
// longer exists.
```

### `go/internal/config/deliverable_kinds_test.go:10` — above `func TestLoad_DeliverableKinds_FromRegistry(t *testing.T) {`

```text
// TestLoad_DeliverableKinds_FromRegistry — ADR-0099 slice 2: the document
// deliverable contract is CONFIG (phase-registry.json:config.deliverable_kinds),
// consumed by ONE deterministic engine. Pins the shipped registry's shape.
```

### `go/internal/config/deliverable_kinds_test.go:109` — above `func TestSignalKeys_AreTheConditionalRuleWords(t *testing.T) {`

```text
// TestSignalKeys_AreTheConditionalRuleWords — ADR-0099 slice 3: an overlay
// `when` clause, core's dispatch projection and the compiled tdd rule name the
// deliverable-kind signal with ONE word, and the goal type is the scout's
// namespaced routable field.
```

### `go/internal/config/domain.go:14` — above `type Domain struct {`

```text
// Domain is the project-level adapter record .evolve/domain.json has carried
// since v8 (docs/reference/configuration.md) with ZERO Go readers until
// ADR-0099 slice 2. Only Domain is consumed today — it yields the project's
// default deliverable kind when a task declares none; the other fields are the
// legacy prompt-layer vocabulary, parsed for round-trip fidelity only.
```

### `go/internal/config/env.go:61` — above `func envPhaseIO(cfg *RoutingConfig, v string, ws *[]Warning) {`

```text
// envPhaseIO is the ADR-0050 Phase 3 unified phase-I/O rollout dial. Reuses
// parseStage (the 4-value off→shadow→advisory→enforce ladder) so a typo falls
// back to off (fail-safe), never leaving the dial in an unintended state.
// Default (no env) is enforce as of the 3.10 cutover, set in defaults(); set
// EVOLVE_PHASE_IO=off to roll back.
```

### `go/internal/config/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is the routing-config leaf (ADR-0103 unit 08
// §2): stdlib plus the two named internal packages — never internal/core,
// never internal/policy (policy imports config: the compiler is the cycle
// guard; this is the leaf-ness declaration — signalcenter/importgraph_test.go idiom).
```

### `go/internal/config/inert_enable_test.go:17` — above `func TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning(t *testing.T) {`

```text
// TestLoad_PlanReviewEnabled_StageAdvisory_NoInertWarning proves the inert
// warning is SCOPED to Stage<Advisory. At Stage>=Advisory the router consults
// PhaseEnable, so a non-spine phase enable is not inert.
// (The env-based EVOLVE_PLAN_REVIEW trigger was removed in cycle-39; this
// baseline checks that no spurious warning fires with empty env.)
```

### `go/internal/config/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 08 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/config/loader_pins_test.go:3` — above `import (`

```text
// loader_pins_test.go — ADR-0103 unit 08 step 1: the pre-move pins and the
// goldens captured on 8e8f080f before any code moved. Every test here was
// green on the unsplit config.go and proven red against its named mutant;
// they replay byte-for-byte through the Loader after the split.
```

### `go/internal/config/loader_pins_test.go:178` — above `func TestLoad_RealRegistry_MatchesTheGolden(t *testing.T) {`

```text
// Test 8 — golden G1: the resolved value for the FROZEN registry copy
// (testdata/frozen_registry.json, the live file at 8e8f080f) is byte-identical
// to the golden captured before the split. The live file is guarded by the
// property-style TestLoad_RealRegistry; cycles edit it, so it is never golden'd.
```

### `go/internal/config/loader_test.go:3` — above `import (`

```text
// loader_test.go — ADR-0103 unit 08 step 2: the Loader with its injected
// reader and Center accessor, the range-stamped triage fields, the two registry
// codes that close the silent kill-path, and the stream contract.
```

### `go/internal/config/model_routing_default_test.go:49` — above `func TestCheckedInPolicyDefaultsModelRoutingAuto(t *testing.T) {`

```text
// TestCheckedInPolicyDefaultsModelRoutingAuto (mr4d AC3): loading the repo's
// OWN checked-in docs/architecture/phase-registry.json — the file
// config.Load actually reads at every real call site (cmd_cycle.go,
// phase_verify.go, router/policy.go all build registryPath from
// "docs/architecture/phase-registry.json", never from .evolve/policy.json)
// — must resolve to ModelRoutingAuto once Task C lands.
//
// NOTE (TDD-engineer, cycle-440): the scout report / api-contract / eval
// mr4d-default-model-routing-auto.md all say the default flip belongs in
// ".evolve/policy.json". Reading the actual producer (config.go's
// registryDoc + every registryPath call site) shows model_routing is parsed
// EXCLUSIVELY from docs/architecture/phase-registry.json's `config.model_routing`
// key; .evolve/policy.json is a separate file (policy.Load) that never feeds
// RoutingConfig.ModelRouting. This test targets the file the code actually
// reads (Rule 8: read first, don't invent an API from context) — see
// test-report.md for the full discrepancy note to Builder/Auditor.
```

### `go/internal/config/parse.go:87` — above `func parseModelRouting(v, varName string, ws *[]Warning) ModelRouting {`

```text
// parseModelRouting parses the cycle-436 model-authority axis. Unknown values
// fall back to the SAFE static side (never silently enable auto) with a
// warning — mirroring parseStage/parseMode's fail-safe-with-warning contract.
// varName names the offending key in the warning.
```

### `go/internal/config/rollout.go:12` — above `CommitEvidence Stage`

```text
// CommitEvidence is the ADR-0027 commit-as-evidence rollout stage:
// StageOff (legacy path-poll, byte-identical), StageShadow (git-evidence
// computed + logged, artifact authoritative), StageEnforce (git-evidence
// authoritative, phases commit, kernel relaxed). StageAdvisory is not used
// for this axis. The bridge driver reads EVOLVE_COMMIT_EVIDENCE from env
// directly (it is a subprocess); this field is the orchestrator's view.
```

### `go/internal/config/rollout.go:38` — above `ContractGate Stage`

```text
// ContractGate is the deliverable-contract gate rollout stage
// (internal/deliverable, ADR-0034): StageOff — no contract gate
// (orchestrator keeps the noopReviewer; byte-identical); StageShadow —
// the verifier runs but every violation is log-only; StageEnforce — a
// confirmed well-formedness violation (missing/misplaced/malformed
// deliverable) rejects the phase. The gate fails OPEN on ambiguity, and a
// runtime circuit breaker demotes enforce→advisory after N consecutive
// blocks so a miscalibrated gate cannot brick the loop. Configured through
// policy.GatesConfig; default StageEnforce.
```

### `go/internal/config/rollout.go:66` — above `SandboxMode string`

```text
// SandboxMode controls OS-level sandbox wrapping for source-writing phases
// (Workstream B — cycle-119 cross-CLI trust bypass). Values:
//   "auto" (default) — wrap when nested-claude is NOT detected and the
//                       host's sandbox binary (sandbox-exec / bwrap) is
//                       present; degrade unwrapped otherwise.
//   "on"             — always wrap when the binary is available; WARN
//                       loudly (no fallback) when it isn't.
//   "off"            — never wrap. Operator-only emergency hatch; the
//                       trust kernel is then Claude-PreToolUse-only.
//
// PRECEDENCE NOTE: the bridge subprocess reads EVOLVE_SANDBOX from its
// own env chain (deps.Env / os.Getenv), which is the actual signal. This
// field is the COMPOSITION-ROOT view — set from the same env var by
// applyEnv so operators auditing the loaded config can see the effective
// mode. Mirrors the CommitEvidence pattern (also env-direct on the
// subprocess hot path). Setting this field in code without also propagating
// EVOLVE_SANDBOX into the bridge's env map has no effect.
```

### `go/internal/config/rollout.go:84` — above `PhaseRecovery Stage`

```text
// PhaseRecovery is the ADR-0044 Unified Phase Recovery program dial. It
// gates the live bidirectional channel (channel.Enabled, ADR-0045 I6),
// the ask-broker, the transient-dwell fast-fail, the observer's
// chain-backed StallPolicy, the orchestrator's failure-adviser promotion
// and the misplaced-deliverable salvage. Two behaviors were SPLIT OUT onto
// their own dials so they could act without arming the rest: SpineFloor
// (R8.5) and FatalPane (F27) — never flip this dial to get either.
//   StageOff     — recovery components inert; byte-identical legacy.
//   StageShadow  — classify + log would-be actions only (DEFAULT).
//   StageEnforce — the corrective actions above execute. Classification
//                  is always-on above off; only ACTING is staged.
// Resolution: policy `recovery.phase_recovery` through ApplyPolicyStages;
// the composition root forwards it to the bridge (wireBridgeStages →
// Deps.RecoveryStage) and the observer (its IPC stage key) — no process
// reads an EVOLVE_PHASE_RECOVERY env var any more. Default StageShadow per
// ADR-0044 (behavior-neutral first ship).
```

### `go/internal/config/rollout.go:102` — above `SpineFloor Stage`

```text
// SpineFloor is the artifact-backed spine floor's OWN rollout dial (the
// R8.5 flip, 2026-07-16). Split out of PhaseRecovery because that dial is
// overloaded — ADR-0045 I6 folded the bidirectional channel into it
// (channel.Enabled: enforce → live producer + per-tick capture) and the
// failure-adviser promotion hook also keys on it — so arming the floor
// through PhaseRecovery would have armed two unsoaked subsystems along
// with it. This dial gates EXACTLY ONE behavior: whether a clean-absence
// mandatory-predecessor handoff gap ABORTS the cycle (StageEnforce, the
// default) or WARN-and-proceeds (StageShadow). Degraded reads fail open at
// every stage. Policy override: `recovery.spine_floor` (no env var).
```

### `go/internal/config/rollout.go:114` — above `FatalPane Stage`

```text
// FatalPane is the ADR-0044 C2 fatal-pane fast-fail's OWN rollout dial
// (F27, 2026-09-26), split out of PhaseRecovery for the SpineFloor reason:
// that dial also arms the live channel (channel.Enabled), the ask-broker
// and the failure-adviser promotion, so flipping it for the fast-fail
// would arm three unsoaked subsystems. This dial gates EXACTLY ONE
// behavior — whether a persisted, non-Busy fatal-pane match at the
// stop-review checkpoint ends the wait in one interval (StageEnforce, the
// default) or only records would_fast_fail and lets the reviewer decide
// (StageShadow). Soak evidence for the flip: every shadow match
// (cycles 1595 build, 1687 triage — trigger dead_shell) was a dead pane
// that then idled 1200s / 900s. Policy override: `recovery.fatal_pane`
// (no env var).
```

### `go/internal/config/rollout.go:128` — above `PhaseIO Stage`

```text
// PhaseIO is the ADR-0050 Phase-3 unified-phase-I/O rollout dial. Unlike the
// gate dials above (off/shadow/enforce trichotomy) it uses the FULL
// off→shadow→advisory→enforce ladder, like the dynamic-routing Stage:
//   StageOff      — the unified phaseio envelope is dormant; byte-identical
//                   legacy dispatch (the rollback escape hatch).
//   StageShadow   — the envelope is assembled and compared against the
//                   legacy disk reads; mismatches log + ledger only.
//   StageAdvisory — the envelope is populated and read alongside the legacy
//                   path (legacy still wins; the two are compared).
//   StageEnforce  — the typed envelope is authoritative.
// Default StageEnforce as of the 3.10 cutover (set in defaults()); set
// EVOLVE_PHASE_IO=off to roll back. A typo falls back to off via parseStage
// (fail-safe — never leaves the dial in an unintended state).
```

### `go/internal/config/rollout.go:143` — above `RouterReplan Stage`

```text
// RouterReplan is the ADR-0052 advisor-maximization post-scout re-plan
// rollout dial (WS2). It uses the off→shadow→advisory subset of the Stage
// ladder (enforce is unused for this axis):
//   StageOff      — no post-scout re-plan; the upfront plan stands.
//   StageShadow   — the re-plan is computed + logged (replan-plan.json), but
//                   the upfront clamped plan still drives (DEFAULT).
//   StageAdvisory — the re-plan replaces the clamped plan after the floor
//                   re-clamps it (opt-in, post-soak).
// PRECEDENCE NOTE: like the other rollout dials this is the composition-root
// view, loaded from policy; the re-plan call site (WS2-S3) reads it.
// Default StageShadow (set in defaults()).
```

### `go/internal/config/routing_config.go:7` — above `type DeliverableKindSpec struct {`

```text
// DeliverableKindSpec is the shape a cycle's deliverable must take for one
// deliverable kind (ADR-0099 slice 2) — declared ONCE in
// phase-registry.json:config.deliverable_kinds and judged by ONE engine
// (internal/solutioncheck) that the build floor, the `evolve solution check`
// CLI and the audit gate all project. Paths are relative to the kind's Root
// directory under the repo, e.g. solutions/<slug>/recommendation.md.
```

### `go/internal/config/routing_config.go:59` — above `ModelRouting ModelRouting`

```text
// ModelRouting is the cycle-436 model-authority axis (static|advisory|
// auto), registry-driven via docs/architecture/phase-registry.json
// `config.model_routing` (no env dial, no policy.json key). Zero value
// ModelRoutingStatic — byte-identical default for every operator who never
// sets model_routing.
```

### `go/internal/config/routing_config.go:71` — above `DeliverableKinds map[string]DeliverableKindSpec`

```text
// DeliverableKinds is the per-kind deliverable contract (ADR-0099 slice 2),
// registry-sourced; absent ⇒ no document contract is enforced.
```

### `go/internal/config/routing_config.go:107` — above `GoalRecipes map[string][]string`

```text
// GoalRecipes is the ADR-0052 WS5 recipe SSOT (config.goal_recipes in the
// registry): goal type → ordered, verbatim recipe-token list. It is the
// single source projected into the persona's "## Goal-Type Recipes" table
// (via router.RenderRecipeProjection, locked by TestRouterPersonaRecipeTable_NoDrift)
// and read by the RecipeVerifier — ending the prior three-source recipe drift.
// Registry-sourced only; nil when no registry supplies it (projection renders empty).
```

### `go/internal/config/routing_config.go:114` — above `RoutingJudge bool`

```text
// RoutingJudge is the ADR-0052 advisor-maximization WS4 route-quality judge
// toggle. false (DEFAULT) — no judge call,
// byte-identical. true — the fast-tier LLM-as-judge scores the emitted route
// for forensics, strictly off the build path (never gates ship, never alters
// the plan). It is a plain bool, NOT a Stage: the judge cannot move behavior,
// so the off→shadow→advisory ladder would be meaningless (shadow≡advisory≡on).
// Composition-root view loaded from policy; the scoring call site reads it.
```

### `go/internal/config/routing_config.go:122` — above `ReconDigest bool`

```text
// ReconDigest is the ADR-0052 advisor-maximization WS2-S0b toggle for the
// deterministic pre-plan recon digest. false
// (DEFAULT) — the initial Plan prompt is byte-identical to pre-slice. true —
// the advisor renders measured repo facts (langs/tests/hotspots, goal-keyword
// hits, backlog/carryover) under "## Pre-plan recon (deterministic)" so
// upfront selection is grounded in evidence, not goal-text inference alone.
// A plain bool (not a Stage): it injects deterministic facts the floor still
// clamps, so there is no shadow/advisory distinction. Composition-root view
// loaded from policy; composePlanPrompt reads it.
```

### `go/internal/config/routing_config.go:132` — above `RePlanMaxDepth int`

```text
// RePlanMaxDepth caps how many post-scout re-plans a single cycle may run
// (ADR-0052 WS2-S5; research P4 — cap depth, escalate not loop). Default 1
// (set in defaults()): one measured re-plan per cycle, then escalate to the
// debugger rather than thrash. A cycle-scoped counter cr.replanDepth tracks
// the live depth. A non-positive policy value falls back to 1.
```

### `go/internal/config/signals.go:80` — above `func (l *Loader) emit(origin string, ws []Warning) {`

```text
// emit is the ONE producer: each Warning becomes a config.warning WARN under
// module config, Cycle 0 (the loader is batch-level: it runs before any cycle
// number exists, so the root's durable sink files it under <evolveDir>),
// Origin naming the exported method, the Message as the reason and the
// Warning's fields verbatim. A nil Center is the Null Object.
```

### `go/internal/config/spine_order_validation_test.go:5` — above `func TestValidateSpine_ShipBeforeAuditWarns(t *testing.T) {`

```text
// TestValidateSpine_ShipBeforeAuditWarns covers the ADR-0058 S6 spine-order guard:
// the config-driven floor positions anchors by their configured order, so a
// scrambled order placing ship before audit must surface a loud spine-order
// warning (the legality graph + audit verdict branch still block it, but the
// misordering must never go unnoticed). A sane order raises no such warning.
```

### `go/internal/config/validate.go:34` — above `func validateInertEnables(cfg RoutingConfig, ws *[]Warning) {`

```text
// validateInertEnables warns when PhaseEnable[p]=EnableOn but p is neither
// mandatory, in the static spine, nor reachable via the router (Stage<Advisory).
// The classic trigger is plan-review enabled via policy.json with default routing:
// plan-review only runs at Stage>=Advisory, so the enable is silently inert at
// Stage=Off AND at Stage=Shadow (per the Stage docstring, shadow computes+logs but
// the STATIC state machine still drives execution — so non-spine phases remain
// unreachable). Surfacing this prevents the operator-confusion failure mode
// from cycle 120.
```

### `go/internal/config/vocabulary.go:3` — above `type Stage int`

```text
// vocabulary.go — the closed routing vocabularies every consumer reads: the
// rollout Stage ladder, the routing Mode, the model-authority axis, the
// per-phase Enable source, the deliverable kinds, the two ADR-0099 signal
// names and the sandbox modes. Pure data with String projections.
```

### `go/internal/config/vocabulary.go:46` — above `type ModelRouting int`

```text
// ModelRouting is the model-authority axis (cycle-436): who decides the LLM
// CLI + abstract model TIER for an EXISTING phase's dispatch. A THIRD axis,
// genuinely orthogonal to Stage (sequencing: which phases run) and Mode (the
// routing brain) — parsing/applying it must never read or write cfg.Stage or
// cfg.Mode, and vice versa (H3/TestC436_015).
```

### `go/internal/config/vocabulary.go:97` — above `const (`

```text
// DeliverableKindCode and DeliverableKindDocument are the two deliverable kinds
// a cycle can declare (ADR-0099). The vocabulary lives here because config is
// the leaf every consumer (router, core, the phases, the CLI) already imports.
```

### `go/internal/config/vocabulary.go:105` — above `const (`

```text
// SignalDeliverableKind and SignalGoalType are the routable field names of the
// two ADR-0099 signals — the ONE word a conditional_mandatory clause, an
// overlay `when` clause and core's dispatch projection all use for each
// (router.resolveField switches on them).
```
