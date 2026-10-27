package phasecontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

const (
	DiagnosisSchema              = "evolve.diagnosis/1"
	FlowSeatSelfhealOrchestrator = "selfheal-orchestrator"
)

const (
	ActionRetryWithVariation = "retry-with-variation"
	ActionRerouteFamily      = "reroute-family"
	ActionQuarantineItem     = "quarantine-item"
	ActionRevertLastLanding  = "revert-last-landing"
	ActionOpenRepairItem     = "open-repair-item"
	ActionHaltAndEscalate    = "halt-and-escalate"
	ActionDispatchPhaseAgent = "dispatch-phase-agent"
)

const (
	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

const (
	paramItem      = "item"
	paramVariation = "variation"
	paramFamily    = "family"
	paramSHA       = "sha"
	paramFiles     = "files"
	paramRepro     = "repro"
	paramReason    = "reason"
	paramRequest   = "request"
)

type catalogAction struct {
	kind     string
	required []string
	optional []string
}

var diagnosisCatalog = []catalogAction{
	{kind: ActionRetryWithVariation, required: []string{paramItem, paramVariation}},
	{kind: ActionRerouteFamily, required: []string{paramItem, paramFamily}},
	{kind: ActionQuarantineItem, required: []string{paramItem}, optional: []string{paramReason}},
	{kind: ActionRevertLastLanding, required: []string{paramSHA}},
	{kind: ActionOpenRepairItem, required: []string{paramFiles, paramRepro}},
	{kind: ActionHaltAndEscalate, optional: []string{paramReason}},
	{kind: ActionDispatchPhaseAgent, required: []string{paramRequest}},
}

var (
	variations   = []string{"model", "tier", "prompt-seed"}
	confidences  = []string{ConfidenceLow, ConfidenceMedium, ConfidenceHigh}
	shaPattern   = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	errDiagnosis = errors.New("diagnosis.json")
)

type DiagnosisParams struct {
	Item      string   `json:"item,omitempty"`
	Variation string   `json:"variation,omitempty"`
	Family    string   `json:"family,omitempty"`
	SHA       string   `json:"sha,omitempty"`
	Files     []string `json:"files,omitempty"`
	Repro     string   `json:"repro,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Request   string   `json:"request,omitempty"`
}

type DiagnosisAction struct {
	Kind   string          `json:"kind"`
	Params DiagnosisParams `json:"params"`
}

type Diagnosis struct {
	Schema         string           `json:"schema"`
	IncidentID     string           `json:"incident_id"`
	Hypothesis     string           `json:"hypothesis"`
	Evidence       []string         `json:"evidence"`
	Action         DiagnosisAction  `json:"action"`
	Alternative    *DiagnosisAction `json:"alternative,omitempty"`
	ExpectedEffect string           `json:"expected_effect"`
	Confidence     string           `json:"confidence"`
}

func DiagnosisActions() []string {
	out := make([]string, 0, len(diagnosisCatalog))
	for _, a := range diagnosisCatalog {
		out = append(out, a.kind)
	}
	return out
}

func ParseDiagnosis(raw []byte, incidentID string, evidence []string) (Diagnosis, error) {
	if err := MustFlowDecision(FlowSeatSelfhealOrchestrator).Validate(raw); err != nil {
		return Diagnosis{}, err
	}
	var d Diagnosis
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Diagnosis{}, fmt.Errorf("%w: %w", errDiagnosis, err)
	}
	if err := d.check(incidentID, evidence); err != nil {
		return Diagnosis{}, fmt.Errorf("%w: %w", errDiagnosis, err)
	}
	return d, nil
}

func (d Diagnosis) check(incidentID string, evidence []string) error {
	switch {
	case d.Schema != DiagnosisSchema:
		return fmt.Errorf("schema %q is not %s", d.Schema, DiagnosisSchema)
	case d.IncidentID != incidentID:
		return fmt.Errorf("incident_id %q is not the incident %s", d.IncidentID, incidentID)
	case strings.TrimSpace(d.Hypothesis) == "":
		return errors.New("the hypothesis is empty")
	case strings.TrimSpace(d.ExpectedEffect) == "":
		return errors.New("the expected_effect is empty")
	case !slices.Contains(confidences, d.Confidence):
		return fmt.Errorf("confidence %q is not one of %v", d.Confidence, confidences)
	case len(d.Evidence) == 0:
		return errors.New("the diagnosis names no evidence")
	}
	for _, id := range d.Evidence {
		if !slices.Contains(evidence, id) {
			return fmt.Errorf("evidence %q is not an entry of the evidence index", id)
		}
	}
	if err := d.Action.check(); err != nil {
		return err
	}
	if d.Alternative != nil {
		if err := d.Alternative.check(); err != nil {
			return fmt.Errorf("alternative: %w", err)
		}
	}
	return nil
}

func (a DiagnosisAction) check() error {
	i := slices.IndexFunc(diagnosisCatalog, func(c catalogAction) bool { return c.kind == a.Kind })
	if i < 0 {
		return fmt.Errorf("action kind %q is not in the catalog %v", a.Kind, DiagnosisActions())
	}
	spec, set := diagnosisCatalog[i], a.Params.set()
	for _, name := range spec.required {
		if !slices.Contains(set, name) {
			return fmt.Errorf("%s needs the param %q", a.Kind, name)
		}
	}
	for _, name := range set {
		if !slices.Contains(spec.required, name) && !slices.Contains(spec.optional, name) {
			return fmt.Errorf("%s does not take the param %q", a.Kind, name)
		}
	}
	return a.Params.check()
}

func (p DiagnosisParams) set() []string {
	var out []string
	for _, f := range []struct {
		name string
		held bool
	}{
		{paramItem, p.Item != ""}, {paramVariation, p.Variation != ""}, {paramFamily, p.Family != ""}, {paramSHA, p.SHA != ""},
		{paramFiles, len(p.Files) > 0}, {paramRepro, p.Repro != ""}, {paramReason, p.Reason != ""}, {paramRequest, p.Request != ""},
	} {
		if f.held {
			out = append(out, f.name)
		}
	}
	return out
}

func (p DiagnosisParams) check() error {
	switch {
	case p.Variation != "" && !slices.Contains(variations, p.Variation):
		return fmt.Errorf("variation %q is not one of %v", p.Variation, variations)
	case p.SHA != "" && !shaPattern.MatchString(p.SHA):
		return fmt.Errorf("sha %q is not a hex commit id of 7 to 40 characters", p.SHA)
	case p.Request != "" && !agentclass.ValidRequestName(p.Request):
		return fmt.Errorf("request %q is not the name of a request file, such as dispatch-1", p.Request)
	}
	for _, f := range p.Files {
		if !filepath.IsLocal(f) || filepath.Clean(f) != f {
			return fmt.Errorf("file %q is not a clean path inside the repository", f)
		}
	}
	return nil
}
