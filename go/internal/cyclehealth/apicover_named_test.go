package cyclehealth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck_MissingArtifact_AnomalyIsSeverityFatal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	if err := os.Remove(filepath.Join(ws, "scout-report.md")); err != nil {
		t.Fatal(err)
	}
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, a := range r.Anomalies {
		if a.Signal == "workspace_artifacts" {
			if a.Severity != SeverityFatal {
				t.Errorf("workspace_artifacts anomaly Severity = %q, want %q", a.Severity, SeverityFatal)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("no workspace_artifacts anomaly produced; anomalies=%+v", r.Anomalies)
	}
}
