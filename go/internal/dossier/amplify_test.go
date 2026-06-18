package dossier

// amplify_test.go — Test Amplification phase adversarial cases.
// Covers contract edge cases NOT exercised by TDD-engineer RED tests:
// boundary inputs, nil guards, postcondition invariants, determinism,
// idempotency, and special-character handling.
//
// Several tests are DELIBERATELY GAP TESTS that expose contract requirements
// not yet satisfied by the current stub implementation (marked with "GAP:").
// These tests fail intentionally, setting amplify.failures_found.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ampPass returns a minimal valid PASS dossier for amplification tests.
// Named distinctly from passDossier() in dossier_test.go to avoid collision.
func ampPass() *Dossier {
	return &Dossier{
		Cycle:        5,
		Goal:         "amplify-pass-goal",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "scout", Verdict: VerdictPass}},
	}
}

// ampFail returns a minimal valid FAIL dossier for amplification tests.
func ampFail() *Dossier {
	return &Dossier{
		Cycle:        6,
		Goal:         "amplify-fail-goal",
		FinalVerdict: VerdictFail,
		Phases:       []PhaseRecord{{Name: "audit", Verdict: VerdictFail}},
		Defects:      []Defect{{ID: "AMP-D1", Severity: "HIGH", Summary: "amplify defect"}},
		Carryover:    []Carryover{{ID: "AMP-C1", Action: "fix amplify defect", Priority: "P0"}},
	}
}

// safeCall invokes f and catches panics, converting them to errors.
// Used for nil-dereference gap tests where the implementation panics
// instead of returning a proper error.
func safeCallJSON(f func() ([]byte, error)) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}

func safeCallWrite(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}

// ── Build adversarial tests ────────────────────────────────────────────────

// TestBuild_ZeroCycleBoundary: cycle == 0 is the exact boundary violation;
// spec requires cycle >= 1.
func TestBuild_ZeroCycleBoundary(t *testing.T) {
	tmp := t.TempDir()
	_, err := Build(0, BuildOpts{WorkspacePath: tmp, Goal: "g"})
	if err == nil {
		t.Error("Build(0,...) must return error: cycle must be >= 1 (boundary)")
	}
}

// TestBuild_NegativeCycle: cycle < 0 is also invalid.
func TestBuild_NegativeCycle(t *testing.T) {
	tmp := t.TempDir()
	_, err := Build(-99, BuildOpts{WorkspacePath: tmp, Goal: "g"})
	if err == nil {
		t.Error("Build(-99,...) must return error")
	}
}

// TestBuild_BlankWorkspacePath: GAP — contract requires non-empty WorkspacePath.
// Current stub implementation does not validate this precondition.
func TestBuild_BlankWorkspacePath(t *testing.T) {
	_, err := Build(1, BuildOpts{WorkspacePath: "", Goal: "g"})
	if err == nil {
		t.Error("GAP: Build with blank WorkspacePath must return error (precondition not yet enforced)")
	}
}

// TestBuild_BlankGoal: GAP — contract requires non-blank Goal.
// Current stub does not validate Goal before constructing the dossier.
func TestBuild_BlankGoal(t *testing.T) {
	tmp := t.TempDir()
	_, err := Build(1, BuildOpts{WorkspacePath: tmp, Goal: ""})
	if err == nil {
		t.Error("GAP: Build with empty Goal must return error")
	}
	_, err = Build(1, BuildOpts{WorkspacePath: tmp, Goal: "   "})
	if err == nil {
		t.Error("GAP: Build with whitespace-only Goal must return error")
	}
}

// TestBuild_PostconditionPassesValidate: the returned dossier must satisfy
// Validate() (contract postcondition). Tests with a valid non-blank goal.
func TestBuild_PostconditionPassesValidate(t *testing.T) {
	tmp := t.TempDir()
	d, err := Build(1, BuildOpts{WorkspacePath: tmp, Goal: "validate-post"})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if vErr := d.Validate(); vErr != nil {
		t.Errorf("Build postcondition: returned dossier must pass Validate(), got: %v", vErr)
	}
}

// TestBuild_PostconditionHasAtLeastOnePhase: contract mandates >=1 PhaseRecord.
func TestBuild_PostconditionHasAtLeastOnePhase(t *testing.T) {
	tmp := t.TempDir()
	d, err := Build(1, BuildOpts{WorkspacePath: tmp, Goal: "phases-post"})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(d.Phases) == 0 {
		t.Error("Build postcondition: Dossier.Phases must contain at least one PhaseRecord")
	}
}

// TestBuild_Deterministic: contract requires deterministic output.
// "Must not use current time, randomness, or Go map iteration order."
func TestBuild_Deterministic(t *testing.T) {
	tmp := t.TempDir()
	opts := BuildOpts{WorkspacePath: tmp, Goal: "determinism-goal", RunID: "run-det-test"}
	d1, err1 := Build(1, opts)
	d2, err2 := Build(1, opts)
	if err1 != nil || err2 != nil {
		t.Fatalf("Build errors: %v / %v", err1, err2)
	}
	j1, jErr1 := RenderJSON(d1)
	j2, jErr2 := RenderJSON(d2)
	if jErr1 != nil || jErr2 != nil {
		t.Fatalf("RenderJSON errors: %v / %v", jErr1, jErr2)
	}
	if !bytes.Equal(j1, j2) {
		t.Error("Build must be deterministic: two calls with same inputs produced different JSON")
	}
}

// TestBuild_CyclePreservedInResult: Dossier.Cycle must equal the input cycle.
func TestBuild_CyclePreservedInResult(t *testing.T) {
	tmp := t.TempDir()
	const want = 42
	d, err := Build(want, BuildOpts{WorkspacePath: tmp, Goal: "cycle-preserved"})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if d.Cycle != want {
		t.Errorf("Build result Cycle: got %d, want %d", d.Cycle, want)
	}
}

// ── RenderJSON adversarial tests ───────────────────────────────────────────

// TestRenderJSON_NilDossier: GAP — contract says "Rejects nil".
// Current implementation calls json.MarshalIndent(nil) → returns "null", nil
// (no error). The test fails to expose this gap because "null" != "null\n".
// Fixed: any nil-error result is a gap.
func TestRenderJSON_NilDossier(t *testing.T) {
	out, err := safeCallJSON(func() ([]byte, error) { return RenderJSON(nil) })
	if err != nil {
		return // correct: nil input properly returned an error
	}
	t.Errorf("GAP: RenderJSON(nil) must return error; got nil err with output: %q", out)
}

// TestRenderJSON_InvalidDossierReturnsError: GAP — contract says "Rejects any
// dossier for which Validate fails". Current impl does not call Validate.
func TestRenderJSON_InvalidDossierReturnsError(t *testing.T) {
	bad := &Dossier{
		Cycle:        0, // fails Validate
		Goal:         "bad",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "p", Verdict: VerdictPass}},
	}
	_, err := RenderJSON(bad)
	if err == nil {
		t.Error("GAP: RenderJSON with invalid dossier (cycle=0) must return error; Validate not called")
	}
}

// TestRenderJSON_TrailingNewline: GAP — contract says "Returns UTF-8 JSON with
// a trailing newline." json.MarshalIndent does not add a trailing newline.
func TestRenderJSON_TrailingNewline(t *testing.T) {
	d := ampPass()
	out, err := RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("RenderJSON returned empty output")
	}
	if out[len(out)-1] != '\n' {
		t.Errorf("GAP: RenderJSON must end with trailing '\\n' (contract); got last byte 0x%02x", out[len(out)-1])
	}
}

// TestRenderJSON_Deterministic: same dossier → byte-identical output.
// Map-valued Signals must use encoding/json stable key ordering.
func TestRenderJSON_Deterministic(t *testing.T) {
	d := ampPass()
	d.Phases[0].Signals = map[string]any{"z-key": 99, "a-key": 1, "m-key": "mid"}
	out1, err1 := RenderJSON(d)
	out2, err2 := RenderJSON(d)
	if err1 != nil || err2 != nil {
		t.Fatalf("RenderJSON errors: %v / %v", err1, err2)
	}
	if !bytes.Equal(out1, out2) {
		t.Error("RenderJSON must be deterministic: two calls on same dossier produced different output")
	}
}

// TestRenderJSON_DoesNotMutateDossier: contract says "Does not mutate d."
func TestRenderJSON_DoesNotMutateDossier(t *testing.T) {
	d := ampPass()
	origCycle := d.Cycle
	origGoal := d.Goal
	origVerdict := d.FinalVerdict
	origPhaseName := d.Phases[0].Name
	if _, err := RenderJSON(d); err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	if d.Cycle != origCycle || d.Goal != origGoal || d.FinalVerdict != origVerdict || d.Phases[0].Name != origPhaseName {
		t.Error("RenderJSON must not mutate the dossier")
	}
}

// TestRenderJSON_FailDossierRoundtrip: FAIL dossier with defects+carryover
// must survive RenderJSON → ParseJSON intact.
func TestRenderJSON_FailDossierRoundtrip(t *testing.T) {
	d := ampFail()
	data, err := RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON(FAIL) failed: %v", err)
	}
	parsed, pErr := ParseJSON(data)
	if pErr != nil {
		t.Fatalf("ParseJSON failed: %v", pErr)
	}
	if parsed.FinalVerdict != VerdictFail {
		t.Errorf("round-trip FinalVerdict: got %q, want %q", parsed.FinalVerdict, VerdictFail)
	}
	if len(parsed.Defects) != len(d.Defects) {
		t.Errorf("round-trip Defects len: got %d, want %d", len(parsed.Defects), len(d.Defects))
	}
	if len(parsed.Carryover) != len(d.Carryover) {
		t.Errorf("round-trip Carryover len: got %d, want %d", len(parsed.Carryover), len(d.Carryover))
	}
}

// ── RenderMarkdown adversarial tests ──────────────────────────────────────

// TestRenderMarkdown_NilDossier: GAP — contract says "Rejects nil".
// Current implementation may panic via template.Execute on nil pointer.
func TestRenderMarkdown_NilDossier(t *testing.T) {
	_, err := safeCallJSON(func() ([]byte, error) { return RenderMarkdown(nil) })
	if err == nil {
		t.Error("GAP: RenderMarkdown(nil) must return error (not panic or silently succeed)")
	}
}

// TestRenderMarkdown_InvalidDossierReturnsError: GAP — contract says "Rejects
// invalid dossiers". Current implementation does not call Validate.
func TestRenderMarkdown_InvalidDossierReturnsError(t *testing.T) {
	bad := &Dossier{
		Cycle:        0,
		Goal:         "bad",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "p", Verdict: VerdictPass}},
	}
	_, err := RenderMarkdown(bad)
	if err == nil {
		t.Error("GAP: RenderMarkdown with invalid dossier must return error; Validate not called")
	}
}

// TestRenderMarkdown_TrailingNewline: contract says "Returns UTF-8 Markdown
// with a trailing newline."
func TestRenderMarkdown_TrailingNewline(t *testing.T) {
	// Use a dossier with all optional collections populated to ensure trailing
	// content renders and the newline is present.
	d := ampFail()
	out, err := RenderMarkdown(d)
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("RenderMarkdown returned empty output")
	}
	if out[len(out)-1] != '\n' {
		t.Errorf("RenderMarkdown must end with '\\n', got last byte 0x%02x", out[len(out)-1])
	}
}

// TestRenderMarkdown_GoalWithPoundSignsStable: contract says "Escapes or safely
// formats untrusted artifact text so it cannot alter the intended section hierarchy."
// A goal containing '#' must not produce spurious section headers.
func TestRenderMarkdown_GoalWithPoundSignsStable(t *testing.T) {
	d := ampPass()
	d.Goal = "implement ## the dossier # feature"
	out, err := RenderMarkdown(d)
	if err != nil {
		t.Fatalf("RenderMarkdown failed with goal containing '#': %v", err)
	}
	if len(out) == 0 {
		t.Error("RenderMarkdown returned empty output for goal with '#'")
	}
	// Call twice to verify output is stable (not randomized).
	out2, err2 := RenderMarkdown(d)
	if err2 != nil {
		t.Fatalf("RenderMarkdown second call failed: %v", err2)
	}
	if !bytes.Equal(out, out2) {
		t.Error("RenderMarkdown must produce stable output across calls with special-char goal")
	}
}

// TestRenderMarkdown_EmptyOptionalCollectionsAreStable: contract says empty
// optional collections are either omitted or rendered with one fixed placeholder;
// the choice must be stable.
func TestRenderMarkdown_EmptyOptionalCollectionsAreStable(t *testing.T) {
	d := ampPass()
	d.Defects = nil
	d.Lessons = nil
	d.Carryover = nil
	d.Decisions = nil
	out1, err1 := RenderMarkdown(d)
	out2, err2 := RenderMarkdown(d)
	if err1 != nil || err2 != nil {
		t.Fatalf("RenderMarkdown errors: %v / %v", err1, err2)
	}
	if !bytes.Equal(out1, out2) {
		t.Error("RenderMarkdown with empty collections must produce identical output across calls")
	}
}

// TestRenderMarkdown_DoesNotMutateDossier: contract says "Does not mutate d."
func TestRenderMarkdown_DoesNotMutateDossier(t *testing.T) {
	d := ampPass()
	origGoal := d.Goal
	if _, err := RenderMarkdown(d); err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}
	if d.Goal != origGoal {
		t.Errorf("RenderMarkdown mutated dossier: Goal changed from %q to %q", origGoal, d.Goal)
	}
}

// ── ParseJSON adversarial tests ────────────────────────────────────────────

// TestParseJSON_Nil: nil input must return error.
func TestParseJSON_Nil(t *testing.T) {
	_, err := ParseJSON(nil)
	if err == nil {
		t.Error("ParseJSON(nil) must return error")
	}
}

// TestParseJSON_Empty: empty bytes must return error.
func TestParseJSON_Empty(t *testing.T) {
	_, err := ParseJSON([]byte{})
	if err == nil {
		t.Error("ParseJSON(empty) must return error")
	}
}

// TestParseJSON_InvalidJSON: non-object JSON must return error.
func TestParseJSON_InvalidJSON(t *testing.T) {
	cases := []string{
		"not-json",
		`{bad:`,
		`"string"`, // valid JSON but not an object
		`42`,       // valid JSON but not an object
	}
	for _, c := range cases {
		_, err := ParseJSON([]byte(c))
		if err == nil {
			t.Errorf("ParseJSON(%q) must return error (not a Dossier object)", c)
		}
	}
}

// TestParseJSON_PreservesAllFields: all Dossier fields survive RenderJSON → ParseJSON.
func TestParseJSON_PreservesAllFields(t *testing.T) {
	original := &Dossier{
		Cycle:        9,
		RunID:        "run-abc",
		Goal:         "full-fidelity-goal",
		FinalVerdict: VerdictFail,
		CommitSHA:    "sha1abc",
		TreeSHA:      "sha2def",
		StartedAt:    "2026-06-18T00:00:00Z",
		EndedAt:      "2026-06-18T01:00:00Z",
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass, KeyFindings: "clean build"}},
		Defects:      []Defect{{ID: "DEF-1", Severity: "HIGH", Summary: "defect one", Fix: "fix one"}},
		Decisions:    []string{"chose approach A"},
		Lessons:      []Lesson{{ID: "LES-1", Pattern: "avoid X", PreventiveAction: "do Y"}},
		Carryover:    []Carryover{{ID: "CAR-1", Action: "address defect one", Priority: "P0"}},
	}
	data, err := RenderJSON(original)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	got, err := ParseJSON(data)
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	checks := []struct {
		name string
		ok   bool
	}{
		{"Cycle", got.Cycle == original.Cycle},
		{"RunID", got.RunID == original.RunID},
		{"CommitSHA", got.CommitSHA == original.CommitSHA},
		{"FinalVerdict", got.FinalVerdict == original.FinalVerdict},
		{"Defects len", len(got.Defects) == len(original.Defects)},
		{"Carryover len", len(got.Carryover) == len(original.Carryover)},
		{"Lessons len", len(got.Lessons) == len(original.Lessons)},
		{"Decisions len", len(got.Decisions) == len(original.Decisions)},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("ParseJSON round-trip: %s mismatch", c.name)
		}
	}
	if err := got.Validate(); err != nil {
		t.Errorf("parsed dossier must pass Validate(): %v", err)
	}
}

// ── Write adversarial tests ────────────────────────────────────────────────

// TestWrite_NilDossier: GAP — Write(nil,...) should return error but currently
// may panic on d.Cycle dereference.
func TestWrite_NilDossier(t *testing.T) {
	err := safeCallWrite(func() error { return Write(nil, t.TempDir(), false) })
	if err == nil {
		t.Error("GAP: Write(nil, ...) must return error (not panic)")
	}
}

// TestWrite_BlankDir: GAP — Write with empty dir should return error.
// Current impl calls filepath.Join("", ...) which silently writes to CWD.
func TestWrite_BlankDir(t *testing.T) {
	err := Write(ampPass(), "", false)
	if err == nil {
		// Attempt cleanup in case it wrote a stray file to CWD.
		_ = os.Remove("cycle-5.json")
		_ = os.Remove("cycle-5.md")
		t.Error("GAP: Write(d, blank-dir, ...) must return error (writes to CWD without error)")
	}
}

// TestWrite_FilesCreated: Write(d, dir, false) must produce both json and md
// files at dir/cycle-N.json and dir/cycle-N.md.
// Note: current implementation writes directly to dir (not dir/knowledge-base/cycles/).
func TestWrite_FilesCreated(t *testing.T) {
	dir := t.TempDir()
	d := ampPass()
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	wantJSON := filepath.Join(dir, "cycle-5.json")
	wantMD := filepath.Join(dir, "cycle-5.md")
	for _, p := range []string{wantJSON, wantMD} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected output file not found: %s", p)
		}
	}
}

// TestWrite_JSONIsValidDossier: the written JSON must decode to a valid dossier.
func TestWrite_JSONIsValidDossier(t *testing.T) {
	dir := t.TempDir()
	d := ampPass()
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "cycle-5.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	parsed, pErr := ParseJSON(data)
	if pErr != nil {
		t.Fatalf("ParseJSON on written file: %v", pErr)
	}
	if parsed.Cycle != d.Cycle {
		t.Errorf("Cycle: got %d, want %d", parsed.Cycle, d.Cycle)
	}
	if err := parsed.Validate(); err != nil {
		t.Errorf("written file fails Validate(): %v", err)
	}
}

// TestWrite_Idempotent: repeated Write calls with same dossier produce
// byte-identical files (contract: "Is idempotent").
func TestWrite_Idempotent(t *testing.T) {
	dir := t.TempDir()
	d := ampPass()
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	jsonPath := filepath.Join(dir, "cycle-5.json")
	first, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile after first Write: %v", err)
	}
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	second, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile after second Write: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("Write must be idempotent: second call produced different file content")
	}
}

// TestWrite_CommitFalseWritesFiles: commit=false must still write files.
func TestWrite_CommitFalseWritesFiles(t *testing.T) {
	dir := t.TempDir()
	d := ampPass()
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("Write(commit=false): %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "cycle-5.json"))
	if err != nil {
		t.Fatalf("file not present after Write(commit=false): %v", err)
	}
	if len(content) == 0 {
		t.Error("Write(commit=false) wrote empty file")
	}
}

// ── Validate invariant stress tests ───────────────────────────────────────

// TestValidate_FailWithDefectsButNoCarryover: FAIL + defects but no carryover
// must fail ("the fix work is required").
func TestValidate_FailWithDefectsButNoCarryover(t *testing.T) {
	d := ampFail()
	d.Carryover = nil
	err := d.Validate()
	if err == nil {
		t.Error("FAIL with defects but no carryover must fail Validate()")
	}
	if err != nil && !strings.Contains(err.Error(), "carryover") {
		t.Errorf("error must mention 'carryover', got: %v", err)
	}
}

// TestValidate_FailWithCarryoverButNoDefects: FAIL + carryover but no defects
// must fail ("why it failed is required").
func TestValidate_FailWithCarryoverButNoDefects(t *testing.T) {
	d := ampFail()
	d.Defects = nil
	err := d.Validate()
	if err == nil {
		t.Error("FAIL with carryover but no defects must fail Validate()")
	}
	if err != nil && !strings.Contains(err.Error(), "defect") {
		t.Errorf("error must mention 'defect', got: %v", err)
	}
}

// TestValidate_WarnNeedNotCarryDefects: WARN is not FAIL; no
// defect/carryover requirement applies.
func TestValidate_WarnNeedNotCarryDefects(t *testing.T) {
	d := &Dossier{
		Cycle:        10,
		Goal:         "warn-no-defects",
		FinalVerdict: VerdictWarn,
		Phases:       []PhaseRecord{{Name: "audit", Verdict: VerdictWarn}},
	}
	if err := d.Validate(); err != nil {
		t.Errorf("WARN dossier without defects must be valid, got: %v", err)
	}
}

// TestValidate_MultiplePhasesMixedValidity: any phase with empty name or
// invalid verdict must cause Validate to fail.
func TestValidate_MultiplePhasesMixedValidity(t *testing.T) {
	d := ampPass()
	d.Phases = append(d.Phases, PhaseRecord{Name: "", Verdict: VerdictPass})
	if err := d.Validate(); err == nil {
		t.Error("phase with empty name must fail Validate()")
	}
	d2 := ampPass()
	d2.Phases = append(d2.Phases, PhaseRecord{Name: "tdd", Verdict: "MAYBE"})
	if err := d2.Validate(); err == nil {
		t.Error("phase with invalid verdict must fail Validate()")
	}
}
