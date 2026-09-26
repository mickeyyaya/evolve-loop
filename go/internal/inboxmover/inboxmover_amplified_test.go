package inboxmover

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseCycleProcessing_EntirelyMissingProcessingDir(t *testing.T) {
	repo := makeRepo(t)
	procRoot := filepath.Join(repo, ".evolve", "inbox", "processing")
	if _, err := os.Stat(procRoot); err == nil {
		t.Skip("processing/ dir was created by makeRepo — test precondition violated")
	}

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 42)
	if err != nil {
		t.Fatalf("entirely absent processing/ must be a clean no-op; got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d, want 0 (nothing to recover from an absent tree)", res.Recovered)
	}
}

func TestReleaseCycleProcessing_EmptyCycleDir(t *testing.T) {
	repo := makeRepo(t)
	cycleDir := filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-77")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 77)
	if err != nil {
		t.Fatalf("empty cycle dir must be a clean no-op; got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d, want 0 (empty dir has nothing to release)", res.Recovered)
	}
}

func TestReleaseCycleProcessing_NilStderrNoPanicOnDoubleMove(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "dup.json", "dup-original")
	dropProcessingFile(t, repo, "5", "dup.json", "dup-claimed")

	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo}, 5)
	if err != nil {
		t.Fatalf("double-move with nil Stderr must not error; got err = %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "dup.json"))
	if readErr != nil {
		t.Fatalf("inbox-root dup.json missing: %v", readErr)
	}
	if !strings.Contains(string(body), "dup-original") {
		t.Errorf("inbox-root file was clobbered with nil Stderr: %s", body)
	}
	_ = res
}

func TestReleaseCycleProcessing_MixedOutcome(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "10", "clean1.json", "clean1")
	dropProcessingFile(t, repo, "10", "clean2.json", "clean2")
	dropInboxFile(t, repo, "coll.json", "coll-original")
	dropProcessingFile(t, repo, "10", "coll.json", "coll-claimed")

	var stderr bytes.Buffer
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo, Stderr: &stderr}, 10)
	if err != nil {
		t.Fatalf("mixed-outcome release must not error; got err = %v", err)
	}
	if res.Recovered != 2 {
		t.Errorf("Recovered = %d, want 2 (only the two clean items count)", res.Recovered)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("collision must emit WARN to stderr; got %q", stderr.String())
	}
	for _, id := range []string{"clean1.json", "clean2.json"} {
		if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", id)); err != nil {
			t.Errorf("%s not released to inbox root: %v", id, err)
		}
	}
	body, readErr := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "coll.json"))
	if readErr != nil {
		t.Fatalf("inbox-root coll.json missing: %v", readErr)
	}
	if !strings.Contains(string(body), "coll-original") {
		t.Errorf("inbox-root collision file was clobbered: %s", body)
	}
}

func TestReleaseCycleProcessing_DoubleMove_NotCountedAsRecovered(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "x.json", "x-original")
	dropProcessingFile(t, repo, "3", "x.json", "x-claimed")

	var stderr bytes.Buffer
	res, err := ReleaseCycleProcessing(Options{ProjectRoot: repo, Stderr: &stderr}, 3)
	if err != nil {
		t.Fatalf("collision must not error; got err = %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("Recovered = %d for a pure-collision release; want 0 (no successful move)", res.Recovered)
	}
}
