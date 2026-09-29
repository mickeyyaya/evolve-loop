package releasepipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRun_ADryRunReusesOneTempJournalPerVersionInsteadOfOnePerProcess(t *testing.T) {
	perProcess := filepath.Join(os.TempDir(), fmt.Sprintf("release-pipeline-dryrun-%d.json", os.Getpid()))
	_ = os.Remove(perProcess)

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		DryRun:      true,
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("dry run err = %v", err)
	}

	if want := filepath.Join(os.TempDir(), "release-pipeline-dryrun-1.2.3.json"); res.JournalPath != want {
		t.Errorf("dry-run journal at %s, want %s: a per-process name adds a file on every run", res.JournalPath, want)
	}
	if _, err := os.Stat(perProcess); !os.IsNotExist(err) {
		t.Errorf("a dry run still wrote the per-process journal %s", perProcess)
	}
}

func TestRun_ADryRunHonorsJournalDir(t *testing.T) {
	dir := t.TempDir()

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		JournalDir:  dir,
		FromTag:     "v1.2.2",
		DryRun:      true,
		MaxPollWait: time.Second,
		Now:         fixedNow(t),
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("dry run err = %v", err)
	}

	if filepath.Dir(res.JournalPath) != dir {
		t.Errorf("dry-run journal at %s, want under JournalDir %s", res.JournalPath, dir)
	}
}
