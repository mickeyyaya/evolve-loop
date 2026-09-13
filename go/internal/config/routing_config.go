package config

// routing_config.go — the resolved value every routing consumer is injected
// with: RoutingConfig and the registry-sourced shapes it carries (the
// deliverable-kind contract, the declarative trigger clauses). Data only.

// DeliverableKindSpec is the shape a cycle's deliverable must take for one
// deliverable kind (ADR-0099 slice 2) — declared ONCE in
// phase-registry.json:config.deliverable_kinds and judged by ONE engine
// (internal/solutioncheck) that the build floor, the `evolve solution check`
// CLI and the audit gate all project. Paths are relative to the kind's Root
// directory under the repo, e.g. solutions/<slug>/recommendation.md.
type DeliverableKindSpec struct {
	Root               string              `json:"root"`
	MinOptions         int                 `json:"min_options"`
	RequiredFiles      []string            `json:"required_files"`
	RequiredSections   map[string][]string `json:"required_sections"`
	ForbidPlaceholders []string            `json:"forbid_placeholders"`
	EvidenceFile       string              `json:"evidence_file"`
}

// DocumentSpec returns the document deliverable contract when the registry
// declares one — the ONE lookup every projection (floor, audit gate, CLI, task
// contract) uses, so none of them spells the kind key.
func (c RoutingConfig) DocumentSpec() (DeliverableKindSpec, bool) {
	spec, ok := c.DeliverableKinds[DeliverableKindDocument]
	return spec, ok
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

// RoutingConfig is the typed configuration object the composition root
// resolves in two steps — Loader.Load (defaults, the registry, the env) and
// Loader.ApplyPolicyStages (the .evolve/policy.json gate/recovery/router
// dials) — and then injects everywhere else, unchanged.
type RoutingConfig struct {
	Stage Stage
	Mode  Mode
	// ModelRouting is the cycle-436 model-authority axis (static|advisory|
	// auto), registry-driven via docs/architecture/phase-registry.json
	// `config.model_routing` (no env dial, no policy.json key). Zero value
	// ModelRoutingStatic — byte-identical default for every operator who never
	// sets model_routing.
	ModelRouting ModelRouting
	// RolloutStages embeds CommitEvidence / ReviewGate / SandboxMode — the
	// three subsystem-migration dials, promoted so existing field access is
	// unchanged (see RolloutStages).
	RolloutStages
	Mandatory   []string            // ordered mandatory phase names
	Conditional map[string]CondRule // phase -> conditional-mandatory rule
	// DeliverableKinds is the per-kind deliverable contract (ADR-0099 slice 2),
	// registry-sourced; absent ⇒ no document contract is enforced.
	DeliverableKinds map[string]DeliverableKindSpec
	MaxInsertions    int
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
