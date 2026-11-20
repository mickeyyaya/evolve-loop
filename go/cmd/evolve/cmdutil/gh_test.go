package cmdutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestGHIn(t *testing.T) {
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = fail ]; then echo boom >&2; exit 1; fi\nprintf ok\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	gh := GHIn(t.TempDir())
	out, err := gh(context.Background(), "pr", "view")
	if err != nil || string(out) != "ok" {
		t.Fatalf("GHIn = %q, %v", out, err)
	}
	if _, err := gh(context.Background(), "fail"); err == nil {
		t.Fatal("expected error for failing gh")
	}
}

func TestPlaneLiveRuns(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	runs := filepath.Join(repo.Dir, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := runlease.Write(runs, runlease.Lease{RunID: "cycle-1", OwnerPID: os.Getpid()}, now); err != nil {
		t.Fatal(err)
	}
	live, err := PlaneLiveRuns(repo.Dir, now)
	if err != nil || len(live) != 1 || live[0].Lease.RunID != "cycle-1" {
		t.Fatalf("PlaneLiveRuns = %+v, %v", live, err)
	}
	if _, err := PlaneLiveRuns(t.TempDir(), now); err == nil {
		t.Fatal("expected error for non-repo dir")
	}
}
