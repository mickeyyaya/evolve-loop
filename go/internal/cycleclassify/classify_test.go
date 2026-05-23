package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

// writeReport seeds a workspace with an orchestrator-report.md.
func writeReport(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return ws
}

func TestClassify_NoReport_IntegrityBreach(t *testing.T) {
	t.Parallel()
	r := Classify(t.TempDir())
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach", r.Class)
	}
	if r.Marker != "" || r.Source != "" {
		t.Fatalf("marker/source should be empty for breach; got %+v", r)
	}
}

func TestClassify_EmptyReport_IntegrityBreach(t *testing.T) {
	t.Parallel()
	r := Classify(writeReport(t, ""))
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("empty report → %q want integrity-breach", r.Class)
	}
}

func TestClassify_Infrastructure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"EPERM", "build started\nEPERM: operation not permitted\n"},
		{"rate-limit", "API call: 429 Too Many Requests"},
		{"overload-529", "got 529 Overloaded from anthropic"},
		{"timeout", "ETIMEDOUT after 30s"},
		{"operation-timed-out", "phase exit: operation timed out"},
		{"sandbox-eperm", "sandbox-exec: deny() Operation not permitted"},
		{"explicit-marker", "INFRASTRUCTURE FAILURE: see above"},
		{"connection-refused", "connect: connection refused"},
		{"sandbox-apply", "sandbox_apply: deny not permitted"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, tc.body))
			if r.Class != ClassInfrastructure {
				t.Fatalf("body=%q → %q want infrastructure (marker=%q)", tc.body, r.Class, r.Marker)
			}
			if r.Marker == "" {
				t.Fatalf("expected non-empty marker")
			}
		})
	}
}

func TestClassify_InfraInStderrLog(t *testing.T) {
	t.Parallel()
	// Report is clean; stderr log has the 529. Per cycle-61 forensics,
	// the classifier must catch this.
	ws := writeReport(t, "## Verdict\nNo errors detected")
	if err := os.WriteFile(filepath.Join(ws, "memo-stdout.log"), []byte("429 Too Many Requests"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure from stdout-log scan; marker=%q source=%q", r.Class, r.Marker, r.Source)
	}
	if r.Source != "memo-stdout.log" {
		t.Fatalf("source=%q want memo-stdout.log", r.Source)
	}
}

func TestClassify_InfraInStderrLogSuffix(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "OK")
	if err := os.WriteFile(filepath.Join(ws, "builder-stderr.log"), []byte("ETIMEDOUT"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure", r.Class)
	}
	if r.Source != "builder-stderr.log" {
		t.Fatalf("source=%q want builder-stderr.log", r.Source)
	}
}

func TestClassify_ShipGateConfig(t *testing.T) {
	t.Parallel()
	tests := []string{
		"SHIP_GATE_DENIED: see audit report",
		"shipgate rejected commit",
		"ship-gate denied at HEAD",
		"integrity-fail: Auditor exited with 1",
	}
	for _, body := range tests {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, body))
			if r.Class != ClassShipGateConfig {
				t.Fatalf("body=%q → %q want ship-gate-config (marker=%q)", body, r.Class, r.Marker)
			}
		})
	}
}

func TestClassify_ShipGateBeatsAuditFail(t *testing.T) {
	t.Parallel()
	// Both markers present on their own lines — ship-gate-config must win.
	body := `
Verdict: FAIL
But actually SHIP_GATE_DENIED — the audit was PASS originally.
`
	r := Classify(writeReport(t, body))
	if r.Class != ClassShipGateConfig {
		t.Fatalf("got %q want ship-gate-config (ship-gate must beat audit-fail)", r.Class)
	}
}

func TestClassify_AuditFail(t *testing.T) {
	t.Parallel()
	// Markers must hit on a single line — bash grep -qiE is line-by-line
	// and Go regex with the default `.` (no newline) replicates that.
	// The split-line "## Verdict\n**FAIL**" form belongs to audit
	// reports, not orchestrator-report.md.
	for _, body := range []string{
		"**Verdict: FAIL** — defects above threshold\n",
		"Verdict: WARN — defects above threshold",
		"verdict: fail",
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, body))
			if r.Class != ClassAuditFail {
				t.Fatalf("body=%q → %q want audit-fail", body, r.Class)
			}
		})
	}
}

func TestClassify_BuildFail(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"Build status: FAIL — 3/12 tests RED",
		"tests RED across the board",
		"builder failed after 3 retries",
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			r := Classify(writeReport(t, body))
			if r.Class != ClassBuildFail {
				t.Fatalf("body=%q → %q want build-fail", body, r.Class)
			}
		})
	}
}

func TestClassify_InfraBeatsEverything(t *testing.T) {
	t.Parallel()
	body := `
Verdict: FAIL
Build status: FAIL — tests RED
SHIP_GATE_DENIED
EPERM: sandbox blocked write
`
	r := Classify(writeReport(t, body))
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure (highest priority)", r.Class)
	}
}

func TestClassify_UnclassifiableReport_Breach(t *testing.T) {
	t.Parallel()
	// Report exists but has no recognized markers.
	body := "Cycle completed. Nothing surprising to report.\nVerdict: SHIPPED"
	r := Classify(writeReport(t, body))
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach (no markers)", r.Class)
	}
}

func TestClassify_SortedLogScan(t *testing.T) {
	t.Parallel()
	// Two logs both contain an infra marker; classifier picks the
	// alphabetically-first one for stable Source value.
	ws := writeReport(t, "OK")
	for _, name := range []string{"zeta-stdout.log", "alpha-stdout.log"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("EPERM"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Fatalf("got %q want infrastructure", r.Class)
	}
	if r.Source != "alpha-stdout.log" {
		t.Fatalf("source=%q want alpha-stdout.log (sorted scan)", r.Source)
	}
}
