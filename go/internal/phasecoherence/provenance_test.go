package phasecoherence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvenanceGate_MissingHeader_ReturnsViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "# Some Report\nNo provenance here."
	expected := ProvenanceFields{Phase: "build", Cycle: 241}
	violations := mustCheckProvenance(t, artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Severity != SeverityWarn {
		t.Errorf("expected Severity %s, got %q", SeverityWarn, violations[0].Severity)
	}
	if violations[0].Kind != "missing-provenance" {
		t.Errorf("expected Kind missing-provenance, got %q", violations[0].Kind)
	}
}

func TestProvenanceGate_ValidHeader_NoViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "<!-- evolve:provenance phase=build cycle=241 tree_sha=abcdef123456 inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "abcdef123456",
		InputsDigest: "digest789",
	}
	violations := mustCheckProvenance(t, artifact, expected)

	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestProvenanceGate_TamperedPhase_ReturnsViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "<!-- evolve:provenance phase=scout cycle=241 tree_sha=abcdef123456 inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "abcdef123456",
		InputsDigest: "digest789",
	}
	violations := mustCheckProvenance(t, artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Severity != SeverityError {
		t.Errorf("expected Severity %s, got %q", SeverityError, violations[0].Severity)
	}
	if violations[0].Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", violations[0].Kind)
	}
}

func TestProvenanceGate_WrongCycle_ReturnsViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "<!-- evolve:provenance phase=build cycle=240 tree_sha=abcdef123456 inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "abcdef123456",
		InputsDigest: "digest789",
	}
	violations := mustCheckProvenance(t, artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Severity != SeverityError {
		t.Errorf("expected Severity %s, got %q", SeverityError, violations[0].Severity)
	}
	if violations[0].Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", violations[0].Kind)
	}
}

func TestProvenanceGate_LedgerCrossCheck(t *testing.T) {
	tmpDir := t.TempDir()
	evolveDir := filepath.Join(tmpDir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EVOLVE_PROJECT_ROOT", tmpDir)

	ledgerPath := filepath.Join(evolveDir, "ledger.jsonl")
	ledgerLine := `{"cycle": 241, "role": "build", "tree_state_sha": "goodsha"}` + "\n"
	if err := os.WriteFile(ledgerPath, []byte(ledgerLine), 0o644); err != nil {
		t.Fatal(err)
	}

	artifact1 := "<!-- evolve:provenance phase=build cycle=241 tree_sha=goodsha inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "goodsha",
		InputsDigest: "digest789",
	}
	violations1 := mustCheckProvenance(t, artifact1, expected)
	if len(violations1) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations1), violations1)
	}

	artifact2 := "<!-- evolve:provenance phase=build cycle=241 tree_sha=badsha inputs_digest=digest789 -->\n# Report"
	violations2 := mustCheckProvenance(t, artifact2, expected)
	if len(violations2) != 1 {
		t.Fatalf("expected 1 violation for bad tree_sha, got %d: %+v", len(violations2), violations2)
	}
	if violations2[0].Severity != SeverityError {
		t.Errorf("expected Severity %s, got %q", SeverityError, violations2[0].Severity)
	}
	if violations2[0].Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", violations2[0].Kind)
	}
}

func mustCheckProvenance(t *testing.T, artifact string, expected ProvenanceFields) []Violation {
	t.Helper()
	violations, err := CheckProvenance(artifact, expected)
	if err != nil {
		t.Fatalf("CheckProvenance: %v", err)
	}
	return violations
}

func TestCheckProvenance_LedgerReadFailuresAreErrors(t *testing.T) {
	artifact := "<!-- evolve:provenance phase=build cycle=241 tree_sha=goodsha inputs_digest=d -->\n# Report"
	expected := ProvenanceFields{Phase: "build", Cycle: 241}
	cases := []struct {
		name  string
		setup func(t *testing.T, ledgerPath string)
	}{
		{"directory at the ledger path", func(t *testing.T, ledgerPath string) {
			if err := os.MkdirAll(ledgerPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"line past the ledger line limit", func(t *testing.T, ledgerPath string) {
			body := `{"cycle":1,"note":"` + strings.Repeat("x", maxLedgerLineBytes) + `"}` + "\n" +
				`{"cycle":241,"role":"builder","tree_state_sha":"latersha"}` + "\n"
			if err := os.WriteFile(ledgerPath, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := hermeticEnv(t)
			if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
				t.Fatal(err)
			}
			ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
			tc.setup(t, ledgerPath)
			violations, err := CheckProvenance(artifact, expected)
			if err == nil || !strings.Contains(err.Error(), "ledger") {
				t.Fatalf("CheckProvenance = %+v, %v; want an error naming the ledger", violations, err)
			}
		})
	}
}

func TestCheckProvenance_AbsentLedgerIsNotAnError(t *testing.T) {
	hermeticEnv(t)
	artifact := "<!-- evolve:provenance phase=build cycle=241 tree_sha=goodsha inputs_digest=d -->\n# Report"
	if got := mustCheckProvenance(t, artifact, ProvenanceFields{Phase: "build", Cycle: 241}); len(got) != 0 {
		t.Errorf("violations = %+v, want none without a ledger", got)
	}
}

func TestCheckProvenance_LedgerScanReportsMalformedAndOversizeLines(t *testing.T) {
	artifact := "<!-- evolve:provenance phase=build cycle=241 tree_sha=goodsha inputs_digest=d -->\n# Report"
	oversize := `{"cycle":1,"note":"` + strings.Repeat("x", 128<<10) + `"}`
	entry := `{"cycle":241,"role":"Build","tree_state_sha":"latersha"}`
	cases := []struct {
		name          string
		lines         []string
		wantMalformed int
		wantMismatch  bool
	}{
		{"oversize line then entry", []string{oversize, entry}, 0, true},
		{"blank lines are not malformed", []string{"", "  ", entry}, 0, true},
		{"two malformed lines yield one count", []string{"{bad", "nope", entry}, 2, true},
		{"malformed only", []string{"{bad"}, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := hermeticEnv(t)
			ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
			if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ledgerPath, []byte(strings.Join(tc.lines, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			violations := mustCheckProvenance(t, artifact, ProvenanceFields{Phase: "build", Cycle: 241})
			var malformed []Violation
			mismatch := false
			for _, v := range violations {
				switch v.Kind {
				case "malformed-ledger":
					malformed = append(malformed, v)
				case "provenance-mismatch":
					mismatch = true
				}
			}
			if tc.wantMalformed == 0 && len(malformed) != 0 {
				t.Errorf("malformed violations = %+v, want none", malformed)
			}
			if tc.wantMalformed > 0 {
				if len(malformed) != 1 || malformed[0].Severity != SeverityWarn ||
					!strings.Contains(malformed[0].Message, fmt.Sprintf("%d malformed line", tc.wantMalformed)) {
					t.Errorf("malformed violations = %+v, want one %s naming %d line(s)", malformed, SeverityWarn, tc.wantMalformed)
				}
			}
			if mismatch != tc.wantMismatch {
				t.Errorf("ledger mismatch = %v, want %v: %+v", mismatch, tc.wantMismatch, violations)
			}
		})
	}
}
