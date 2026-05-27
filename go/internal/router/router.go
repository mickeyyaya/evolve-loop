package router

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
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

	// Proposer context — populated by the orchestrator, consumed ONLY by a
	// DynamicLLM Proposer (which needs to dispatch a bridge call). The pure
	// Route() function ignores these, so determinism is preserved.
	Workspace   string
	ProjectRoot string
	Cycle       int
	Env         map[string]string
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
