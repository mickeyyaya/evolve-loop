package inboxmover

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseCycleProcessing_ReleasesScopedCycle(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "7", "a.json", "a")
	dropProcessingFile(t, repo, "9", "b.json", "b")

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 7)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessing err = %v", err)
	}
	if res.Recovered != 1 {
		t.Errorf("Recovered = %d, want 1 (only cycle-7's one item)", res.Recovered)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "a.json")); err != nil {
		t.Errorf("a.json not released to inbox root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-7", "a.json")); err == nil {
		t.Errorf("a.json still in processing/cycle-7 — should have moved")
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-9", "b.json")); err != nil {
		t.Errorf("cycle-9 item wrongly released — release is not scoped to cycle 7: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "b.json")); err == nil {
		t.Errorf("cycle-9's b.json leaked to inbox root — scope violation")
	}
}

func TestReleaseCycleProcessing_Idempotent(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "7", "a.json", "a")

	if _, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 7); err != nil {
		t.Fatalf("first release err = %v", err)
	}
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 7)
	if err != nil {
		t.Fatalf("second release must be a clean no-op, got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("second release Recovered = %d, want 0 (already drained)", res.Recovered)
	}
}

func TestInboxRelease_FailedCycleReleasesAllClaimed(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "12", "t1.json", "t1")
	dropProcessingFile(t, repo, "12", "t2.json", "t2")
	dropProcessingFile(t, repo, "12", "t3.json", "t3")

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 12)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessing err = %v", err)
	}
	if res.Recovered != 3 {
		t.Errorf("Recovered = %d, want 3 (all claimed items released on cycle fail)", res.Recovered)
	}
	for _, id := range []string{"t1", "t2", "t3"} {
		if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", id+".json")); err != nil {
			t.Errorf("%s not released to inbox root: %v", id, err)
		}
	}
	left, _ := os.ReadDir(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-12"))
	if len(left) != 0 {
		t.Errorf("processing/cycle-12 still has %d file(s) — not all claims released", len(left))
	}
}

func TestInboxRelease_DoubleClaimRaceIsWarn(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "dup.json", "dup-original")
	dropProcessingFile(t, repo, "5", "dup.json", "dup-claimed")

	var stderr bytes.Buffer
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo, Stderr: &stderr}, 5)
	if err != nil {
		t.Fatalf("double-move must be a WARN, not an error; got err = %v", err)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("double-move must emit a WARN to stderr; got %q", stderr.String())
	}
	body, readErr := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "dup.json"))
	if readErr != nil {
		t.Fatalf("inbox-root dup.json missing after release: %v", readErr)
	}
	if !strings.Contains(string(body), "dup-original") {
		t.Errorf("inbox-root file was clobbered by the processing copy: %s", body)
	}
	_ = res
}
