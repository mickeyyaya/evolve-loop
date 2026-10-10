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

var canonicalOrder = []string{
	"intent", "scout", "triage", "plan-review",
	"tdd", "build-planner", "build", "tester",
	"audit", "ship", "retrospective", "memo",
}

const PhaseEnd = "end"

type RouteInput struct {
	Current        string
	Verdict        string
	Signals        RoutingSignals
	History        []failureadapter.Entry
	Cfg            config.RoutingConfig
	Completed      []string
	Strict         bool
	Now            time.Time
	IntentRequired bool
	PSMASEnabled   bool

	Workspace      string
	ProjectRoot    string
	ActiveWorktree string
	Cycle          int
	Env            map[string]string

	BenchedCLIs       []BenchedCLI
	UnavailablePhases []string

	GoalText string

	LaneItems []LaneItem

	CarryoverTodos []CarryoverTodo

	Catalog        []PhaseCard
	OnDemandPhases []string

	LastReason string
	Lessons    []string

	Plan *PhasePlan

	Blocker *Blocker
}

type PhaseCard struct {
	Name              string                      `json:"name"`
	Role              string                      `json:"role"`
	Tier              string                      `json:"tier,omitempty"`
	WritesSource      bool                        `json:"writes_source,omitempty"`
	Optional          bool                        `json:"optional,omitempty"`
	Description       string                      `json:"description,omitempty"`
	WhenToUse         string                      `json:"when_to_use,omitempty"`
	Categories        []string                    `json:"categories,omitempty"`
	AllowedCLIs       []string                    `json:"allowed_clis,omitempty"`
	ModelTierEnvelope *profiles.ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
}

type CarryoverTodo struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Priority       string `json:"priority"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
}

type Clamp struct {
	Phase    string `json:"phase,omitempty"`
	Rule     string `json:"rule"`
	Proposed string `json:"proposed"`
	Forced   string `json:"forced"`
}

type RouterDecision struct {
	NextPhase     string                 `json:"next_phase"`
	InsertPhases  []string               `json:"insert_phases,omitempty"`
	SkipPhases    []string               `json:"skip_phases,omitempty"`
	Reason        string                 `json:"reason"`
	Evidence      map[string]interface{} `json:"evidence,omitempty"`
	Clamps        []Clamp                `json:"clamps,omitempty"`
	Justification string                 `json:"justification,omitempty"`
}

type Proposal struct {
	NextPhase        string   `json:"next_phase"`
	InsertPhases     []string `json:"insert_phases"`
	Justification    string   `json:"justification"`
	LearningRichness string   `json:"learning_richness,omitempty"`
	RecoveryAction   string   `json:"recovery_action,omitempty"`
}

type PhasePlanEntry struct {
	Phase         string    `json:"phase"`
	Run           bool      `json:"run"`
	Justification string    `json:"justification,omitempty"`
	CLI           string    `json:"cli,omitempty"`
	Tier          string    `json:"tier,omitempty"`
	Mint          *MintSpec `json:"mint,omitempty"`
}

type MintSpec struct {
	Prompt       string `json:"prompt"`
	Tier         string `json:"tier,omitempty"`
	CLI          string `json:"cli,omitempty"`
	Description  string `json:"description,omitempty"`
	WhenToUse    string `json:"when_to_use,omitempty"`
	WritesSource *bool  `json:"writes_source,omitempty"`
}

type PhasePlan struct {
	Entries    []PhasePlanEntry
	MintPhases []phaseconfig.PhaseConfig
}

func Route(in RouteInput, proposal *Proposal) RouterDecision {
	cur := normalize(in.Current)

	if cur == "retrospective" {
		return retroDecision(in, proposal)
	}

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

	if cur == "audit" {
		if in.Verdict == "FAIL" {
			d := RouterDecision{
				NextPhase: "retrospective",
				Reason:    "audit-fail-to-retrospective",
				Evidence:  map[string]interface{}{"verdict": in.Verdict},
			}
			if in.Cfg.AuditFailRoutesTo != "" {
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

func retroDecision(in RouteInput, proposal *Proposal) RouterDecision {
	dec := failureadapter.Decide(in.History, failureadapter.Options{Now: in.Now, Strict: in.Strict})
	d := RouterDecision{
		Reason:   "retro:" + string(dec.Action),
		Evidence: map[string]interface{}{"action": string(dec.Action)},
	}
	d.SkipPhases = append(d.SkipPhases, dec.SkipPhases...)
	d.NextPhase = PhaseEnd
	if dec.Action == failureadapter.ActionRetryWithFallback {
		d.NextPhase = "tdd"
	}
	applyFailureProposal(&d, proposal, dec.Action)
	return d
}

var failureInsertPhases = map[string]struct{}{
	"fault-localization": {},
	"bug-reproduction":   {},
}

func applyFailureProposal(d *RouterDecision, proposal *Proposal, action failureadapter.Action) {
	if proposal == nil || proposal.RecoveryAction == "" {
		return
	}
	if proposal.Justification != "" {
		d.Justification = proposal.Justification
	}
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

func IsFailureInsert(phase string) bool {
	_, ok := failureInsertPhases[phase]
	return ok
}

func FailureInsertPhases() []string {
	out := make([]string, 0, len(failureInsertPhases))
	for p := range failureInsertPhases {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func applyLearningRichness(d *RouterDecision, proposal *Proposal, in RouteInput) {
	if proposal == nil || proposal.LearningRichness != "memo" {
		return
	}
	d.Evidence["learning_richness"] = proposal.LearningRichness
	if d.NextPhase == "memo" {
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
	start := indexOfIn(order, normalize(in.Current))

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

func shouldRun(in RouteInput, phase string, optionalUsed int) (bool, bool, *Clamp) {
	enable := enableOf(in.Cfg, phase)

	if run, clamp, pinned := mandatoryPhaseRun(in, phase, enable); pinned {
		return run, false, clamp
	}

	if slices.Contains(in.UnavailablePhases, phase) {
		return false, true, &Clamp{Phase: phase, Rule: DropUnavailablePhaseRule, Proposed: phase + "=insert", Forced: phase + "=skip"}
	}

	if in.Cfg.Stage >= config.StageAdvisory && in.Plan != nil {
		return shouldRunFromPlan(in, phase, enable, optionalUsed)
	}

	switch enable {
	case config.EnableOff:
		return false, true, nil
	case config.EnableOn:
		return true, true, nil
	default:
		if optionalUsed >= in.Cfg.MaxInsertions {
			return false, true, &Clamp{Rule: "max-insertions-cap", Proposed: phase + "=insert", Forced: phase + "=skip"}
		}
		return triggerFires(in.Signals, in.Cfg.Triggers[phase]), true, nil
	}
}

func mandatoryPhaseRun(in RouteInput, phase string, enable config.Enable) (bool, *Clamp, bool) {
	if isMandatory(in.Cfg, phase) {
		if enable == config.EnableOff {
			return true, &Clamp{Rule: "mandatory-never-skipped", Proposed: phase + "=off", Forced: phase + "=on"}, true
		}
		return true, nil, true
	}
	if rule, ok := in.Cfg.Conditional[phase]; ok && evalCondRule(in.Signals, rule) {
		if enable == config.EnableOff {
			return true, &Clamp{Rule: "conditional-mandatory-pin", Proposed: phase + "=off", Forced: phase + "=on"}, true
		}
		return true, nil, true
	}
	return false, nil, false
}

const (
	RuleSkipWhenGatesPlan   = "skip-when-gates-plan"
	RuleInsertWhenGatesPlan = "insert-when-gates-plan"
)

func shouldRunFromPlan(in RouteInput, phase string, enable config.Enable, optionalUsed int) (bool, bool, *Clamp) {
	runs := planRuns(in.Plan, phase)
	block := in.Cfg.Triggers[phase]
	if runs && !isFloorPhase(phase) && skipWhenFires(in.Signals, block) {
		return false, true, &Clamp{Phase: phase, Rule: RuleSkipWhenGatesPlan, Proposed: phase + "=run", Forced: phase + "=skip"}
	}
	if runs && insertWhenGatesPlan(in.Signals, phase, enable, block) {
		return false, true, &Clamp{Phase: phase, Rule: RuleInsertWhenGatesPlan, Proposed: phase + "=run", Forced: phase + "=skip"}
	}
	if runs && enable == config.EnableOff {
		return true, true, &Clamp{Rule: "floor-overrides-enable-off", Proposed: phase + "=off", Forced: phase + "=run"}
	}
	if runs && enable == config.EnableContent && optionalUsed >= in.Cfg.MaxInsertions && !isFloorPhase(phase) {
		return false, true, &Clamp{Rule: "max-insertions-cap", Proposed: phase + "=insert", Forced: phase + "=skip"}
	}
	return runs, true, nil
}

func skipWhenFires(sig RoutingSignals, block config.RoutingBlock) bool {
	for _, c := range block.SkipWhen {
		if evalCondition(sig, c) {
			return true
		}
	}
	return false
}

func insertWhenGatesPlan(sig RoutingSignals, phase string, enable config.Enable, block config.RoutingBlock) bool {
	return enable == config.EnableContent && !isFloorPhase(phase) && block.TriggerIsTheWholeRule() && !triggerFires(sig, block)
}

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
			continue
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
	if in.Cfg.Stage >= config.StageAdvisory && in.Plan != nil && planRuns(in.Plan, phase) {
		return "plan:" + phase
	}
	if enableOf(in.Cfg, phase) == config.EnableOn {
		return "forced-on:" + phase
	}
	return "content-insert:" + phase
}

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

type LaneItem struct {
	ID              string
	Kind            string
	DeliverableKind string
	Acceptance      []string
	Unresolved      string
}

type BenchedCLI struct {
	Family string
	Reason string
	Until  time.Time
}
