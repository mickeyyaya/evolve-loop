package coherence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCycleVerdicts_Fixture(t *testing.T) {
	dir := t.TempDir()
	audit := "## Verdict\n**PASS**\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(audit), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "acs-verdict.json"), []byte(`{"verdict":"PASS","red_count":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	gotA, gotACS, ran := ReadCycleVerdicts(dir)
	if !ran || gotA != "PASS" || gotACS != "PASS" {
		t.Fatalf("ReadCycleVerdicts = audit=%q acs=%q ran=%v; want PASS/PASS/true", gotA, gotACS, ran)
	}
	// Absent artifacts → empty, auditRan=false, no fabrication.
	a2, acs2, ran2 := ReadCycleVerdicts(t.TempDir())
	if ran2 || a2 != "" || acs2 != "" {
		t.Errorf("absent workspace = audit=%q acs=%q ran=%v; want empty/empty/false", a2, acs2, ran2)
	}
}

// ADR-0072 S2: the verdict-coherence signal. A recorded FAIL/WARN is only
// trustworthy if the phases' own on-disk artifacts agree. When the audit report
// says PASS and ACS says PASS but the cycle recorded FAIL/WARN, the pipeline
// forged the verdict — that is verdict-incoherence, the clean-exit signature
// that must halt (not retry). This is the exact fingerprint of cycles 862→899.

func TestCheckVerdictCoherence(t *testing.T) {
	tests := []struct {
		name      string
		in        VerdictInputs
		wantIncoh bool
	}{
		{
			name:      "clean-exit signature: recorded FAIL but audit+acs PASS",
			in:        VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true},
			wantIncoh: true,
		},
		{
			name:      "WARN variant is also incoherent when artifacts are green",
			in:        VerdictInputs{Recorded: "WARN", Audit: "PASS", ACS: "PASS", AuditRan: true},
			wantIncoh: true,
		},
		{
			name:      "genuine audit FAIL is coherent",
			in:        VerdictInputs{Recorded: "FAIL", Audit: "FAIL", ACS: "PASS", AuditRan: true},
			wantIncoh: false,
		},
		{
			name:      "genuine ACS FAIL is coherent even if audit PASS",
			in:        VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "FAIL", AuditRan: true},
			wantIncoh: false,
		},
		{
			name:      "recorded PASS is never incoherent",
			in:        VerdictInputs{Recorded: "PASS", Audit: "PASS", ACS: "PASS", AuditRan: true},
			wantIncoh: false,
		},
		{
			name:      "audit did not run (genuine incomplete) is coherent",
			in:        VerdictInputs{Recorded: "FAIL", Audit: "", ACS: "", AuditRan: false},
			wantIncoh: false,
		},
		{
			name:      "substantive (non-infra) error justifies the FAIL — coherent",
			in:        VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true, SubstantiveError: true},
			wantIncoh: false,
		},
		{
			name:      "audit PASS but ACS artifact absent — cannot claim incoherence",
			in:        VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "", AuditRan: true},
			wantIncoh: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckVerdictCoherence(tc.in)
			if got.Incoherent != tc.wantIncoh {
				t.Errorf("Incoherent = %v, want %v (evidence: %q)", got.Incoherent, tc.wantIncoh, got.Evidence)
			}
			if got.Incoherent {
				if got.Category != "verdict-incoherence" {
					t.Errorf("Category = %q, want verdict-incoherence", got.Category)
				}
				if got.Evidence == "" {
					t.Error("incoherent result must carry evidence")
				}
			}
		})
	}
}
