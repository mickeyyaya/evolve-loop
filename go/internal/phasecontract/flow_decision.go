package phasecontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type FlowStage string

const (
	FlowStageLegacy  FlowStage = "legacy"
	FlowStageShadow  FlowStage = "shadow"
	FlowStageEnforce FlowStage = "enforce"
)

func ParseFlowStage(s string) (FlowStage, bool) {
	switch stage := FlowStage(s); stage {
	case "", FlowStageLegacy:
		return FlowStageLegacy, true
	case FlowStageShadow, FlowStageEnforce:
		return stage, true
	}
	return FlowStageLegacy, false
}

func FlowStageAt(projectRoot string, warn io.Writer) FlowStage {
	if projectRoot == "" {
		return FlowStageLegacy
	}
	pol, err := policy.Load(filepath.Join(paths.EvolveDirOf(projectRoot), "policy.json"))
	if err != nil {
		fmt.Fprintf(warn, "[isolation] WARN flow stage: %v (flow decisions stay at legacy)\n", err)
		return FlowStageLegacy
	}
	stage, ok := ParseFlowStage(pol.IsolationFlowStage())
	if !ok {
		fmt.Fprintf(warn, "[isolation] WARN unknown isolation.i8.stage %q (flow decisions stay at legacy)\n", pol.IsolationFlowStage())
	}
	return stage
}

const (
	FlowSeatPlan             = "advisor-plan"
	FlowSeatRePlan           = "advisor-replan"
	FlowSeatProposal         = "advisor-proposal"
	FlowSeatJudge            = "plan-judge"
	FlowSeatRetryAdjudicator = "retry-adjudicator"
	FlowSeatFailureAdvisor   = "failure-advisor"
	FlowSeatTriage           = "triage"
)

type FlowDecision struct {
	Seat         string
	ArtifactName string
	Shape        JSONShape
	Required     []string
	Allowed      []string
}

var planEntryKeys = []string{"phase", "run", "justification", "cli", "tier", "mint", "forced_by"}

var flowDecisions = []FlowDecision{
	{Seat: FlowSeatPlan, ArtifactName: "routing-plan.json", Shape: JSONShapeArray,
		Required: []string{"phase", "run"}, Allowed: planEntryKeys},
	{Seat: FlowSeatRePlan, ArtifactName: "routing-replan.json", Shape: JSONShapeArray,
		Required: []string{"phase", "run"}, Allowed: planEntryKeys},
	{Seat: FlowSeatProposal, ArtifactName: "routing-proposal.json", Shape: JSONShapeObject,
		Required: []string{"next_phase", "justification"},
		Allowed:  []string{"next_phase", "insert_phases", "justification", "learning_richness", "recovery_action"}},
	{Seat: FlowSeatJudge, ArtifactName: "routing-judge.json", Shape: JSONShapeObject,
		Required: []string{"score", "rationale"}, Allowed: []string{"score", "rationale", "missing_phases"}},
	{Seat: FlowSeatRetryAdjudicator, ArtifactName: "failure-adjudication.json", Shape: JSONShapeObject,
		Required: []string{"action", "justification"}, Allowed: []string{"action", "reentry_phase", "justification"}},
	{Seat: FlowSeatFailureAdvisor, ArtifactName: "failure-advice.json", Shape: JSONShapeObject,
		Required: []string{"cause", "pane_substr", "justification"}, Allowed: []string{"cause", "pane_substr", "justification"}},
	{Seat: FlowSeatTriage, ArtifactName: "triage-decision.json", Shape: JSONShapeObject,
		Required: []string{"top_n"},
		Allowed: []string{"cycle", "top_n", "deferred", "dropped", "superseded", "skip_shipped", "skip_rejected",
			"escalate_block", "committed_floors", "deferred_floors", "phase_skip", "unified_commitment",
			"unified_projection", "projected_by_orchestrator"}},
}

func FlowDecisions() []FlowDecision { return slices.Clone(flowDecisions) }

func FlowDecisionFor(seat string) (FlowDecision, bool) {
	i := slices.IndexFunc(flowDecisions, func(d FlowDecision) bool { return d.Seat == seat })
	if i < 0 {
		return FlowDecision{}, false
	}
	return flowDecisions[i], true
}

func MustFlowDecision(seat string) FlowDecision {
	d, ok := FlowDecisionFor(seat)
	if !ok {
		panic("phasecontract: no flow decision for seat " + seat)
	}
	return d
}

func (d FlowDecision) Resolve(projectRoot string, cell FlowCell, legacy func() ([]byte, bool), signals *signalcenter.Center) []byte {
	r := d.Read(FlowStageAt(projectRoot, os.Stderr), cell.Path, legacy)
	if e, ok := d.ViolationEvent(r, cell); ok {
		signals.Emit(e)
	}
	return r.Raw
}

func (d FlowDecision) Validate(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var v any
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("%s: %w", d.ArtifactName, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: trailing data after the JSON value", d.ArtifactName)
	}
	if d.Shape != JSONShapeArray {
		return d.checkObject(v, "the file")
	}
	items, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%s: the file is not an array", d.ArtifactName)
	}
	for i, item := range items {
		if err := d.checkObject(item, fmt.Sprintf("element %d", i)); err != nil {
			return err
		}
	}
	return nil
}

func (d FlowDecision) checkObject(v any, at string) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: %s is not an object", d.ArtifactName, at)
	}
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		if !slices.Contains(d.Allowed, key) {
			return fmt.Errorf("%s: %s has unknown field %q", d.ArtifactName, at, key)
		}
	}
	for _, key := range d.Required {
		if _, ok := obj[key]; !ok {
			return fmt.Errorf("%s: %s has missing field %q", d.ArtifactName, at, key)
		}
	}
	return nil
}

const (
	FlowSourceFile   = "file"
	FlowSourceStdout = "stdout"
)

type FlowRead struct {
	Raw      []byte
	Source   string
	Stage    FlowStage
	FileErr  error
	Disagree bool
}

func (d FlowDecision) Read(stage FlowStage, path string, legacy func() ([]byte, bool)) FlowRead {
	file, fileErr := d.readFile(path)
	r := FlowRead{Stage: stage, FileErr: fileErr}
	if fileErr == nil {
		r.Raw, r.Source = file, FlowSourceFile
	}
	if stage == FlowStageEnforce || (stage == FlowStageLegacy && fileErr == nil) {
		return r
	}
	span, ok := legacy()
	if ok {
		r.Raw, r.Source = span, FlowSourceStdout
	}
	if stage == FlowStageShadow {
		r.Disagree = (fileErr == nil) != ok || (ok && !sameJSON(file, span))
	}
	return r
}

func (d FlowDecision) readFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := d.Validate(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func sameJSON(a, b []byte) bool {
	var va, vb any
	return json.Unmarshal(a, &va) == nil && json.Unmarshal(b, &vb) == nil && reflect.DeepEqual(va, vb)
}

const (
	FlowRuleStdoutFallback = "flow.fallback.stdout"
	FlowRuleShadowDisagree = "flow.shadow.disagree"
	FlowRuleFileInvalid    = "flow.file.invalid"
	FlowRuleProseFallback  = "flow.fallback.prose"
)

type FlowCell struct {
	Cycle  int
	Phase  string
	Origin string
	Path   string
}

func (d FlowDecision) ViolationEvent(r FlowRead, cell FlowCell) (signalcenter.Event, bool) {
	action, rule, reason := "logged", "", ""
	switch {
	case r.Stage == FlowStageEnforce && r.FileErr != nil:
		action, rule = "refused", FlowRuleFileInvalid
		reason = fmt.Sprintf("%s refused the decision: %s is missing or fails its schema (%v)", d.Seat, d.ArtifactName, r.FileErr)
	case r.Stage == FlowStageShadow && r.Disagree:
		rule = FlowRuleShadowDisagree
		reason = fmt.Sprintf("%s: %s and the stdout span disagree", d.Seat, d.ArtifactName)
	case r.Stage == FlowStageLegacy && r.Source == FlowSourceStdout:
		rule = FlowRuleStdoutFallback
		reason = fmt.Sprintf("%s took its decision from stdout: %s is missing or fails its schema (%v)", d.Seat, d.ArtifactName, r.FileErr)
	default:
		return signalcenter.Event{}, false
	}
	return FlowViolationEvent(FlowViolation{Seat: d.Seat, Stage: r.Stage, Action: action, Rule: rule, Reason: reason}, cell), true
}

type FlowViolation struct {
	Seat   string
	Stage  FlowStage
	Action string
	Rule   string
	Reason string
}

func FlowViolationEvent(v FlowViolation, cell FlowCell) signalcenter.Event {
	return signalcenter.Event{
		Cycle: cell.Cycle, Phase: cell.Phase, Module: signalcenter.ModuleIsolation, Origin: cell.Origin,
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(8),
		Reason: v.Reason,
		Fields: map[string]string{
			"invariant": "I8", "stage": string(v.Stage), "action": v.Action, "cell": v.Seat,
			"target": cell.Path, "rule": v.Rule, "source": "parser",
		},
	}
}
