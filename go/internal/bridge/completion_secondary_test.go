package bridge

// completion_secondary_test.go — Phase B pins (plan parallel-weaving-wolf,
// ADR-0084 lineage): the artifact detector must HOLD phase-complete while a
// contract secondary is absent, so the session survives long enough for the
// agent to write it — the single-artifact cutoff killed retro's
// disposition.json on 86/88 recent cycles and audit's
// defect-dispositions.json across 1397-1429. The settle window itself stays
// primary-only (cycle-1210/1212 race design), and phases with no secondaries
// are byte-identical.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func settledPrimary(t *testing.T, dir string) *Config {
	t.Helper()
	p := filepath.Join(dir, "retrospective-report.md")
	if err := os.WriteFile(p, []byte("# report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Config{Workspace: dir, Artifact: p}
}

// pollUntilStable drives the detector past the 2-tick settle window.
func pollUntilStable(t *testing.T, d *artifactDetector) (bool, error) {
	t.Helper()
	var done bool
	var err error
	for i := 0; i < artifactStableTicks+1; i++ {
		done, _, _, err = d.poll(context.Background())
		if err != nil {
			return done, err
		}
	}
	return done, err
}

func TestArtifactDetector_HoldsCompletionWhileSecondaryMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := settledPrimary(t, dir)
	sec := filepath.Join(dir, "disposition.json")
	cfg.SecondaryArtifacts = []string{sec}
	d := &artifactDetector{cfg: cfg}

	done, err := pollUntilStable(t, d)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if done {
		t.Fatal("detector completed with the contract secondary ABSENT — the session would be torn down before the agent can write it (the exact 1397-1429 cutoff)")
	}

	if err := os.WriteFile(sec, []byte(`{"cycle":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	done, err = pollUntilStable(t, d)
	if err != nil {
		t.Fatalf("poll after secondary write: %v", err)
	}
	if !done {
		t.Error("detector must complete once the settled primary is joined by every secondary")
	}
}

func TestArtifactDetector_EmptySecondaryDoesNotCount(t *testing.T) {
	dir := t.TempDir()
	cfg := settledPrimary(t, dir)
	sec := filepath.Join(dir, "disposition.json")
	if err := os.WriteFile(sec, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.SecondaryArtifacts = []string{sec}
	d := &artifactDetector{cfg: cfg}
	if done, _ := pollUntilStable(t, d); done {
		t.Error("a zero-byte secondary is an in-flight write, not a deliverable")
	}
}

func TestArtifactDetector_NoSecondariesIsLegacyBehavior(t *testing.T) {
	d := &artifactDetector{cfg: settledPrimary(t, t.TempDir())}
	if done, err := pollUntilStable(t, d); err != nil || !done {
		t.Errorf("no-secondaries phase must complete exactly as before (done=%v err=%v)", done, err)
	}
}

func TestSplitNonEmptyCSV(t *testing.T) {
	if got := splitNonEmptyCSV(""); got != nil {
		t.Errorf("empty flag must yield nil, got %v", got)
	}
	got := splitNonEmptyCSV("/a.json" + secondaryArtifactSep + " " + secondaryArtifactSep + "/b, with comma.json" + secondaryArtifactSep)
	if len(got) != 2 || got[0] != "/a.json" || got[1] != "/b, with comma.json" {
		t.Errorf("split = %v, want the comma-bearing path intact (unit-separator round trip)", got)
	}
}
