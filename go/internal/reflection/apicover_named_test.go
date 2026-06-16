package reflection

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise the exported reflection consts/types that no existing test names by
// identifier. Each test asserts a REAL contract through a real consumer.

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSchemaVersionV1_GatesValidation pins the const through its only
// consumer, Validate(): a Reflection stamped SchemaVersionV1 passes the
// version check; SchemaVersionV1+1 (an unrecognized future version) fails, so
// producers and consumers upgrade in lockstep. Also pins the documented value.
func TestSchemaVersionV1_GatesValidation(t *testing.T) {
	t.Parallel()
	if SchemaVersionV1 != 1 {
		t.Fatalf("SchemaVersionV1=%d, want documented value 1", SchemaVersionV1)
	}
	ok := &Reflection{
		SchemaVersion: SchemaVersionV1, Cycle: 1, Phase: "scout", Agent: "evolve-scout",
		Slowdowns: []Slowdown{}, ReflectionConfidence: 0.5,
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate with SchemaVersion=SchemaVersionV1 must pass: %v", err)
	}
	future := &Reflection{
		SchemaVersion: SchemaVersionV1 + 1, Cycle: 1, Phase: "scout", Agent: "evolve-scout",
		Slowdowns: []Slowdown{}, ReflectionConfidence: 0.5,
	}
	if err := future.Validate(); err == nil {
		t.Error("Validate with SchemaVersion=SchemaVersionV1+1 must fail (unrecognized version)")
	}
}

// TestAggregatorConfidenceThreshold_StrictBoundary pins the const through its
// consumer, AcceptedByAggregator(): confidence strictly GREATER than the
// threshold is accepted; confidence EQUAL to it is rejected (the >, not >=,
// boundary that mirrors reflection-schema-test.sh T2). Also pins value 0.5.
func TestAggregatorConfidenceThreshold_StrictBoundary(t *testing.T) {
	t.Parallel()
	if AggregatorConfidenceThreshold != 0.5 {
		t.Fatalf("AggregatorConfidenceThreshold=%g, want documented value 0.5", AggregatorConfidenceThreshold)
	}
	above := &Reflection{ReflectionConfidence: AggregatorConfidenceThreshold + 0.1}
	if !above.AcceptedByAggregator() {
		t.Error("confidence above the threshold must be accepted")
	}
	atBoundary := &Reflection{ReflectionConfidence: AggregatorConfidenceThreshold}
	if atBoundary.AcceptedByAggregator() {
		t.Error("confidence EQUAL to the threshold must be rejected (filter is strictly >)")
	}
}

// TestFriction_FullStruct names Friction by full-struct construction and
// asserts it round-trips as the FrictionReceivedFrom element it models: the
// upstream phase, the issue, and the optional evidence pointer. The assertion
// exercises real production serialization (yaml.Marshal/Unmarshal honoring the
// struct's yaml tags): the <phase>-reflection.yaml sidecar these structs marshal
// to is parsed by downstream consumers, so the yaml keys are the wire contract.
func TestFriction_YAMLRoundTrip(t *testing.T) {
	t.Parallel()
	want := Friction{
		UpstreamPhase: "scout",
		Issue:         "AC#2 was untestable",
		Evidence:      "scout-report.md#ac-2",
	}
	in := &Reflection{
		SchemaVersion: SchemaVersionV1, Cycle: 1, Phase: "scout", Agent: "evolve-scout",
		Slowdowns: []Slowdown{}, FrictionReceivedFrom: []Friction{want}, ReflectionConfidence: 0.5,
	}
	blob, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	for _, key := range []string{"friction_received_from:", "upstream_phase: scout", "issue: AC#2 was untestable"} {
		if !strings.Contains(string(blob), key) {
			t.Errorf("Friction YAML missing %q (wire-key contract); got:\n%s", key, blob)
		}
	}
	var back Reflection
	if err := yaml.Unmarshal(blob, &back); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if len(back.FrictionReceivedFrom) != 1 || back.FrictionReceivedFrom[0] != want {
		t.Errorf("Friction did not survive the YAML round-trip: got %+v, want %+v", back.FrictionReceivedFrom, want)
	}
}

// TestImprovement_FullStruct names Improvement by full-struct construction and
// asserts the suggested-improvement contract: action, target file, evidence
// pointer, and a priority the validator accepts.
func TestImprovement_FullStruct(t *testing.T) {
	t.Parallel()
	want := Improvement{
		Action:          "Bump kb-search quota to 30",
		TargetFile:      ".evolve/profiles/scout.json",
		EvidencePointer: "scout-stdout.log:line=42",
		Priority:        "medium",
	}
	r := &Reflection{
		SchemaVersion: SchemaVersionV1, Cycle: 1, Phase: "scout", Agent: "evolve-scout",
		Slowdowns: []Slowdown{}, SuggestedImprovements: []Improvement{want},
		ReflectionConfidence: 0.5,
	}
	if r.SuggestedImprovements[0] != want {
		t.Fatalf("Improvement round-trip: got %+v, want %+v", r.SuggestedImprovements[0], want)
	}
	// A valid priority must let the whole Reflection pass validation.
	if err := r.Validate(); err != nil {
		t.Errorf("Reflection with a medium-priority Improvement must validate: %v", err)
	}
}
