package dossier

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuild_IngestsCIWatchVerdict pins AC3 of push-ci-watch-remote-parity
// (cycle-748): the CI verdict recorded by the post-push watch round-trips
// into the cycle dossier, and an absent verdict is never fabricated.
func TestBuild_IngestsCIWatchVerdict(t *testing.T) {
	t.Run("verdict artifact round-trips into the dossier", func(t *testing.T) {
		ws := t.TempDir()
		body := `{"sha":"deadbeefcafe0123","conclusion":"failure","run_url":"https://x/runs/42","failing_test":"TestOrchestrator_TriageLeakRecover","checked_at":"2026-07-13T12:00:00Z"}`
		if err := os.WriteFile(filepath.Join(ws, CIWatchVerdictFile), []byte(body), 0o644); err != nil {
			t.Fatalf("seed verdict: %v", err)
		}
		d, err := Build(748, BuildOpts{WorkspacePath: ws, Goal: "g"})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if d.CIWatch == nil {
			t.Fatal("CIWatch = nil, want the ingested verdict")
		}
		want := CIWatchRecord{
			SHA:         "deadbeefcafe0123",
			Conclusion:  "failure",
			RunURL:      "https://x/runs/42",
			FailingTest: "TestOrchestrator_TriageLeakRecover",
			CheckedAt:   "2026-07-13T12:00:00Z",
		}
		if *d.CIWatch != want {
			t.Errorf("CIWatch = %+v, want %+v", d.CIWatch, want)
		}
		if err := d.Validate(); err != nil {
			t.Errorf("dossier with CI verdict must stay valid: %v", err)
		}
	})

	t.Run("absent verdict is never fabricated", func(t *testing.T) {
		d, err := Build(748, BuildOpts{WorkspacePath: t.TempDir(), Goal: "g"})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if d.CIWatch != nil {
			t.Errorf("CIWatch = %+v, want nil when no verdict artifact exists", d.CIWatch)
		}
	})

	t.Run("malformed verdict artifact is skipped not ingested", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, CIWatchVerdictFile), []byte(`{"sha":""}`), 0o644); err != nil {
			t.Fatalf("seed verdict: %v", err)
		}
		d, err := Build(748, BuildOpts{WorkspacePath: ws, Goal: "g"})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if d.CIWatch != nil {
			t.Errorf("CIWatch = %+v, want nil for an unusable artifact", d.CIWatch)
		}
	})
}
