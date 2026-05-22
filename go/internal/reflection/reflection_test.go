// Package reflection ports the per-phase Reflection Journal YAML
// schema. Bash-side enforcement lives in
// scripts/lifecycle/phase-gate.sh (when EVOLVE_REFLECTION_JOURNAL=1)
// and scripts/observability/aggregate-reflections.sh. This Go port
// owns reading, writing, validating, and computing canonical paths so
// phase impls don't have to reach back into bash.
package reflection

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canonicalYAML matches the fixture written by reflection-schema-test.sh.
const canonicalYAML = `schema_version: 1
cycle: 1001
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=42"
    severity: medium
friction_received_from: []
suggested_improvements:
  - action: "Bump kb-search quota to 30"
    target_file: ".evolve/profiles/scout.json"
    evidence_pointer: "scout-stdout.log:line=42"
    priority: medium
reflection_confidence: 0.8
phase_tracker_refs:
  latency_ms: 1000
  cost_usd: 0.5
  turns: 10
`

// TestRead_CanonicalSchema verifies every required field round-trips.
// The schema is documented in agents/reflection-journal-schema.md and
// pinned by reflection-schema-test.sh T1.
func TestRead_CanonicalSchema(t *testing.T) {
	r, err := Parse([]byte(canonicalYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.SchemaVersion != 1 {
		t.Errorf("SchemaVersion=%d, want 1", r.SchemaVersion)
	}
	if r.Cycle != 1001 {
		t.Errorf("Cycle=%d, want 1001", r.Cycle)
	}
	if r.Phase != "scout" {
		t.Errorf("Phase=%q, want scout", r.Phase)
	}
	if r.Agent != "evolve-scout" {
		t.Errorf("Agent=%q, want evolve-scout", r.Agent)
	}
	if r.PhaseSmooth {
		t.Error("PhaseSmooth=true, want false")
	}
	if len(r.Slowdowns) != 1 {
		t.Fatalf("Slowdowns=%d entries, want 1", len(r.Slowdowns))
	}
	if r.Slowdowns[0].Category != "research-quota" {
		t.Errorf("Slowdown.Category=%q, want research-quota", r.Slowdowns[0].Category)
	}
	if r.Slowdowns[0].Severity != "medium" {
		t.Errorf("Slowdown.Severity=%q, want medium", r.Slowdowns[0].Severity)
	}
	if r.ReflectionConfidence != 0.8 {
		t.Errorf("Confidence=%g, want 0.8", r.ReflectionConfidence)
	}
	if r.PhaseTrackerRefs.LatencyMS != 1000 || r.PhaseTrackerRefs.CostUSD != 0.5 || r.PhaseTrackerRefs.Turns != 10 {
		t.Errorf("phase_tracker_refs round-trip: %+v", r.PhaseTrackerRefs)
	}
	if len(r.SuggestedImprovements) != 1 {
		t.Fatalf("SuggestedImprovements=%d, want 1", len(r.SuggestedImprovements))
	}
	imp := r.SuggestedImprovements[0]
	if imp.Action == "" || imp.TargetFile == "" || imp.EvidencePointer == "" {
		t.Errorf("Improvement missing required fields: %+v", imp)
	}
}

// TestRead_FrictionReceivedFrom — T5 of reflection-schema-test.sh:
// upstream/downstream pair must parse.
func TestRead_FrictionReceivedFrom(t *testing.T) {
	src := `schema_version: 1
cycle: 5001
phase: tdd
agent: evolve-tdd-engineer
phase_smooth: false
slowdowns: []
friction_received_from:
  - upstream_phase: scout
    issue: "AC#2 was untestable"
    evidence: "scout-report.md#ac-2"
reflection_confidence: 0.7
phase_tracker_refs:
  latency_ms: 30000
  cost_usd: 0.6
  turns: 20
`
	r, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(r.FrictionReceivedFrom) != 1 {
		t.Fatalf("FrictionReceivedFrom=%d, want 1", len(r.FrictionReceivedFrom))
	}
	f := r.FrictionReceivedFrom[0]
	if f.UpstreamPhase != "scout" || f.Issue == "" {
		t.Errorf("Friction round-trip: %+v", f)
	}
}

// TestRead_PhaseSmooth_EmptySlowdowns — T3: smooth phase has empty
// slowdowns array; aggregator counts the reflection but emits no
// category. The reader must produce []Slowdown{} (empty, not nil).
func TestRead_PhaseSmooth_EmptySlowdowns(t *testing.T) {
	src := `schema_version: 1
cycle: 3001
phase: audit
agent: evolve-auditor
phase_smooth: true
slowdowns: []
friction_received_from: []
reflection_confidence: 0.9
phase_tracker_refs:
  latency_ms: 500
  cost_usd: 0.3
  turns: 5
`
	r, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !r.PhaseSmooth {
		t.Error("PhaseSmooth=false, want true")
	}
	if r.Slowdowns == nil || len(r.Slowdowns) != 0 {
		t.Errorf("Slowdowns=%v, want empty (non-nil) slice", r.Slowdowns)
	}
}

// TestValidate_HappyPath — canonical reflection passes validation.
func TestValidate_HappyPath(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	if err := r.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// TestValidate_MissingRequiredFields — empty Reflection must fail.
func TestValidate_MissingRequiredFields(t *testing.T) {
	r := &Reflection{}
	if err := r.Validate(); err == nil {
		t.Error("Validate() on empty: want error")
	}
}

// TestValidate_InvalidSeverity — slowdown severity must be one of
// low/medium/high.
func TestValidate_InvalidSeverity(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	r.Slowdowns[0].Severity = "ultra-critical"
	if err := r.Validate(); err == nil {
		t.Error("Validate(): bad severity should fail")
	}
}

// TestValidate_InvalidPriority — improvement priority must be
// low/medium/high.
func TestValidate_InvalidPriority(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	r.SuggestedImprovements[0].Priority = "nuclear"
	if err := r.Validate(); err == nil {
		t.Error("Validate(): bad priority should fail")
	}
}

// TestValidate_ConfidenceRange — reflection_confidence must be [0, 1].
func TestValidate_ConfidenceRange(t *testing.T) {
	for _, bad := range []float64{-0.1, 1.1, 2.0} {
		r, _ := Parse([]byte(canonicalYAML))
		r.ReflectionConfidence = bad
		if err := r.Validate(); err == nil {
			t.Errorf("Validate(): confidence=%g should fail", bad)
		}
	}
}

// TestValidate_SchemaVersion — only v1 is recognized today; future
// versions surface as errors so consumers can upgrade in lockstep.
func TestValidate_SchemaVersion(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	r.SchemaVersion = 99
	if err := r.Validate(); err == nil {
		t.Error("Validate(): unknown schema_version should fail")
	}
}

// TestSidecar_CanonicalPath — phase-gate.sh expects sidecars at
// <runs_dir>/cycle-<N>/<phase>-reflection.yaml.
func TestSidecar_CanonicalPath(t *testing.T) {
	got := Sidecar("/x/.evolve/runs", 42, "scout")
	want := "/x/.evolve/runs/cycle-42/scout-reflection.yaml"
	if got != want {
		t.Errorf("Sidecar=%q, want %q", got, want)
	}
}

// TestWriteAndRead_RoundTrip — write to disk, read back, compare.
// Atomic write semantics (tmp file + rename) verified by file existence
// at the final path with the expected payload.
func TestWriteAndRead_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cyc := filepath.Join(tmp, "cycle-7")
	if err := os.MkdirAll(cyc, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(cyc, "build-reflection.yaml")

	in := &Reflection{
		SchemaVersion: 1, Cycle: 7, Phase: "build", Agent: "evolve-builder",
		PhaseSmooth: false,
		Slowdowns:   []Slowdown{{Category: "tool-batching", Evidence: "stdout.log:1", Severity: "high"}},
		ReflectionConfidence: 0.7,
		PhaseTrackerRefs:     PhaseTrackerRefs{LatencyMS: 1, CostUSD: 0.1, Turns: 5},
	}
	if err := Write(out, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	r, err := ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if r.Phase != "build" || r.Slowdowns[0].Severity != "high" {
		t.Errorf("Round-trip mismatch: %+v", r)
	}
}

// TestWrite_AtomicReplace — overwriting an existing file must not
// leave a corrupt half-write if the underlying syscall fails mid-flight.
// We can't simulate ENOSPC easily; instead we verify the tmp file is
// cleaned up on success and the destination matches new content.
func TestWrite_AtomicReplace(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "x-reflection.yaml")
	if err := os.WriteFile(out, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := &Reflection{
		SchemaVersion: 1, Cycle: 1, Phase: "x", Agent: "a", PhaseSmooth: true,
		Slowdowns: []Slowdown{}, ReflectionConfidence: 0.5,
		PhaseTrackerRefs: PhaseTrackerRefs{},
	}
	if err := Write(out, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "old") {
		t.Errorf("Destination still has old content: %q", b)
	}
	// No leftover .tmp file in the dir.
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

// TestParse_InvalidYAML — surface a parse error with context.
func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("not: yaml: :::"))
	if err == nil {
		t.Error("Parse(garbage): want error")
	}
}

// TestReadFile_NotFound — ErrNotExist propagates.
func TestReadFile_NotFound(t *testing.T) {
	_, err := ReadFile("/nonexistent-path-xyz")
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err=%v, want os.ErrNotExist", err)
	}
}

// TestParse_AbsentSlowdownsNormalized — when slowdowns field is
// absent in the YAML (vs explicitly []), the parser must normalize
// to a non-nil empty slice. The aggregator depends on this distinction
// when emitting empty `slowdown_categories: []`.
func TestParse_AbsentSlowdownsNormalized(t *testing.T) {
	src := `schema_version: 1
cycle: 1
phase: x
agent: a
phase_smooth: true
reflection_confidence: 0.9
phase_tracker_refs:
  latency_ms: 0
  cost_usd: 0
  turns: 0
`
	r, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Slowdowns == nil {
		t.Error("Slowdowns=nil after absent-field parse; want []Slowdown{}")
	}
	if len(r.Slowdowns) != 0 {
		t.Errorf("Slowdowns=%v, want empty", r.Slowdowns)
	}
}

// TestWrite_RenameFailure_RemovesTmp — when rename fails (e.g.,
// destination becomes a directory), tmp file must be cleaned up.
// Simulating rename failure: create a directory at the destination
// path so os.Rename(file → directory) refuses on macOS/Linux.
func TestWrite_RenameFailure_RemovesTmp(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "block-reflection.yaml")
	// Make dest a non-empty directory so rename can't replace it
	// (POSIX: rename(file, dir) fails when dir is non-empty or
	// when the dest is a directory and src is a file).
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "filler"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Reflection{
		SchemaVersion: 1, Cycle: 1, Phase: "x", Agent: "a", PhaseSmooth: true,
		Slowdowns: []Slowdown{}, ReflectionConfidence: 0.5,
	}
	if err := Write(dest, r); err == nil {
		t.Error("Write into existing non-empty dir: want error")
	}
	// Tmp must not linger.
	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file lingered at %s", dest+".tmp")
	}
}

// TestValidate_NilReceiver — defensive: r.Validate on nil pointer
// must return an error, not panic.
func TestValidate_NilReceiver(t *testing.T) {
	var r *Reflection
	if err := r.Validate(); err == nil {
		t.Error("Validate(nil): want error")
	}
}

// TestValidate_ZeroOrNegativeCycle — cycle must be positive.
func TestValidate_ZeroOrNegativeCycle(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	r.Cycle = 0
	if err := r.Validate(); err == nil {
		t.Error("Validate(cycle=0): want error")
	}
}

// TestValidate_EmptyPhase — phase required.
func TestValidate_EmptyPhase(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	r.Phase = ""
	if err := r.Validate(); err == nil {
		t.Error("Validate(phase=\"\"): want error")
	}
}

// TestValidate_EmptyAgent — agent required.
func TestValidate_EmptyAgent(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	r.Agent = ""
	if err := r.Validate(); err == nil {
		t.Error("Validate(agent=\"\"): want error")
	}
}

// TestWrite_RefusesInvalid — Write Validates before serializing.
// A malformed sidecar is worse than no sidecar (phase-gate.sh would
// pass an invalid file as "present", masking the producer bug).
func TestWrite_RefusesInvalid(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "bad-reflection.yaml")
	bad := &Reflection{SchemaVersion: 1, Phase: "x", Agent: "a", Slowdowns: []Slowdown{}, ReflectionConfidence: 0.5} // cycle=0
	if err := Write(out, bad); err == nil {
		t.Error("Write(invalid): want error, got nil")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("invalid Reflection should leave no file at %s", out)
	}
}

// TestWrite_NonexistentDir — tmp-file path goes through a missing
// parent dir; the write must fail (os.WriteFile error path).
func TestWrite_NonexistentDir(t *testing.T) {
	r := &Reflection{
		SchemaVersion: 1, Cycle: 1, Phase: "x", Agent: "a", PhaseSmooth: true,
		Slowdowns: []Slowdown{}, ReflectionConfidence: 0.5,
	}
	err := Write("/no/such/dir/reflection.yaml", r)
	if err == nil {
		t.Error("Write to missing dir: want error")
	}
}

// TestAcceptedByAggregator — the aggregator filter is confidence > 0.5
// (per reflection-schema-test.sh T2: 0.2 is skipped, 0.8 is counted).
// Expose a predicate matching the bash threshold so callers don't
// re-derive it.
func TestAcceptedByAggregator(t *testing.T) {
	r, _ := Parse([]byte(canonicalYAML))
	if !r.AcceptedByAggregator() {
		t.Error("AcceptedByAggregator()=false for confidence=0.8 (want true)")
	}
	r.ReflectionConfidence = 0.2
	if r.AcceptedByAggregator() {
		t.Error("AcceptedByAggregator()=true for confidence=0.2 (want false)")
	}
	r.ReflectionConfidence = 0.5
	if r.AcceptedByAggregator() {
		t.Error("AcceptedByAggregator()=true for confidence==0.5 (want false; threshold is >0.5)")
	}
}
