package router

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
)

// canonicalOrder is the linear phase sequence the walk advances through. The
// mandatory spine (scout→build→audit→ship) is a subset; optional phases sit
// between anchors and run only when triggered/enabled. "retrospective" is the
// canonical name for the retro phase (core.PhaseRetro="retro" is an alias the
// orchestrator maps at the boundary).
var canonicalOrder = []string{
	"intent", "scout", "triage", "plan-review",
	"tdd", "build-planner", "build", "tester",
	"audit", "ship", "retrospective", "memo",
}

// PhaseEnd is the terminal sentinel Route returns when no phase remains.
const PhaseEnd = "end"

// RouteInput is the complete, pre-digested context for one routing decision.
// PURE: the caller does all I/O and hands in plain values.
type RouteInput struct {
	Current         string                 // phase that just completed ("start" on cycle entry)
	Verdict         string                 // its canonical verdict
	Signals         RoutingSignals         // digest envelope (objective)
	History         []failureadapter.Entry // converted state.failedApproaches
	Cfg             config.RoutingConfig
	BudgetRemaining float64
	Completed       []string // phases already done this cycle
	Strict          bool     // EVOLVE_STRICT_AUDIT — threaded to failureadapter for retro
	Now             time.Time
	IntentRequired  bool

	// Proposer context — populated by the orchestrator, consumed ONLY by a
	// DynamicLLM Proposer (which needs to dispatch a bridge call). The pure
	// Route() function ignores these, so determinism is preserved.
	Workspace   string
	ProjectRoot string
	Cycle       int
	Env         map[string]string

	// GoalText is the human-readable goal/strategy for this cycle (the same text
	// Scout works from). Populated by the orchestrator from
	// CycleRequest.Context["strategy"]; consumed ONLY by the DynamicLLM advisor's
	// prompt so the brain can reason about WHAT the cycle is for — the precondition
	// for genuinely selecting a design phase or minting one, rather than planning
	// blind. The pure Route() ignores it, so determinism is preserved. Empty when
	// no goal text was threaded (the advisor then plans from signals + recall only).
	GoalText string

	// CarryoverTodos are unresolved operator/workflow tasks from previous cycles,
	// projected from state.json:carryoverTodos by the orchestrator at cycle start.
	// Consumed ONLY by the DynamicLLM planner prompt so the upfront whole-cycle
	// advisor can select phases based on known backlog before Scout has produced
	// any handoff artifacts. The pure Route() ignores it.
	CarryoverTodos []CarryoverTodo

	// Catalog is the set of pre-defined phases the advisor may SELECT instead of
	// minting a new one (WS3: prefer select-over-mint = DRY at the agent level).
	// Populated by the orchestrator from the phase catalog; consumed ONLY by a
	// DynamicLLM Planner when rendering its prompt. The pure Route() ignores it,
	// so determinism is preserved. Additive: nil until the advisor-prompt slice
	// wires it.
	Catalog []PhaseCard

	// LastReason + Lessons are the recall-memory context (WS2): the short "why"
	// of the most recent failure and the prior lessons that match it, looked up
	// from the knowledge base by the orchestrator. Consumed ONLY by the advisor's
	// prompt (recall informs planning); the pure Route() ignores them. Empty when
	// there is no recent failure or no matching lesson — which is itself the
	// novel-failure signal.
	LastReason string
	Lessons    []string

	// Plan is the advisor's whole-cycle run/skip plan, ALREADY clamped to the
	// integrity floor (ClampPlanToFloor) by the orchestrator before threading.
	// Consulted by shouldRun ONLY at Stage>=Advisory: a NON-mandatory phase runs
	// iff the plan schedules it (Run==true). Because the plan is pre-clamped, the
	// ship-chain (build∧audit∧tdd) is Run even when those phases are not in the
	// configurable mandatory set — the integrity floor that config cannot weaken.
	// Nil ⇒ no advisor plan this cycle ⇒ the legacy trigger-driven path runs
	// unchanged (byte-identical / fail-safe to static). Plain data handed in by
	// the caller, so Route stays deterministic and I/O-free.
	Plan *PhasePlan

	// Blocker is the string-only ship-error envelope the orchestrator passes
	// when routing a ship FAILURE for recovery (Recover, not Route). It is nil
	// for ordinary (non-recovery) routing; the pure Route() ignores it entirely.
	Blocker *Blocker
}

// PhaseCard is the advisor-facing projection of one pre-defined phase: enough
// for the planner to decide "select this" vs "mint a new one". Deliberately
// minimal — name, role archetype, the model tier, and whether it writes source
// (the advisor needs the last for sandbox reasoning).
type PhaseCard struct {
	Name         string `json:"name"`
	Role         string `json:"role"` // plan|build|evaluate|control
	Tier         string `json:"tier,omitempty"`
	WritesSource bool   `json:"writes_source,omitempty"`
}

// CarryoverTodo is the router/advisor-facing projection of one unresolved
// carryover task. It mirrors core.CarryoverTodo without importing core into the
// leaf router package.
type CarryoverTodo struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Priority       string `json:"priority"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
}

// Clamp records a hard-rule override applied to a soft/proposed decision.
type Clamp struct {
	Rule     string `json:"rule"`
	Proposed string `json:"proposed"`
	Forced   string `json:"forced"`
}

// RouterDecision is the structured output. NextPhase is the immediate next
// phase to run; InsertPhases/SkipPhases are the optional phases chosen/declined
// while reaching it (forensic + ledger). Reason names the rule that fired.
type RouterDecision struct {
	NextPhase    string                 `json:"next_phase"`
	InsertPhases []string               `json:"insert_phases,omitempty"`
	SkipPhases   []string               `json:"skip_phases,omitempty"`
	Reason       string                 `json:"reason"`
	Evidence     map[string]interface{} `json:"evidence,omitempty"`
	Clamps       []Clamp                `json:"clamps,omitempty"`
	// Justification is the LLM advisor's one-sentence rationale (DynamicLLM
	// mode only; empty in deterministic/static routing). Captured even when the
	// proposal is clamped, so the shadow soak can diff advisor-rationale against
	// the kernel's static path (ADR-0024 problem #2).
	Justification string `json:"justification,omitempty"`
}

// Proposal is the optional LLM advisory input (DynamicLLM mode). Route treats
// it as advisory only and clamps it to legal/objective bounds.
type Proposal struct {
	NextPhase     string   `json:"next_phase"`
	InsertPhases  []string `json:"insert_phases"`
	Justification string   `json:"justification"`
}

// PhasePlanEntry is one phase's whole-cycle run/skip decision plus the advisor's
// rationale. It is the building block of PhasePlan (ADR-0024 §2): the upfront,
// whole-cycle advisory deciding which phases run this cycle, computed once at
// cycle start. It is the cadence companion to Proposal — Proposal answers the
// per-branch "insert this optional phase?" question from post-phase signals the
// upfront plan cannot yet see.
type PhasePlanEntry struct {
	Phase         string `json:"phase"`
	Run           bool   `json:"run"`
	Justification string `json:"justification,omitempty"`
	// Mint, when present, marks this entry as a NEW phase the advisor is
	// proposing (absent from the catalog). The orchestrator registers it via
	// the trust-kernel clamp and dispatches it by Phase name. Absent (the
	// common case) ⇒ a plain run/skip decision for an existing phase.
	Mint *MintSpec `json:"mint,omitempty"`
}

// MintSpec is the LLM-authorable subset of a minted phase: the persona + the
// dispatch knobs an advisor can realistically emit. The advisor emits a TIER
// (fast/balanced/deep), never a raw model. The full phaseconfig.PhaseConfig is
// reconstructed from this + the entry's Phase name at parse time; gates/IO take
// safe defaults (the registrar forces Optional + sandboxes source-writers).
type MintSpec struct {
	Prompt       string `json:"prompt"`
	Tier         string `json:"tier,omitempty"`
	CLI          string `json:"cli,omitempty"`
	WritesSource bool   `json:"writes_source,omitempty"`
}

// PhasePlan is the advisor's whole-cycle plan. ADVISORY only: the kernel clamp
// re-validates it against the integrity floor before any phase runs, so a
// hallucinated plan can never weaken the ship guarantee ("model proposes, kernel
// disposes"). The clamp itself lands in a later slice; this is the carrier type.
//
// Wire format note: the on-disk/LLM form is a BARE JSON array of PhasePlanEntry
// (see parsePhasePlan). Entries carries no json tag deliberately — callers
// serialize plan.Entries directly (a bare array), never the PhasePlan wrapper,
// so the round-trip stays symmetric with what the advisor emits.
type PhasePlan struct {
	Entries []PhasePlanEntry
	// MintPhases are NEW phases the advisor proposes that are absent from the
	// catalog — each a self-contained config (inline prompt/persona + tier +
	// CLI + gates). The orchestrator registers them through the trust-kernel
	// clamp (envelope/allowed-CLIs) at cycle start, then dispatches them by
	// name through the same path as a built-in. Empty in the common case.
	MintPhases []phaseconfig.PhaseConfig
}

// Route computes the routing decision. PURE: deterministic given its inputs.
// Ordered rules; the clamp pass is non-bypassable and runs last.
func Route(in RouteInput, proposal *Proposal) RouterDecision {
	cur := normalize(in.Current)

	// Rule 0 — Retro delegation. Do NOT duplicate failure logic; defer to the
	// deterministic failure-adapter exactly as orchestrator.decideAfterRetro does.
	if cur == "retrospective" {
		return retroDecision(in)
	}

	// Rule 1 — Audit verdict branch (the one verdict-driven edge). FAIL must not
	// proceed to ship; it diverts to the retrospective learning gate.
	if cur == "audit" {
		if in.Verdict == "FAIL" {
			next := "retrospective"
			if enableOf(in.Cfg, "retrospective") == config.EnableOff {
				next = PhaseEnd
			}
			return RouterDecision{
				NextPhase: next,
				Reason:    "audit-fail-to-retrospective",
				Evidence:  map[string]interface{}{"verdict": in.Verdict},
			}
		}
		// PASS/WARN fall through to the walk (→ ship).
	}

	// Rules 1–3 — walk the canonical order from Current+1 to the first runnable
	// phase, accumulating chosen inserts and declined skips.
	return walk(in, proposal)
}

// retroDecision implements Rule 0 via failureadapter.Decide.
func retroDecision(in RouteInput) RouterDecision {
	dec := failureadapter.Decide(in.History, failureadapter.Options{Now: in.Now, Strict: in.Strict})
	d := RouterDecision{
		Reason:   "retro:" + string(dec.Action),
		Evidence: map[string]interface{}{"action": string(dec.Action)},
	}
	d.SkipPhases = append(d.SkipPhases, dec.SkipPhases...) // carry failure-adapter skips
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		d.NextPhase = "tdd"
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		d.NextPhase = PhaseEnd
	default: // PROCEED
		d.NextPhase = PhaseEnd
	}
	return d
}

func walk(in RouteInput, proposal *Proposal) RouterDecision {
	d := RouterDecision{Evidence: map[string]interface{}{}}
	done := toSet(in.Completed)
	order := effectiveOrder(in.Cfg)
	start := indexOfIn(order, normalize(in.Current)) // -1 for "start"/unknown

	optionalUsed := countOptionalInserts(in.Cfg, in.Completed)

	for i := start + 1; i < len(order); i++ {
		phase := order[i]
		if done[phase] {
			continue
		}
		run, optional, clamp := shouldRun(in, phase, optionalUsed)
		if clamp != nil {
			d.Clamps = append(d.Clamps, *clamp)
		}
		if !run {
			if optional {
				d.SkipPhases = append(d.SkipPhases, phase)
			}
			continue
		}
		if optional {
			d.InsertPhases = append(d.InsertPhases, phase)
		}
		d.NextPhase = phase
		d.Reason = reasonFor(in, phase, optional)
		// Rule 4 — apply the LLM proposal as advisory, clamped to this legal next.
		applyProposal(&d, proposal, in)
		return d
	}

	d.NextPhase = PhaseEnd
	d.Reason = "no-runnable-phase-remaining"
	applyProposal(&d, proposal, in)
	return d
}

// reserved imports guard removed; all imports are used.

// shouldRun decides whether a candidate phase runs. Returns (run, isOptional,
// clamp) where clamp is non-nil when a hard rule overrode the soft decision.
func shouldRun(in RouteInput, phase string, optionalUsed int) (bool, bool, *Clamp) {
	enable := enableOf(in.Cfg, phase)

	// Mandatory phases always run (kernel clamp: enable=Off cannot disable them).
	if isMandatory(in.Cfg, phase) {
		if enable == config.EnableOff {
			return true, false, &Clamp{Rule: "mandatory-never-skipped", Proposed: phase + "=off", Forced: phase + "=on"}
		}
		return true, false, nil
	}

	// Conditional-mandatory (e.g. tdd pinned unless cycle_size==trivial).
	if rule, ok := in.Cfg.Conditional[phase]; ok {
		if evalCondRule(in.Signals, rule) {
			if enable == config.EnableOff {
				return true, false, &Clamp{Rule: "conditional-mandatory-pin", Proposed: phase + "=off", Forced: phase + "=on"}
			}
			return true, false, nil // pinned
		}
		// rule not satisfied → phase is genuinely optional this cycle; fall through.
	}

	// Advisory+ with an advisor plan: the (already floor-clamped) whole-cycle
	// plan drives run/skip for every NON-mandatory phase, replacing the
	// trigger-driven path below. ClampPlanToFloor ran before this plan was
	// threaded in, so the ship-chain (build∧audit∧tdd) is Run==true here even
	// when those phases are absent from cfg.Mandatory — the non-configurable
	// integrity floor. A phase the advisor skipped/omitted is genuinely optional
	// this cycle (scout/triage included, when the operator shrinks the mandatory
	// set). Below Advisory, or with no plan, control falls through to the legacy
	// trigger path (byte-identical / fail-safe to static). The MaxInsertions cap
	// is intentionally not applied here: the plan is the advisor's coherent,
	// budget-aware whole-cycle selection, clamped by the floor rather than capped.
	if in.Cfg.Stage >= config.StageAdvisory && in.Plan != nil {
		runs := planRuns(in.Plan, phase)
		if runs && enable == config.EnableOff {
			// The (clamped) plan runs a phase the operator disabled via EnableOff.
			// The integrity floor or the advisor overrides the operator's off;
			// record a clamp so the ledger shows the override — mirroring the
			// mandatory-never-skipped clamp, never a silent discrepancy.
			return true, true, &Clamp{Rule: "floor-overrides-enable-off", Proposed: phase + "=off", Forced: phase + "=run"}
		}
		if runs && enable == config.EnableContent && optionalUsed >= in.Cfg.MaxInsertions && !isFloorPhase(phase) {
			return false, true, &Clamp{Rule: "max-insertions-cap", Proposed: phase + "=insert", Forced: phase + "=skip"}
		}
		return runs, true, nil
	}

	switch enable {
	case config.EnableOff:
		return false, true, nil
	case config.EnableOn:
		return true, true, nil
	default: // EnableContent → trigger-driven, subject to budget + insertion cap
		if in.BudgetRemaining <= 0 {
			return false, true, &Clamp{Rule: "budget-exhausted", Proposed: phase + "=insert", Forced: phase + "=skip"}
		}
		if optionalUsed >= in.Cfg.MaxInsertions {
			return false, true, &Clamp{Rule: "max-insertions-cap", Proposed: phase + "=insert", Forced: phase + "=skip"}
		}
		return triggerFires(in.Signals, in.Cfg.Triggers[phase]), true, nil
	}
}

// triggerFires evaluates a RoutingBlock's insert_when (OR) minus skip_when (OR).
func triggerFires(sig RoutingSignals, block config.RoutingBlock) bool {
	for _, c := range block.SkipWhen {
		if evalCondition(sig, c) {
			return false
		}
	}
	for _, c := range block.InsertWhen {
		if evalCondition(sig, c) {
			return true
		}
	}
	return false
}

// applyProposal adopts an LLM-proposed insert ONLY if it is the legal next phase
// the kernel already permits; any divergence is recorded as a clamp. The proposal
// can never introduce a mandatory-skip or a ship the objective signals don't support.
func applyProposal(d *RouterDecision, proposal *Proposal, in RouteInput) {
	if proposal == nil || proposal.NextPhase == "" {
		return
	}
	// Capture the advisor's rationale on the decision (whether or not it gets
	// clamped) so the recorded decision carries the would-have-routed reasoning.
	d.Justification = proposal.Justification
	want := normalize(proposal.NextPhase)
	if want == d.NextPhase {
		return // proposal agrees with the kernel; nothing to clamp
	}
	d.Clamps = append(d.Clamps, Clamp{
		Rule:     "llm-proposal-clamped",
		Proposed: proposal.NextPhase,
		Forced:   d.NextPhase,
	})
}

func enableOf(cfg config.RoutingConfig, phase string) config.Enable {
	if e, ok := cfg.PhaseEnable[phase]; ok {
		return e
	}
	if isMandatory(cfg, phase) {
		return config.EnableOn
	}
	return config.EnableContent
}

func isMandatory(cfg config.RoutingConfig, phase string) bool {
	for _, m := range cfg.Mandatory {
		if m == phase {
			return true
		}
	}
	return false
}

// countOptionalInserts counts completed phases that are neither mandatory nor a
// conditional key — i.e. optional inserts already spent against the cap.
func countOptionalInserts(cfg config.RoutingConfig, completed []string) int {
	n := 0
	for _, p := range completed {
		if isMandatory(cfg, p) {
			continue
		}
		if _, ok := cfg.Conditional[p]; ok {
			continue
		}
		switch normalize(p) {
		case "start", "intent", "scout", "end":
			continue // entry/discovery phases are not optional enrichment inserts
		}
		n++
	}
	return n
}

func reasonFor(in RouteInput, phase string, optional bool) string {
	if rule, ok := in.Cfg.Conditional[phase]; ok && evalCondRule(in.Signals, rule) {
		return "conditional-pin:" + phase
	}
	if isMandatory(in.Cfg, phase) {
		return "spine:" + phase
	}
	if enableOf(in.Cfg, phase) == config.EnableOn {
		return "forced-on:" + phase
	}
	// Plan-driven (Stage>=Advisory): the upfront whole-cycle plan scheduled this
	// phase, not a content trigger — name it so the ledger/forensics distinguish
	// advisor-planned from trigger-inserted.
	if in.Cfg.Stage >= config.StageAdvisory && in.Plan != nil && planRuns(in.Plan, phase) {
		return "plan:" + phase
	}
	return "content-insert:" + phase
}

// effectiveOrder is the phase sequence the walk advances through: the
// config-supplied Order (registry order, possibly with user phases spliced in)
// when present, else the built-in canonicalOrder. Keeping the fallback means a
// RoutingConfig built without a registry stays byte-identical to pre-Order behavior.
func effectiveOrder(cfg config.RoutingConfig) []string {
	if len(cfg.Order) > 0 {
		return cfg.Order
	}
	return canonicalOrder
}

func indexOfIn(order []string, phase string) int {
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return -1
}

// normalize folds the core.PhaseRetro alias "retro" to the canonical "retrospective".
func normalize(phase string) string {
	if phase == "retro" {
		return "retrospective"
	}
	return phase
}

func isFloorPhase(phase string) bool {
	switch phase {
	case "intent", "scout", "tdd", "build", "audit", "ship":
		return true
	default:
		return false
	}
}
