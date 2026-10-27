package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func TestGC_TakesItsLeaseAndSkipsLiveRuns(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	var reaped []string
	gcInjectedReapers = &gcReapers{
		sessions: func(context.Context) swarm.OrphanReapReport {
			reaped = append(reaped, "sessions")
			return swarm.OrphanReapReport{}
		},
		sockets: func(context.Context) swarm.OrphanSocketReport {
			reaped = append(reaped, "sockets")
			return swarm.OrphanSocketReport{}
		},
	}
	listProcesses := gcInjectedProcessHost.list
	gcInjectedProcessHost.list = func(ctx context.Context) ([]proctree.Process, error) {
		reaped = append(reaped, "processes")
		return listProcesses(ctx)
	}
	liveLaneLease(t, root, 0)
	worktree := filepath.Join(root, ".evolve", "worktrees", "cycle-1790")
	contractUnitWrite(t, worktree, "work.txt", "a live lane's work\n")
	var stdout, stderr bytes.Buffer

	rc := runGC([]string{"--json", "--project-root", root}, nil, &stdout, &stderr)

	var got gcResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || rc != cliExitOK || got.Scope != "all" || strings.Join(got.LiveRuns, ",") != "cycle-1790" {
		t.Fatalf("rc %d result %+v (%v) stderr %q, want the full scope and the live run named", rc, got, err, stderr.String())
	}
	if len(reaped) != 0 {
		t.Errorf("gc reaped %v while a run was live", reaped)
	}
	if _, err := os.Stat(filepath.Join(worktree, "work.txt")); err != nil {
		t.Errorf("the live lane's worktree is gone: %v", err)
	}
	if !strings.Contains(stderr.String(), "the sweeps that stop processes") {
		t.Errorf("stderr %q must say why the other sweeps wait", stderr.String())
	}

	release, err := verifylock.AcquireAt(context.Background(), filepath.Join(root, ".evolve", gcLeaseFile), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	stderr.Reset()
	if rc := runGC([]string{"--lease-wait", "300ms", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitRefused || !strings.Contains(stderr.String(), "another gc holds the gc lease") {
		t.Errorf("a second gc: rc %d stderr %q, want exit 3", rc, stderr.String())
	}
	if rc := runGC([]string{"--dry-run", "--only", "cache", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Errorf("a dry run takes no lease: rc %d stderr %q", rc, stderr.String())
	}
}

func TestGC_WithNoLiveRunRunsTheProcessSweeps(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	var reaped []string
	gcInjectedReapers = &gcReapers{
		sessions: func(context.Context) swarm.OrphanReapReport {
			reaped = append(reaped, "sessions")
			return swarm.OrphanReapReport{}
		},
		sockets: func(context.Context) swarm.OrphanSocketReport {
			reaped = append(reaped, "sockets")
			return swarm.OrphanSocketReport{}
		},
	}
	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK || strings.Join(reaped, ",") != "sessions,sockets" {
		t.Fatalf("rc %d reaped %v stderr %q, want both reapers with no live run", rc, reaped, stderr.String())
	}
}
