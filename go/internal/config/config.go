// Package config is the single composition-root loader for evolve-loop's
// dynamic-routing configuration. It is the ONLY place that reads routing
// env vars + the central registry file; every downstream consumer receives
// the immutable RoutingConfig by injection and never calls os.Getenv.
//
// Leaf package by design (imports only stdlib): like internal/failureadapter,
// it must NOT import internal/core, so that core.Orchestrator can import it
// without a cycle. Phase identifiers cross the boundary as plain strings;
// core converts to/from core.Phase at the call site.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Stage is the dynamic-routing rollout stage (shadow → advisory → enforce).
type Stage int

const (
	StageOff      Stage = iota // legacy: static state machine drives, router off
	StageShadow                // router computes + logs, static still drives
	StageAdvisory              // router drives the optional surface; spine static
	StageEnforce               // router drives, clamped by the kernel
)

func (s Stage) String() string {
	switch s {
	case StageShadow:
		return "shadow"
	case StageAdvisory:
		return "advisory"
	case StageEnforce:
		return "enforce"
	default:
		return "0"
	}
}

// Mode selects the routing brain (Strategy). Default is DynamicLLM (locked decision).
type Mode int

const (
	ModeDynamicLLM   Mode = iota // LLM proposes, kernel clamps (default)
	ModeStaticPreset             // deterministic: triggers + spine only, no LLM
)

func (m Mode) String() string {
	if m == ModeStaticPreset {
		return "static"
	}
	return "llm"
}

// ModelRouting is the model-authority axis (cycle-436): who decides the LLM
// CLI + abstract model TIER for an EXISTING phase's dispatch. A THIRD axis,
// genuinely orthogonal to Stage (sequencing: which phases run) and Mode (the
// routing brain) — parsing/applying it must never read or write cfg.Stage or
// cfg.Mode, and vice versa (H3/TestC436_015).
type ModelRouting int

const (
	// ModelRoutingStatic is the zero value (safe default): every phase's CLI/
	// tier stays profile-pinned, exactly as today — an advisor {cli,tier}
	// proposal (if any) is never even generated as an authority signal.
	ModelRoutingStatic ModelRouting = iota
	// ModelRoutingAdvisory logs the advisor's proposed {cli,tier} per phase
	// (forensics/soak) but never applies it — dispatch stays profile-pinned.
	ModelRoutingAdvisory
	// ModelRoutingAuto applies the advisor's proposed {cli,tier} as a soft
	// overlay, clamped by router.ClampPlanModelRouting before dispatch.
	ModelRoutingAuto
)

func (m ModelRouting) String() string {
	switch m {
	case ModelRoutingAdvisory:
		return "advisory"
	case ModelRoutingAuto:
		return "auto"
	default:
		return "static"
	}
}

// Enable is the per-phase enablement decision source.
type Enable int

const (
	EnableContent Enable = iota // decided by routing triggers (Specification)
	EnableOn                    // force-run
	EnableOff                   // force-skip
)

func (e Enable) String() string {
	switch e {
	case EnableOn:
		return "on"
	case EnableOff:
		return "off"
	default:
		return "content"
	}
}

// CondRule is a parsed conditional-mandatory predicate, e.g. cycle_size != trivial.
type CondRule struct {
	Field string
	Op    string
	Value string
}

// Condition is one declarative routing-trigger clause (a Specification), held
// as data from the registry. Evaluation lives in package router (it needs the
// digested signals); config only parses and carries it.
type Condition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// RoutingBlock is the per-phase declarative trigger set.
type RoutingBlock struct {
	InsertWhen []Condition `json:"insert_when"`
	SkipWhen   []Condition `json:"skip_when"`
	// RubricHint lines render into the advisor's decision rubric (one "- "
	// bullet each, phases sorted), making the rubric phase DATA instead of
	// hardcoded Go (failure floor Phase 4b). A rubric-only block is
	// walk-inert: mandatory phases never consult Triggers, and empty
	// insert_when never fires (pinned by
	// TestWalk_MandatoryPhaseWithRubricOnlyRoutingBlockUnchanged).
	RubricHint []string `json:"rubric_hint,omitempty"`
}

// RolloutStages groups the three independent rollout-axis dials, each gating a
// subsystem's Off→Shadow→Enforce migration. They share no logic, but all three
// are composition-root *views* of an env-driven signal the subprocess reads
// directly. Embedded anonymously in RoutingConfig, so every cfg.CommitEvidence
// / cfg.ReviewGate / cfg.SandboxMode access is unchanged via field promotion.
type RolloutStages struct {
	// CommitEvidence is the ADR-0027 commit-as-evidence rollout stage:
	// StageOff (legacy path-poll, byte-identical), StageShadow (git-evidence
	// computed + logged, artifact authoritative), StageEnforce (git-evidence
	// authoritative, phases commit, kernel relaxed). StageAdvisory is not used
	// for this axis. The bridge driver reads EVOLVE_COMMIT_EVIDENCE from env
	// directly (it is a subprocess); this field is the orchestrator's view.
	CommitEvidence Stage
	// ReviewGate is Workstream E2's per-phase review-gate rollout stage:
	//   StageOff      — orchestrator uses noopReviewer (every non-SKIPPED
	//                   verdict approved). Byte-identical to pre-E2.
	//   StageShadow   — deterministic reviewer runs but is log-only.
	//   StageEnforce  — deterministic + (future) LLM reviewer authoritative;
	//                   reject aborts the cycle.
	// StageAdvisory is not used for this axis (no advisory-intermediate). The
	// orchestrator owns the stage interpretation; this field is the
	// composition-root view, exactly like CommitEvidence.
	ReviewGate Stage
	// EvalGate is the structural inter-phase eval-gate rollout stage
	// (internal/evalgate): StageOff — no eval gates (orchestrator keeps the
	// noopReviewer; byte-identical); StageShadow — Gate A (scout eval-file
	// materialization) + Gate B (tdd predicate-quality) run + log but always
	// approve; StageEnforce — a CERTAIN violation (a stat'd-missing eval file
	// or a definite tautology) aborts the cycle. The gates fail OPEN on any
	// ambiguity, so enforce-default never false-blocks a healthy cycle.
	// Configured through policy.GatesConfig; default StageEnforce.
	EvalGate Stage
	// ContractGate is the deliverable-contract gate rollout stage
	// (internal/deliverable, ADR-0034): StageOff — no contract gate
	// (orchestrator keeps the noopReviewer; byte-identical); StageShadow —
	// the verifier runs but every violation is log-only; StageEnforce — a
	// confirmed well-formedness violation (missing/misplaced/malformed
	// deliverable) rejects the phase. The gate fails OPEN on ambiguity, and a
	// runtime circuit breaker demotes enforce→advisory after N consecutive
	// blocks so a miscalibrated gate cannot brick the loop. Configured through
	// policy.GatesConfig; default StageEnforce.
	ContractGate Stage
	// TriageCapGate is the R9.2 triage capacity clamp rollout stage
	// (internal/triagecap): StageOff — no clamp; StageShadow — overpacked
	// triage logs a would-block but is approved; StageEnforce — committed
	// coverage floors above ceil(1.25·K) (K = observed throughput window)
	// reject the triage deliverable through the correction ladder with a
	// cap directive (inbox coverage-floor-overpacking: three consecutive
	// coverage cycles burned on the same overpacked shape). Fails OPEN on
	// any ambiguity. Configured through policy.GatesConfig; default StageEnforce.
	TriageCapGate Stage
	// TopNGate is the build->audit top_n task-binding gate rollout stage
	// (internal/topngate): StageOff — no gate (noopReviewer; byte-identical);
	// StageShadow — an out-of-lane build (a build-report ## Task: slug outside
	// triage ## top_n) is logged but approved; StageEnforce — a CERTAIN
	// out-of-lane build aborts the cycle at the build->audit transition before
	// audit/ship spend (inbox builder-task-binding-topn-gate, 8th recurrence).
	// Fails OPEN on ambiguity (missing report, empty top_n). Configured through
	// policy.GatesConfig; default StageEnforce.
	TopNGate Stage
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
	SandboxMode string
	// PhaseRecovery is the ADR-0044 Unified Phase Recovery rollout stage —
	// the ONE dial for the whole program (fatal-pane fast-fail, the
	// observer's chain-backed StallPolicy, the orchestrator's failure-advisor
	// hook):
	//   StageOff     — recovery components inert; byte-identical legacy.
	//   StageShadow  — classify + log would-be actions only (DEFAULT).
	//   StageEnforce — corrective actions execute (fast-fail, stall policy,
	//                  advise+promote). Classification is always-on above off;
	//                  only ACTING is staged.
	// PRECEDENCE NOTE: the bridge and observer subprocesses read
	// EVOLVE_PHASE_RECOVERY from their own env (the actual hot-path signal,
	// same pattern as CommitEvidence/SandboxMode); this field is the
	// composition-root/orchestrator view, set from the same env var by
	// applyEnv. Default StageShadow per ADR-0044 (behavior-neutral first ship).
	PhaseRecovery Stage

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
	SpineFloor Stage

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
	PhaseIO Stage

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
	RouterReplan Stage

	// MergeGate is the merge-to-main gate rollout dial. Like RouterReplan/PhaseIO
	// it uses the FULL off→shadow→advisory→enforce ladder:
	//   StageOff      — the gate is never inserted; byte-identical legacy (the
	//                   per-cycle ship path to main is unaffected).
	//   StageShadow   — the gate runs and records its would-be promotion verdict
	//                   to the ledger/dossier, but promotes nothing (DEFAULT).
	//   StageAdvisory — the gate's verdict surfaces as a recommendation; still no
	//                   automatic promotion.
	//   StageEnforce  — on a PASS verdict the kernel auto-promotes the completed
	//                   milestone's integration branch to main through the
	//                   hardened ship/merge-train path (armed auto-rollback).
	// This is the composition-root view, loaded from policy.MergeGateConfig; the
	// promoter call site reads it. Default StageShadow (set in defaults()) — the
	// auto-merge in enforce activates only after a human-watched shadow soak.
	MergeGate Stage
	// ParallelEvaluate is the post-build checking-phase parallelization dial. It
	// uses the off→shadow→enforce trichotomy:
	//   StageOff      — the dispatcher is dormant; phases run sequentially
	//                   (DEFAULT, byte-identical legacy).
	//   StageShadow   — the would-be batch + projected saving is recorded only.
	//   StageEnforce  — the independent post-build evaluate phases (archetype
	//                   "evaluate", excluding the audit verdict-brancher) run
	//                   CONCURRENTLY; a single serialized merge records all
	//                   outcomes (weakest-link verdict). The enforce flip activates
	//                   only after a shadow soak (the ~11% projected fleet saving).
	// Concurrency is ParallelEvaluateConcurrency (RoutingConfig). Composition-root
	// view: cmd_cycle.go calls policy.ParallelEvaluateConfig() and writes both
	// ParallelEvaluate and ParallelEvaluateConcurrency before the runners are built.
	ParallelEvaluate Stage
	// ScoutDecompose is the scout map-reduce dial (off->shadow->enforce, default off):
	// enforce runs N scout-scan workers over codebase slices concurrently and the
	// scout agent synthesizes from their merged digests instead of scanning itself.
	// Concurrency = ScoutDecomposeConcurrency. Flip to enforce only after a shadow soak.
	ScoutDecompose Stage
}

// RoutingConfig is the immutable, typed configuration object. Loaded once at
// the composition root, injected everywhere else.
type RoutingConfig struct {
	Stage Stage
	Mode  Mode
	// ModelRouting is the cycle-436 model-authority axis (static|advisory|
	// auto), config-driven via .evolve/policy.json `model_routing`. Zero value
	// ModelRoutingStatic — byte-identical default for every operator who never
	// sets model_routing.
	ModelRouting ModelRouting
	// RolloutStages embeds CommitEvidence / ReviewGate / SandboxMode — the
	// three subsystem-migration dials, promoted so existing field access is
	// unchanged (see RolloutStages).
	RolloutStages
	Mandatory     []string            // ordered mandatory phase names
	Conditional   map[string]CondRule // phase -> conditional-mandatory rule
	MaxInsertions int
	// ParallelEvaluateConcurrency bounds how many post-build evaluate phases the
	// ParallelEvaluate=enforce dispatcher runs at once. Default 3 (the soak sweet
	// spot: ~11% saving, diminishing past it). Resolved from policy.
	ParallelEvaluateConcurrency int
	// ScoutDecomposeConcurrency bounds the parallel scout-scan workers. Default 4.
	ScoutDecomposeConcurrency int
	PhaseEnable               map[string]Enable       // phase -> enablement source
	Triggers                  map[string]RoutingBlock // phase -> declarative triggers
	// Order is the linear phase sequence the router walks, in registry order.
	// Empty ⇒ the router falls back to its built-in canonicalOrder (so a config
	// loaded without a registry stays byte-identical to pre-Order behavior).
	// The composition root may splice user phases into this slice.
	Order []string
	// SpineOrder is the config-declared static-spine successor sequence (registry
	// config.spine_order, PA-DDK DDK-3) — the mandatory-default linear path the
	// state machine walks, distinct from Order (which interleaves optional
	// insertions). Empty ⇒ the kernel's canonical spineOrder literal.
	SpineOrder []string
	// LegalSuccessors is the config-declared transition legality graph (registry
	// config.legal_successors, PA-DDK DDK-5): phase name → legal successor names.
	// It keys by NAME — including the graph sentinels start/end and the control
	// phase debugger, which are not registry phases[] entries — so the whole graph
	// has one SSOT (mirroring SpineOrder/Mandatory) rather than orphaning sentinel
	// edges across per-phase fields. Empty ⇒ the kernel's literal `allowed` graph
	// (byte-identical fallback). The load-time ValidateSafetyInvariants validator
	// is the trust anchor that gates any operator edit to this map.
	LegalSuccessors map[string][]string
	// AuditFailRoutesTo is the failure-floor policy route for the audit-FAIL
	// edge ("retrospective" | "memo"), merged from .evolve/policy.json:
	// failure_floor at the composition root — the ONE user surface for this
	// decision. Empty ⇒ retro is enabled via PhaseEnable defaults.
	AuditFailRoutesTo string
	// GoalRecipes is the ADR-0052 WS5 recipe SSOT (config.goal_recipes in the
	// registry): goal type → ordered, verbatim recipe-token list. It is the
	// single source projected into the persona's "## Goal-Type Recipes" table
	// (via router.RenderRecipeProjection, locked by TestRouterPersonaRecipeTable_NoDrift)
	// and read by the RecipeVerifier — ending the prior three-source recipe drift.
	// Registry-sourced only; nil when no registry supplies it (projection renders empty).
	GoalRecipes map[string][]string
	// RoutingJudge is the ADR-0052 advisor-maximization WS4 route-quality judge
	// toggle. false (DEFAULT) — no judge call,
	// byte-identical. true — the fast-tier LLM-as-judge scores the emitted route
	// for forensics, strictly off the build path (never gates ship, never alters
	// the plan). It is a plain bool, NOT a Stage: the judge cannot move behavior,
	// so the off→shadow→advisory ladder would be meaningless (shadow≡advisory≡on).
	// Composition-root view loaded from policy; the scoring call site reads it.
	RoutingJudge bool
	// ReconDigest is the ADR-0052 advisor-maximization WS2-S0b toggle for the
	// deterministic pre-plan recon digest. false
	// (DEFAULT) — the initial Plan prompt is byte-identical to pre-slice. true —
	// the advisor renders measured repo facts (langs/tests/hotspots, goal-keyword
	// hits, backlog/carryover) under "## Pre-plan recon (deterministic)" so
	// upfront selection is grounded in evidence, not goal-text inference alone.
	// A plain bool (not a Stage): it injects deterministic facts the floor still
	// clamps, so there is no shadow/advisory distinction. Composition-root view
	// loaded from policy; composePlanPrompt reads it.
	ReconDigest bool
	// RePlanMaxDepth caps how many post-scout re-plans a single cycle may run
	// (ADR-0052 WS2-S5; research P4 — cap depth, escalate not loop). Default 1
	// (set in defaults()): one measured re-plan per cycle, then escalate to the
	// debugger rather than thrash. A cycle-scoped counter cr.replanDepth tracks
	// the live depth. A non-positive policy value falls back to 1.
	RePlanMaxDepth int
	// CompactPrompts, when true, enables on-demand reference-section stripping from
	// disk-loaded agent docs before dispatch ("## Reference Index (Layer 3, on-demand)"
	// and everything after it). Default true (set in defaults()). Config path:
	// registry config.workflow.compact_prompts=false disables for all phases.
	// The value flows: config.Load → RoutingConfig.CompactPrompts → phase Config.CompactPrompts
	// → runner.Options.CompactPrompts — never a bare literal in a phase constructor.
	CompactPrompts bool
}

// Sandbox mode string constants — exported so the bridge + tests can match
// without sprinkling magic strings.
const (
	SandboxModeAuto = "auto"
	SandboxModeOn   = "on"
	SandboxModeOff  = "off"
)

// Warning is a non-fatal config diagnostic surfaced to the operator (and ledger).
type Warning struct {
	Code    string // "weak-spine" | "unknown-value" | "unknown-key" | "inert-phase-enable" | "deprecated-flag"
	Message string
}

// registryDoc is the subset of phase-registry.json this loader reads.
type registryDoc struct {
	Config struct {
		DynamicRouting        string              `json:"dynamic_routing"`
		RoutingMode           string              `json:"routing_mode"`
		ModelRouting          string              `json:"model_routing"`
		MandatoryPhases       []string            `json:"mandatory_phases"`
		SpineOrder            []string            `json:"spine_order"`
		LegalSuccessors       map[string][]string `json:"legal_successors"`
		ConditionalMandatory  map[string]string   `json:"conditional_mandatory"`
		MaxOptionalInsertions *int                `json:"max_optional_insertions"`
		GoalRecipes           map[string][]string `json:"goal_recipes"`
		Workflow              struct {
			// CompactPrompts enables/disables on-demand reference-section stripping.
			// Absent = use RoutingConfig default (true). Explicit false opts out.
			CompactPrompts *bool `json:"compact_prompts,omitempty"`
		} `json:"workflow"`
	} `json:"config"`
	Phases []struct {
		Name     string        `json:"name"`
		Optional bool          `json:"optional"`
		Enabled  string        `json:"enabled"`
		Routing  *RoutingBlock `json:"routing"`
	} `json:"phases"`
}

// Load resolves the effective RoutingConfig. Precedence: env override >
// registry file > built-in default. env is injected (not read from the
// process) so the loader stays testable and is the sole contained env site.
func Load(registryPath string, env map[string]string) (RoutingConfig, []Warning) {
	var ws []Warning

	cfg := defaults()

	if env["EVOLVE_USE_PHASE_REGISTRY"] != "0" {
		if doc, ok := readRegistry(registryPath); ok {
			applyRegistry(&cfg, doc, &ws)
		}
	}

	applyEnv(&cfg, env, &ws)

	validateSpine(cfg, &ws)
	validateInertEnables(cfg, &ws)
	return cfg, ws
}

// staticSpinePhases is the set of phases the legacy state machine drives as
// agent runs (excluding the start/end sentinels). When Stage==StageOff the
// router is off and ONLY these phases get a turn — so a PhaseEnable[p]=On for
// any other phase is silently inert. Encoded as a local set rather than
// imported from core because config is a leaf package; the
// TestStaticSpineMatchesStateMachine cross-package contract test pins this
// against the actual state machine's edge map.
var staticSpinePhases = map[string]struct{}{
	"intent":        {},
	"scout":         {},
	"triage":        {},
	"tdd":           {},
	"build-planner": {},
	"build":         {},
	"audit":         {},
	"ship":          {},
	"retro":         {},
}

// validateInertEnables warns when PhaseEnable[p]=EnableOn but p is neither
// mandatory, in the static spine, nor reachable via the router (Stage<Advisory).
// The classic trigger is plan-review enabled via policy.json with default routing:
// plan-review only runs at Stage>=Advisory, so the enable is silently inert at
// Stage=Off AND at Stage=Shadow (per the Stage docstring, shadow computes+logs but
// the STATIC state machine still drives execution — so non-spine phases remain
// unreachable). Surfacing this prevents the operator-confusion failure mode
// from cycle 120.
func validateInertEnables(cfg RoutingConfig, ws *[]Warning) {
	if cfg.Stage >= StageAdvisory {
		return // router drives; enable is effective
	}
	// Sort for deterministic warning order — map iteration is randomized.
	phases := make([]string, 0, len(cfg.PhaseEnable))
	for p := range cfg.PhaseEnable {
		phases = append(phases, p)
	}
	sort.Strings(phases)
	for _, p := range phases {
		if cfg.PhaseEnable[p] != EnableOn {
			continue
		}
		if containsPhase(cfg.Mandatory, p) {
			continue
		}
		if _, inSpine := staticSpinePhases[p]; inSpine {
			continue
		}
		*ws = append(*ws, Warning{"inert-phase-enable",
			fmt.Sprintf("phase %q is force-enabled but the router is off/shadow (dynamic_routing<advisory) and it is not in the static state machine — the enable is inert; set dynamic_routing>=advisory or remove the enable", p),
		})
	}
}

func defaults() RoutingConfig {
	return RoutingConfig{
		// Dynamic routing is DEFAULT-ON (Component #7): the advisor drives phase
		// selection every cycle, with the integrity floor (ClampPlanToFloor +
		// SpineSatisfiedUpTo) — not a flag — protecting the ship guarantee.
		// EVOLVE_DYNAMIC_ROUTING still overrides (e.g. =off for the legacy static
		// path). Flipped from StageOff after the advisory mode soaked since
		// cycle-108.
		Stage: StageAdvisory,
		Mode:  ModeDynamicLLM,
		// SpineFloor=StageEnforce (the R8.5 flip, landed 2026-07-16 as its OWN
		// dial): the artifact-backed spine floor ABORTS on a clean-absence
		// handoff gap instead of WARN-and-proceed. Evidence: after the
		// scout/audit digest fallbacks (router/digest.go), a 536-run-dir replay
		// showed 0 would-block transitions on every cycle shape since
		// ~cycle-480 (the only misses were pre-convention dirs from the 361-479
		// era). Degraded reads stay fail-open (the cleanAbsence guard in
		// cyclerun_select.go), an enforce block records to failure-learning,
		// and policy.json `recovery.spine_floor: "shadow"` is the no-recompile
		// escape hatch. Deliberately DECOUPLED from PhaseRecovery: that dial is
		// overloaded (ADR-0045 I6 folded the bidirectional channel into it, and
		// the failure-adviser promotion path also keys on it), so arming the
		// floor via PhaseRecovery would have armed two unsoaked subsystems.
		// PhaseRecovery itself stays shadow.
		RolloutStages:  RolloutStages{CommitEvidence: StageOff, SandboxMode: SandboxModeAuto, EvalGate: StageEnforce, ContractGate: StageEnforce, TriageCapGate: StageEnforce, TopNGate: StageEnforce, PhaseRecovery: StageShadow, SpineFloor: StageEnforce, PhaseIO: StageEnforce, RouterReplan: StageShadow, MergeGate: StageShadow, ParallelEvaluate: StageOff, ScoutDecompose: StageOff},
		CompactPrompts: true, // default ON: strips ~23 KB/cycle of on-demand reference tails
		// NOTE: this built-in baseline intentionally omits triage; the real
		// registry (docs/architecture/phase-registry.json) adds it via
		// applyRegistry (cycles 263/264: the advisory router skipped the
		// scope-clamp). Tests constructing RoutingConfig directly keep this
		// 4-phase baseline.
		Mandatory:                   []string{"scout", "build", "audit", "ship"},
		Conditional:                 map[string]CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions:               4,
		ParallelEvaluateConcurrency: 3, // soak sweet spot (~11% saving; diminishing past it)
		ScoutDecomposeConcurrency:   4,
		RePlanMaxDepth:              1, // ADR-0052 WS2-S5: one measured re-plan/cycle, then escalate

		// Legacy phase-enable defaults, so PhasePolicy reproduces pre-routing
		// behavior even when the registry file is absent (e.g. tests): triage
		// and tdd run by default; build-planner is opt-in (shadow). These are
		// the floor the registry `enabled` field and env flags override.
		PhaseEnable: map[string]Enable{
			"triage":        EnableOn,
			"tdd":           EnableOn,
			"build-planner": EnableOff,
			"swarm-plan":    EnableOff,
		},
		Triggers: map[string]RoutingBlock{},
	}
}

func readRegistry(path string) (registryDoc, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return registryDoc{}, false
	}
	var doc registryDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return registryDoc{}, false
	}
	return doc, true
}

func applyRegistry(cfg *RoutingConfig, doc registryDoc, ws *[]Warning) {
	c := doc.Config
	if c.DynamicRouting != "" {
		cfg.Stage = parseStage(c.DynamicRouting, "dynamic_routing", ws)
	}
	if c.RoutingMode != "" {
		cfg.Mode = parseMode(c.RoutingMode, ws)
	}
	if c.ModelRouting != "" {
		cfg.ModelRouting = parseModelRouting(c.ModelRouting, "model_routing", ws)
	}
	if len(c.MandatoryPhases) > 0 {
		cfg.Mandatory = c.MandatoryPhases
	}
	if len(c.SpineOrder) > 0 {
		cfg.SpineOrder = c.SpineOrder
	}
	if len(c.LegalSuccessors) > 0 {
		cfg.LegalSuccessors = c.LegalSuccessors
	}
	if c.MaxOptionalInsertions != nil {
		cfg.MaxInsertions = *c.MaxOptionalInsertions
	}
	if len(c.GoalRecipes) > 0 {
		cfg.GoalRecipes = c.GoalRecipes
	}
	if c.Workflow.CompactPrompts != nil {
		cfg.CompactPrompts = *c.Workflow.CompactPrompts
	}
	for phase, expr := range c.ConditionalMandatory {
		if rule, err := parseCondRule(expr); err == nil {
			cfg.Conditional[phase] = rule
		} else {
			*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("conditional_mandatory[%s]=%q: %v", phase, expr, err)})
		}
	}
	for _, p := range doc.Phases {
		if p.Name == "" {
			continue
		}
		cfg.Order = append(cfg.Order, p.Name)
		if p.Enabled != "" {
			cfg.PhaseEnable[p.Name] = parseEnable(p.Enabled, ws)
		}
		if p.Routing != nil {
			cfg.Triggers[p.Name] = *p.Routing
		}
	}
}

func applyEnv(cfg *RoutingConfig, env map[string]string, ws *[]Warning) {
	if v := env["EVOLVE_DYNAMIC_ROUTING"]; v != "" {
		cfg.Stage = parseStage(v, "dynamic_routing", ws)
	}
	if v := env["EVOLVE_ROUTING_MODE"]; v != "" {
		cfg.Mode = parseMode(v, ws)
	}
	// model_routing is CONFIG-DRIVEN via .evolve/policy.json only — no env dial.
	// The flag-ceiling ratchet forbids adding operator env flags; the axis lives
	// in policy.json (parsed above via the registry model_routing key).
	if v := env["EVOLVE_COMMIT_EVIDENCE"]; v != "" {
		cfg.CommitEvidence = parseEvidenceStage(v, "EVOLVE_COMMIT_EVIDENCE", ws)
	}
	if v := env["EVOLVE_PHASE_IO"]; v != "" {
		// ADR-0050 Phase 3 — the unified phase-I/O rollout dial. Reuses
		// parseStage (the 4-value off→shadow→advisory→enforce ladder) so a typo
		// falls back to off (fail-safe), never leaving the dial in an unintended
		// state. Default (no env) is enforce as of the 3.10 cutover, set in
		// defaults(); set EVOLVE_PHASE_IO=off to roll back.
		cfg.PhaseIO = parseStage(v, "EVOLVE_PHASE_IO", ws)
	}
	if v := env["EVOLVE_SANDBOX"]; v != "" {
		switch strings.TrimSpace(v) {
		case SandboxModeAuto, SandboxModeOn, SandboxModeOff:
			cfg.SandboxMode = strings.TrimSpace(v)
		default:
			*ws = append(*ws, Warning{"unknown-value",
				fmt.Sprintf("EVOLVE_SANDBOX=%q unknown (want auto|on|off), defaulting to %q", v, cfg.SandboxMode)})
		}
	}
	if v := env["EVOLVE_MANDATORY_PHASES"]; v != "" {
		cfg.Mandatory = splitCSV(v)
	}
	if v := env["EVOLVE_CONDITIONAL_MANDATORY"]; v != "" {
		// format: phase:expr  e.g. tdd:cycle_size!=trivial
		if phase, expr, ok := strings.Cut(v, ":"); ok {
			if rule, err := parseCondRule(expr); err == nil {
				cfg.Conditional[strings.TrimSpace(phase)] = rule
			} else {
				*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("EVOLVE_CONDITIONAL_MANDATORY=%q: %v", v, err)})
			}
		}
	}
	if v := env["EVOLVE_MAX_OPTIONAL_INSERTIONS"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxInsertions = n
		} else {
			*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("EVOLVE_MAX_OPTIONAL_INSERTIONS=%q not an int", v)})
		}
	}
}

// containsPhase reports whether slice contains p.
func containsPhase(slice []string, p string) bool {
	for _, s := range slice {
		if s == p {
			return true
		}
	}
	return false
}

func validateSpine(cfg RoutingConfig, ws *[]Warning) {
	var missing []string
	if !containsPhase(cfg.Mandatory, "audit") {
		missing = append(missing, "audit")
	}
	if !containsPhase(cfg.Mandatory, "ship") {
		missing = append(missing, "ship")
	}
	if len(missing) > 0 {
		*ws = append(*ws, Warning{
			Code:    "weak-spine",
			Message: "mandatory_phases omits " + strings.Join(missing, "+") + " — audit-before-ship guarantee weakened",
		})
	}
	// The artifact-backed floor (core.SpineSatisfiedUpTo) walks the mandatory
	// anchors in their configured-order position, so a scrambled order that places
	// ship before audit would let ship's gate skip the shippable-audit check. The
	// legality graph + audit verdict branch still independently block it, but
	// surface the misordering loudly so it is never the sole guard.
	auditPos, shipPos := -1, -1
	for i, p := range cfg.Order {
		switch p {
		case "audit":
			auditPos = i
		case "ship":
			shipPos = i
		}
	}
	if auditPos >= 0 && shipPos >= 0 && auditPos > shipPos {
		*ws = append(*ws, Warning{
			Code:    "spine-order",
			Message: "phase order places ship before audit — audit must precede ship for the shippable-audit floor to gate ship",
		})
	}
}

// parseStage parses a full off→shadow→advisory→enforce dial (the 4-value
// ladder, unlike parseEvidenceStage's off/shadow/enforce trichotomy). varName
// names the offending key in the unknown-value warning; an unknown value
// defaults to off (a typo must never silently enable a kill-path or a staged
// rollout). Shared by dynamic routing and unified phase I/O.
func parseStage(v, varName string, ws *[]Warning) Stage {
	switch strings.TrimSpace(v) {
	case "0", "off":
		return StageOff
	case "shadow":
		return StageShadow
	case "advisory":
		return StageAdvisory
	case "enforce":
		return StageEnforce
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("%s=%q unknown, defaulting to off", varName, v)})
		return StageOff
	}
}

// parseEvidenceStage parses an off/shadow/enforce dial. Unlike parseStage it has
// no "advisory" middle state — these axes are compute-and-log (shadow) vs act
// (enforce). Any unknown value defaults to off with a warning (a typo must never
// silently enable a kill-path). Used by the remaining environment-backed
// rollout stages; varName names the offending env var in the warning.
func parseEvidenceStage(v, varName string, ws *[]Warning) Stage {
	switch strings.TrimSpace(v) {
	case "0", "off":
		return StageOff
	case "shadow":
		return StageShadow
	case "enforce":
		return StageEnforce
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("%s=%q unknown (want off|shadow|enforce), defaulting to off", varName, v)})
		return StageOff
	}
}

func parseMode(v string, ws *[]Warning) Mode {
	switch strings.TrimSpace(v) {
	case "llm", "dynamic", "dynamic-llm":
		return ModeDynamicLLM
	case "static", "static-preset", "preset":
		return ModeStaticPreset
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("routing_mode=%q unknown, defaulting to llm", v)})
		return ModeDynamicLLM
	}
}

// parseModelRouting parses the cycle-436 model-authority axis. Unknown values
// fall back to the SAFE static side (never silently enable auto) with a
// warning — mirroring parseStage/parseMode's fail-safe-with-warning contract.
// varName names the offending key/env var in the warning.
func parseModelRouting(v, varName string, ws *[]Warning) ModelRouting {
	switch strings.TrimSpace(v) {
	case "static":
		return ModelRoutingStatic
	case "advisory":
		return ModelRoutingAdvisory
	case "auto":
		return ModelRoutingAuto
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("%s=%q unknown (want static|advisory|auto), defaulting to static", varName, v)})
		return ModelRoutingStatic
	}
}

func parseEnable(v string, ws *[]Warning) Enable {
	switch strings.TrimSpace(v) {
	case "on":
		return EnableOn
	case "off":
		return EnableOff
	case "content":
		return EnableContent
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("enabled=%q unknown, defaulting to content", v)})
		return EnableContent
	}
}

// parseCondRule parses "field<op>value" where op is one of != == >= <= > <.
// Tolerates surrounding whitespace. Two-char ops are matched before one-char.
func parseCondRule(expr string) (CondRule, error) {
	for _, op := range []string{"!=", "==", ">=", "<="} {
		if i := strings.Index(expr, op); i >= 0 {
			return CondRule{
				Field: strings.TrimSpace(expr[:i]),
				Op:    op,
				Value: strings.TrimSpace(expr[i+2:]),
			}, nil
		}
	}
	for _, op := range []string{">", "<"} {
		if i := strings.Index(expr, op); i >= 0 {
			return CondRule{
				Field: strings.TrimSpace(expr[:i]),
				Op:    op,
				Value: strings.TrimSpace(expr[i+1:]),
			}, nil
		}
	}
	return CondRule{}, fmt.Errorf("no comparison operator in %q", expr)
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
