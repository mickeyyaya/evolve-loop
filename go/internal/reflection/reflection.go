// Package reflection ports the Reflection Journal YAML schema
// (CLAUDE.md env-var EVOLVE_REFLECTION_JOURNAL; v10.20 advisory,
// v10.21 enforce). It reads, writes, and validates per-phase YAML
// sidecars at .evolve/runs/cycle-<N>/<phase>-reflection.yaml.
//
// The bash source is at scripts/lifecycle/phase-gate.sh (enforcement
// half) and scripts/observability/aggregate-reflections.sh (consumer
// half). This Go port owns the data structures so phase impls and the
// aggregator (Phase 3) work against the same types.
//
// Schema reference: agents/reflection-journal-schema.md;
// fixtures pinned by scripts/tests/reflection-schema-test.sh.
package reflection

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SchemaVersionV1 is the only schema currently recognized. Future
// versions surface as validation errors so consumers and producers
// upgrade in lockstep.
const SchemaVersionV1 = 1

// AggregatorConfidenceThreshold is the aggregator's filter: only
// reflections with strictly greater confidence are rolled up. Mirrors
// reflection-schema-test.sh T2 (0.2 skipped, 0.8 counted).
const AggregatorConfidenceThreshold = 0.5

// Reflection is the typed shape of a phase reflection sidecar.
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

// Slowdown describes one friction point a phase observed in itself.
type Slowdown struct {
	Category string `yaml:"category"`
	Evidence string `yaml:"evidence"`
	Severity string `yaml:"severity"` // low | medium | high
}

// Friction describes friction RECEIVED from an upstream phase.
type Friction struct {
	UpstreamPhase string `yaml:"upstream_phase"`
	Issue         string `yaml:"issue"`
	Evidence      string `yaml:"evidence,omitempty"`
}

// Improvement is a suggested-improvement entry consumed by retrospective.
type Improvement struct {
	Action          string `yaml:"action"`
	TargetFile      string `yaml:"target_file"`
	EvidencePointer string `yaml:"evidence_pointer"`
	Priority        string `yaml:"priority"` // low | medium | high
}

// PhaseTrackerRefs holds the numeric refs into the phase-tracker NDJSON
// log so the retrospective can cross-reference cost/latency claims.
type PhaseTrackerRefs struct {
	LatencyMS int     `yaml:"latency_ms"`
	CostUSD   float64 `yaml:"cost_usd"`
	Turns     int     `yaml:"turns"`
}

// Sidecar returns the canonical reflection-file path:
// <runsDir>/cycle-<N>/<phase>-reflection.yaml.
// This is the path phase-gate.sh:reflection-required check looks for.
func Sidecar(runsDir string, cycle int, phase string) string {
	return filepath.Join(runsDir, fmt.Sprintf("cycle-%d", cycle), phase+"-reflection.yaml")
}

// Parse deserializes YAML into a Reflection.
func Parse(data []byte) (*Reflection, error) {
	r := &Reflection{}
	if err := yaml.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("reflection: parse: %w", err)
	}
	// yaml.v3 leaves `slowdowns: []` as nil; normalize to non-nil
	// empty slice so callers can distinguish "empty array per schema"
	// from "field absent".
	if r.Slowdowns == nil {
		r.Slowdowns = []Slowdown{}
	}
	return r, nil
}

// ReadFile reads + parses a reflection YAML file.
func ReadFile(path string) (*Reflection, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Write serializes a Reflection to YAML and writes it to path
// atomically (tmp file + rename, so concurrent readers never see a
// half-written file). Validates the Reflection first; a malformed
// sidecar is worse than no sidecar.
func Write(path string, r *Reflection) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("reflection: refusing to write invalid: %w", err)
	}
	data, err := yaml.Marshal(r)
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

// Validate enforces the schema constraints not captured by yaml.v3's
// type system: required fields, severity/priority enums, confidence
// range, and schema_version recognition.
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

// AcceptedByAggregator reports whether this reflection's confidence
// passes the aggregator's filter (> 0.5; cf. reflection-schema-test.sh T2).
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
