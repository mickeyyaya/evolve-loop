package router

import (
	"slices"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// canonicalOrder is the walk order when the config supplies none.
var canonicalOrder = []string{
	"intent", "scout", "triage", "plan-review",
	"tdd", "build-planner", "build", "tester",
	"audit", "ship", "retrospective", "memo",
}

// PhaseEnd is the terminal sentinel Route returns when no phase remains.
const PhaseEnd = "end"

// RouteInput is the complete, pre-digested context for one routing decision.
type RouteInput struct {
	Current        string // "start" on cycle entry
	Verdict        string
	Signals        RoutingSignals
	History        []failureadapter.Entry // state.failedApproaches, converted
	Cfg            config.RoutingConfig
	Completed      []string
	Strict         bool // policy.json workflow.strict_audit, passed to failureadapter
	Now            time.Time
	IntentRequired bool
	PSMASEnabled   bool // consume triage phase_skip[]

	// Workspace through Env are the advisor's bridge-launch context; the pure Route ignores them.
	Workspace   string
	ProjectRoot string
	// ActiveWorktree is required under EVOLVE_FLEET=1: the tmux driver refuses an empty worktree.
	ActiveWorktree string
	Cycle          int
	Env            map[string]string

	// BenchedCLIs is advisor prompt context only, like GoalText, CarryoverTodos, LastReason and Lessons.
	BenchedCLIs []BenchedCLI
	// UnavailablePhases are catalog-optional phases whose persona doc is absent; the walk and the
	// floor clamp drop them. Mandatory and floor phases are never listed: their absence must fail loudly.
	UnavailablePhases []string

	GoalText string

	CarryoverTodos []CarryoverTodo

	// Catalog is the set of pre-defined phases the advisor may select instead of minting.
	Catalog []PhaseCard
	// OnDemandPhases are installed phases that declined a SELECT slot (phasespec.CatalogOnDemand).
	// They travel with Catalog: the menu and what is off it are one statement.
	OnDemandPhases []string

	// LastReason and Lessons are recall context; empty means a novel failure.
	LastReason string
	Lessons    []string

	// Plan is the advisor's whole-cycle plan, already floor-clamped by the caller.
	// At Stage>=Advisory it drives non-mandatory phases; nil keeps the trigger path.
	Plan *PhasePlan

	// Blocker is the ship failure Recover routes; nil for ordinary routing.
	Blocker *Blocker
}

// PhaseCard is the advisor-facing projection of one pre-defined phase.
type PhaseCard struct {
	Name         string   `json:"name"`
	Role         string   `json:"role"` // plan|build|evaluate|control
	Tier         string   `json:"tier,omitempty"`
	WritesSource bool     `json:"writes_source,omitempty"`
	Optional     bool     `json:"optional,omitempty"`    // selectable, not spine
	Description  string   `json:"description,omitempty"` // one line: what the phase produces
	WhenToUse    string   `json:"when_to_use,omitempty"` // the SELECT hint
	Categories   []string `json:"categories,omitempty"`  // goal types
	// AllowedCLIs and ModelTierEnvelope project the phase's profile guardrails so the
	// advisor proposes in bounds; ClampPlanModelRouting re-validates regardless.
	AllowedCLIs       []string                    `json:"allowed_clis,omitempty"`
	ModelTierEnvelope *profiles.ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
}

// CarryoverTodo mirrors core.CarryoverTodo for the advisor without importing core.
type CarryoverTodo struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Priority       string `json:"priority"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
}

// Clamp records a hard rule overriding a proposed decision; Phase is empty for a whole-plan clamp.
type Clamp struct {
	Phase    string `json:"phase,omitempty"`
	Rule     string `json:"rule"`
	Proposed string `json:"proposed"`
	Forced   string `json:"forced"`
}

// RouterDecision is the next phase to run plus the inserts, skips and rule that led to it.
type RouterDecision struct {
	NextPhase    string                 `json:"next_phase"`
	InsertPhases []string               `json:"insert_phases,omitempty"`
	SkipPhases   []string               `json:"skip_phases,omitempty"`
	Reason       string                 `json:"reason"`
	Evidence     map[string]interface{} `json:"evidence,omitempty"`
	Clamps       []Clamp                `json:"clamps,omitempty"`
	// Justification is the advisor's rationale, captured even when its proposal is clamped.
	Justification string `json:"justification,omitempty"`
}

// Proposal is the advisor's per-transition advice; Route treats it as advisory and clamps it.
type Proposal struct {
	NextPhase     string   `json:"next_phase"`
	InsertPhases  []string `json:"insert_phases"`
	Justification string   `json:"justification"`
	// LearningRichness ("memo") picks the learning phase after an audit FAIL; RecoveryAction
	// ("retry"|"end") picks the post-retro branch unless the failure adapter blocks.
	LearningRichness string `json:"learning_richness,omitempty"`
	RecoveryAction   string `json:"recovery_action,omitempty"`
}

// PhasePlanEntry is one phase's run/skip decision in the whole-cycle plan.
type PhasePlanEntry struct {
	Phase         string `json:"phase"`
	Run           bool   `json:"run"`
	Justification string `json:"justification,omitempty"`
	// CLI and Tier are the advisor's proposed dispatch CLI and abstract tier, never a raw
	// model. ClampPlanModelRouting re-validates both before dispatch.
	CLI  string `json:"cli,omitempty"`
	Tier string `json:"tier,omitempty"`
	// Mint marks a new phase absent from the catalog; nil for an existing phase.
	Mint *MintSpec `json:"mint,omitempty"`
}

// MintSpec is the advisor-authorable subset of a minted phase: its prompt, tier and CLI.
type MintSpec struct {
	Prompt string `json:"prompt"`
	Tier   string `json:"tier,omitempty"`
	CLI    string `json:"cli,omitempty"`
	// Description and WhenToUse use phasespec.PhaseSpec's json keys so the minter needs no translation.
	Description string `json:"description,omitempty"`
	WhenToUse   string `json:"when_to_use,omitempty"`
	// WritesSource is tri-state so omitted advisor output can take the safe
	// minted-phase default while an explicit false remains a read-only opt-out.
	WritesSource *bool `json:"writes_source,omitempty"`
}

// PhasePlan is the advisor's whole-cycle plan; the floor clamp re-validates it before any phase runs.
// Entries has no json tag because callers serialize plan.Entries as a bare array, the form the advisor emits.
type PhasePlan struct {
	Entries []PhasePlanEntry
	// MintPhases are new phases the orchestrator registers through the trust-kernel clamp at cycle start.
	MintPhases []phaseconfig.PhaseConfig
}

// Route computes the routing decision from ordered rules; the clamp pass runs last.
func Route(in RouteInput, proposal *Proposal) RouterDecision {
	cur := normalize(in.Current)

	// Defer to the failure adapter exactly as the orchestrator's decideAfterRetro does.
	if cur == "retrospective" {
		return retroDecision(in, proposal)
	}

	// Triage owns the cycle's task commitment. A failed contract or an explicit
	// empty top_n has no authorized work for downstream implementation phases.
	if cur == "triage" && in.Verdict == "FAIL" {
		return RouterDecision{
			NextPhase: PhaseEnd,
			Reason:    "triage-fail",
			Evidence:  map[string]interface{}{"verdict": in.Verdict},
		}
	}
	if cur == "triage" && in.Signals.HasEmptyTriageCommitment() {
		return RouterDecision{
			NextPhase: PhaseEnd,
			Reason:    "triage-empty-top-n",
			Evidence:  map[string]interface{}{"committed_count": 0},
		}
	}

	// Audit FAIL never proceeds to ship; it diverts to a learning phase.
	if cur == "audit" {
		if in.Verdict == "FAIL" {
			d := RouterDecision{
				NextPhase: "retrospective",
				Reason:    "audit-fail-to-retrospective",
				Evidence:  map[string]interface{}{"verdict": in.Verdict},
			}
			if in.Cfg.AuditFailRoutesTo != "" {
				// policy.json:failure_floor supersedes the deprecated enable chain.
				d.NextPhase = in.Cfg.AuditFailRoutesTo
				d.Reason = "audit-fail-to-" + in.Cfg.AuditFailRoutesTo
			} else if enableOf(in.Cfg, "retrospective") == config.EnableOff {
				d.NextPhase = PhaseEnd
			}
			applyLearningRichness(&d, proposal, in)
			return d
		}
	}

	return walk(in, proposal)
}

// retroDecision applies failureadapter.Decide, then the advisor's failure proposal above it.
func retroDecision(in RouteInput, proposal *Proposal) RouterDecision {
	dec := failureadapter.Decide(in.History, failureadapter.Options{Now: in.Now, Strict: in.Strict})
	d := RouterDecision{
		Reason:   "retro:" + string(dec.Action),
		Evidence: map[string]interface{}{"action": string(dec.Action)},
	}
	d.SkipPhases = append(d.SkipPhases, dec.SkipPhases...)
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		d.NextPhase = "tdd"
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		d.NextPhase = PhaseEnd
	default: // PROCEED
		d.NextPhase = PhaseEnd
	}
	applyFailureProposal(&d, proposal, dec.Action)
	return d
}

// failureInsertPhases may be inserted ahead of a retry; both precede tdd, so the walk continues into it.
var failureInsertPhases = map[string]struct{}{
	"fault-localization": {},
	"bug-reproduction":   {},
}

// applyFailureProposal adopts the advisor's RecoveryAction and optional failure-scoped insert
// unless the failure adapter blocks; a blocked override is recorded as a clamp.
func applyFailureProposal(d *RouterDecision, proposal *Proposal, action failureadapter.Action) {
	if proposal == nil || proposal.RecoveryAction == "" {
		return
	}
	if proposal.Justification != "" {
		d.Justification = proposal.Justification
	}
	// Validate before recording: an unknown action must not appear in the evidence.
	if proposal.RecoveryAction != "retry" && proposal.RecoveryAction != "end" {
		d.Clamps = append(d.Clamps, Clamp{
			Rule:     "failure-proposal-clamped",
			Proposed: proposal.RecoveryAction,
			Forced:   d.NextPhase,
		})
		return
	}
	d.Evidence["recovery_action"] = proposal.RecoveryAction

	blocked := action == failureadapter.ActionBlockCode || action == failureadapter.ActionBlockOperatorAction
	want := PhaseEnd
	if proposal.RecoveryAction == "retry" {
		want = "tdd"
		if len(proposal.InsertPhases) > 0 {
			if p := normalize(proposal.InsertPhases[0]); IsFailureInsert(p) {
				want = p
			}
		}
	}
	if blocked {
		if want != d.NextPhase {
			d.Clamps = append(d.Clamps, Clamp{
				Rule:     "failure-proposal-clamped",
				Proposed: want,
				Forced:   d.NextPhase,
			})
		}
		return
	}
	d.NextPhase = want
}

// IsFailureInsert reports whether phase is a failure-scoped insert an advisor may schedule ahead of a retry.
func IsFailureInsert(phase string) bool {
	_, ok := failureInsertPhases[phase]
	return ok
}

// FailureInsertPhases returns the retry-path insert phases, sorted; the advisor prompt renders from it.
func FailureInsertPhases() []string {
	out := make([]string, 0, len(failureInsertPhases))
	for p := range failureInsertPhases {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// applyLearningRichness lets the advisor pick memo over the retrospective after an audit FAIL;
// some learning phase always runs, so a memo choice that cannot apply is clamped.
func applyLearningRichness(d *RouterDecision, proposal *Proposal, in RouteInput) {
	if proposal == nil || proposal.LearningRichness != "memo" {
		return
	}
	d.Evidence["learning_richness"] = proposal.LearningRichness
	if d.NextPhase == "memo" {
		// Policy already routed memo; nothing was forced, so no clamp.
		return
	}
	if d.NextPhase != "retrospective" || enableOf(in.Cfg, "memo") == config.EnableOff {
		d.Clamps = append(d.Clamps, Clamp{
			Rule:     "failure-proposal-clamped",
			Proposed: "memo",
			Forced:   d.NextPhase,
		})
		return
	}
	d.NextPhase = "memo"
	d.Reason = "audit-fail-to-memo"
}

func walk(in RouteInput, proposal *Proposal) RouterDecision {
	d := RouterDecision{Evidence: map[string]interface{}{}}
	done := toSet(in.Completed)
	psmasSkip := psmasSkipSet(in)
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
		if optional && psmasSkip[phase] {
			d.SkipPhases = append(d.SkipPhases, phase)
			continue
		}
		if optional {
			d.InsertPhases = append(d.InsertPhases, phase)
		}
		d.NextPhase = phase
		d.Reason = reasonFor(in, phase, optional)
		applyProposal(&d, proposal, in)
		return d
	}

	d.NextPhase = PhaseEnd
	d.Reason = "no-runnable-phase-remaining"
	applyProposal(&d, proposal, in)
	return d
}

func psmasSkipSet(in RouteInput) map[string]bool {
	out := map[string]bool{}
	if !in.PSMASEnabled || !in.Signals.Triage.Present {
		return out
	}
	for _, phase := range in.Signals.Triage.PhaseSkip {
		if p := normalize(phase); p != "" {
			out[p] = true
		}
	}
	return out
}

// shouldRun returns (run, optional, clamp); clamp is non-nil when a hard rule overrode the soft decision.
func shouldRun(in RouteInput, phase string, optionalUsed int) (bool, bool, *Clamp) {
	enable := enableOf(in.Cfg, phase)

	if isMandatory(in.Cfg, phase) {
		if enable == config.EnableOff {
			return true, false, &Clamp{Rule: "mandatory-never-skipped", Proposed: phase + "=off", Forced: phase + "=on"}
		}
		return true, false, nil
	}

	if rule, ok := in.Cfg.Conditional[phase]; ok {
		if evalCondRule(in.Signals, rule) {
			if enable == config.EnableOff {
				return true, false, &Clamp{Rule: "conditional-mandatory-pin", Proposed: phase + "=off", Forced: phase + "=on"}
			}
			return true, false, nil
		}
	}

	// A phase whose persona doc is absent would only dispatch a skip. Core lists
	// only optional, non-floor phases here.
	if slices.Contains(in.UnavailablePhases, phase) {
		return false, true, &Clamp{Phase: phase, Rule: DropUnavailablePhaseRule, Proposed: phase + "=insert", Forced: phase + "=skip"}
	}

	// At Advisory and above, the pre-clamped plan drives every non-mandatory
	// phase; below that, or with no plan, the trigger path runs.
	if in.Cfg.Stage >= config.StageAdvisory && in.Plan != nil {
		runs := planRuns(in.Plan, phase)
		// A configured skip_when gates the plan. Floor phases are exempt so the
		// gate can never bypass the floor.
		if runs && !isFloorPhase(phase) && skipWhenFires(in.Signals, in.Cfg.Triggers[phase]) {
			return false, true, &Clamp{Rule: "skip-when-gates-plan", Proposed: phase + "=run", Forced: phase + "=skip"}
		}
		if runs && enable == config.EnableOff {
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
	default: // EnableContent: trigger-driven, under the insertion cap
		if optionalUsed >= in.Cfg.MaxInsertions {
			return false, true, &Clamp{Rule: "max-insertions-cap", Proposed: phase + "=insert", Forced: phase + "=skip"}
		}
		return triggerFires(in.Signals, in.Cfg.Triggers[phase]), true, nil
	}
}

// skipWhenFires reports whether any skip_when clause holds; the trigger path and the plan gate share it.
func skipWhenFires(sig RoutingSignals, block config.RoutingBlock) bool {
	for _, c := range block.SkipWhen {
		if evalCondition(sig, c) {
			return true
		}
	}
	return false
}

// triggerFires evaluates a RoutingBlock's insert_when (OR) minus skip_when (OR).
func triggerFires(sig RoutingSignals, block config.RoutingBlock) bool {
	if skipWhenFires(sig, block) {
		return false
	}
	for _, c := range block.InsertWhen {
		if evalCondition(sig, c) {
			return true
		}
	}
	return false
}

// applyProposal records the advisor's rationale and clamps any divergence from the kernel's
// next phase; it never changes NextPhase.
func applyProposal(d *RouterDecision, proposal *Proposal, in RouteInput) {
	if proposal == nil || proposal.NextPhase == "" {
		return
	}
	d.Justification = proposal.Justification
	want := normalize(proposal.NextPhase)
	if want == d.NextPhase {
		return
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

// countOptionalInserts counts the optional inserts already spent against the cap.
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
	// Name plan-driven phases so forensics tell advisor-planned from trigger-inserted.
	if in.Cfg.Stage >= config.StageAdvisory && in.Plan != nil && planRuns(in.Plan, phase) {
		return "plan:" + phase
	}
	return "content-insert:" + phase
}

// effectiveOrder is cfg.Order (registry order with user phases spliced in) when set, else canonicalOrder.
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

// normalize folds the aliases "retro" and "tdd-engineer" to their canonical phase names.
func normalize(phase string) string {
	switch phase {
	case "retro":
		return "retrospective"
	case "tdd-engineer":
		return "tdd"
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

// BenchedCLI is one CLI family the cli-health store has benched, carried as advisor context.
type BenchedCLI struct {
	Family string    // e.g. "codex"
	Reason string    // classifier pattern, e.g. "rate_limit"
	Until  time.Time // the canary re-probes after this
}
