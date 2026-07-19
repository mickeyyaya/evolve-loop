package coherence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoherence_ResultShape(t *testing.T) {
	// Names the Coherence result type (apicover) and pins its incoherent shape.
	var got Coherence = CheckVerdictCoherence(VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true})
	if !got.Incoherent || got.Category != "verdict-incoherence" || got.Evidence == "" {
		t.Errorf("Coherence = %+v; want incoherent verdict-incoherence with evidence", got)
	}
}

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

// TestCheckVerdictCoherence_Reconcile — the clean-exit-late-write self-heal: the
// SAME forgery signature (recorded FAIL/WARN, green artifacts) but with a
// FULLY-VALID deliverable (DeliverableValid=true) is a benign timing race →
// Reconciled, NOT Incoherent (no halt). Also pins that DeliverableValid never
// manufactures a reconcile out of a case that is coherent without it — it only
// ever downgrades the would-be halt.
func TestCheckVerdictCoherence_Reconcile(t *testing.T) {
	for _, rec := range []string{"FAIL", "WARN"} {
		got := CheckVerdictCoherence(VerdictInputs{Recorded: rec, Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: true})
		if !got.Reconciled {
			t.Errorf("recorded=%s + green artifacts + valid deliverable: Reconciled=false, want true (%+v)", rec, got)
		}
		if got.Incoherent {
			t.Errorf("recorded=%s + valid deliverable must NOT be Incoherent (no halt) (%+v)", rec, got)
		}
		if got.Category != "verdict-reconciled" || got.Evidence == "" {
			t.Errorf("recorded=%s reconcile result = %+v, want category verdict-reconciled with evidence", rec, got)
		}
	}

	// DeliverableValid only downgrades the forgery signature; a case that is
	// coherent without it must stay the zero Coherence{} even when valid.
	coherentWithValid := []struct {
		name string
		in   VerdictInputs
	}{
		{"recorded PASS", VerdictInputs{Recorded: "PASS", Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: true}},
		{"substantive error", VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true, SubstantiveError: true, DeliverableValid: true}},
		{"audit never ran", VerdictInputs{Recorded: "FAIL", Audit: "", ACS: "PASS", AuditRan: false, DeliverableValid: true}},
		{"acs absent", VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "", AuditRan: true, DeliverableValid: true}},
	}
	for _, tc := range coherentWithValid {
		got := CheckVerdictCoherence(tc.in)
		if got.Reconciled || got.Incoherent {
			t.Errorf("%s: a coherent case must stay zero Coherence{} even with a valid deliverable, got %+v", tc.name, got)
		}
	}
}

// TestCheckVerdictCoherence_ForgedStillHalts — the anti-laundering boundary: the
// forgery signature with a deliverable that does NOT fully verify
// (DeliverableValid=false — a malformed report merely tagged with a PASS
// sentinel) is genuine forgery → still Incoherent (halt), never Reconciled.
func TestCheckVerdictCoherence_ForgedStillHalts(t *testing.T) {
	for _, rec := range []string{"FAIL", "WARN"} {
		got := CheckVerdictCoherence(VerdictInputs{Recorded: rec, Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: false})
		if !got.Incoherent {
			t.Errorf("recorded=%s + green sentinels + INVALID deliverable: Incoherent=false, want true (forgery must halt) (%+v)", rec, got)
		}
		if got.Reconciled {
			t.Errorf("recorded=%s + INVALID deliverable must NOT reconcile (would launder forgery to PASS) (%+v)", rec, got)
		}
		if got.Category != "verdict-incoherence" {
			t.Errorf("recorded=%s category = %q, want verdict-incoherence", rec, got.Category)
		}
	}
}
