// Package reflection reads, writes and validates the per-phase Reflection Journal YAML sidecar.
// See docs/architecture/packages/internal-reflection.md.
package reflection

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
)

const SchemaVersionV1 = 1

const AggregatorConfidenceThreshold = 0.5

type Reflection struct {
	SchemaVersion         int              `yaml:"schema_version"`
	Cycle                 int              `yaml:"cycle"`
	Phase                 string           `yaml:"phase"`
	Agent                 string           `yaml:"agent"`
	PhaseSmooth           bool             `yaml:"phase_smooth"`
	Slowdowns             []Slowdown       `yaml:"slowdowns"`
	FrictionReceivedFrom  []Friction       `yaml:"friction_received_from,omitempty"`
	SuggestedImprovements []Improvement    `yaml:"suggested_improvements,omitempty"`
	ReflectionConfidence  float64          `yaml:"reflection_confidence"`
	PhaseTrackerRefs      PhaseTrackerRefs `yaml:"phase_tracker_refs"`
}

type Slowdown struct {
	Category string `yaml:"category"`
	Evidence string `yaml:"evidence"`
	Severity string `yaml:"severity"`
}

type Friction struct {
	UpstreamPhase string `yaml:"upstream_phase"`
	Issue         string `yaml:"issue"`
	Evidence      string `yaml:"evidence,omitempty"`
}

type Improvement struct {
	Action          string `yaml:"action"`
	TargetFile      string `yaml:"target_file"`
	EvidencePointer string `yaml:"evidence_pointer"`
	Priority        string `yaml:"priority"`
}

type PhaseTrackerRefs struct {
	LatencyMS int     `yaml:"latency_ms"`
	CostUSD   float64 `yaml:"cost_usd"`
	Turns     int     `yaml:"turns"`
}

func Sidecar(runsDir string, cycle int, phase string) string {
	return filepath.Join(runsDir, fmt.Sprintf("cycle-%d", cycle), phase+"-reflection.yaml")
}

func Parse(data []byte) (*Reflection, error) {
	r := &Reflection{}
	if err := yamlcodec.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("reflection: parse: %w", err)
	}
	if r.Slowdowns == nil {
		r.Slowdowns = []Slowdown{}
	}
	return r, nil
}

func ReadFile(path string) (*Reflection, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

var marshalYAML = yamlcodec.Marshal

func Write(path string, r *Reflection) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("reflection: refusing to write invalid: %w", err)
	}
	data, err := marshalYAML(r)
	if err != nil {
		return fmt.Errorf("reflection: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("reflection: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("reflection: rename: %w", err)
	}
	return nil
}

func (r *Reflection) Validate() error {
	if r == nil {
		return errors.New("reflection: nil")
	}
	if r.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("reflection: unknown schema_version=%d (want %d)", r.SchemaVersion, SchemaVersionV1)
	}
	if r.Cycle <= 0 {
		return fmt.Errorf("reflection: invalid cycle=%d", r.Cycle)
	}
	if r.Phase == "" {
		return errors.New("reflection: phase is required")
	}
	if r.Agent == "" {
		return errors.New("reflection: agent is required")
	}
	if r.ReflectionConfidence < 0 || r.ReflectionConfidence > 1 {
		return fmt.Errorf("reflection: confidence=%g out of [0,1]", r.ReflectionConfidence)
	}
	for i, s := range r.Slowdowns {
		if !isValidSeverity(s.Severity) {
			return fmt.Errorf("reflection: slowdowns[%d].severity=%q not in {low,medium,high}", i, s.Severity)
		}
	}
	for i, imp := range r.SuggestedImprovements {
		if !isValidPriority(imp.Priority) {
			return fmt.Errorf("reflection: suggested_improvements[%d].priority=%q not in {low,medium,high}", i, imp.Priority)
		}
	}
	return nil
}

func (r *Reflection) AcceptedByAggregator() bool {
	return r.ReflectionConfidence > AggregatorConfidenceThreshold
}

func isValidSeverity(s string) bool {
	switch s {
	case "low", "medium", "high":
		return true
	}
	return false
}

func isValidPriority(s string) bool {
	switch s {
	case "low", "medium", "high":
		return true
	}
	return false
}
