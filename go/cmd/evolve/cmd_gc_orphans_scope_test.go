package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func failOnSessionReap(t *testing.T) {
	t.Helper()
	prev := gcInjectedReapers
	gcInjectedReapers = &gcReapers{
		sessions: func(context.Context) swarm.OrphanReapReport {
			t.Error("a mid-wave scope reaped tmux sessions")
			return swarm.OrphanReapReport{}
		},
		sockets: func(context.Context) swarm.OrphanSocketReport {
			t.Error("a mid-wave scope reaped sockets")
			return swarm.OrphanSocketReport{}
		},
	}
	t.Cleanup(func() { gcInjectedReapers = prev })
}

func liveLane(t *testing.T, projectRoot string, cycle int) string {
	t.Helper()
	tree := gcClosedOutCycleTree(t, projectRoot, cycle)
	runDir := filepath.Join(projectRoot, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVELANE", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestRunGC_OnlyOrphansTerminatesAFinishedCycleOrphanWhileALaneHoldsALiveLease(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	failOnSessionReap(t)
	liveLane(t, projectRoot, 1801)
	pid := spawnOrphanIn(t, gcClosedOutCycleTree(t, projectRoot, 1700))
	label := "pid=" + strconv.Itoa(pid)

	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--only", "orphans", "--project-root", projectRoot}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("exit %d\nstdout=%s\nstderr=%s", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "terminated "+label) {
		t.Fatalf("the orphans scope does not report terminating %s:\n%s\nstderr=%s", label, stdout.String(), stderr.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Errorf("orphan %s still runs after `evolve gc --only orphans`", label)
	}
	if strings.Contains(stdout.String(), "workspace") || strings.Contains(stdout.String(), "go build cache") {
		t.Errorf("the orphans scope ran another sweep:\n%s", stdout.String())
	}
}

func TestRunGC_OnlyOrphansSparesAProcessAndTheWorktreeOfALiveLane(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	failOnSessionReap(t)
	tree := liveLane(t, projectRoot, 1802)
	pid := spawnOrphanIn(t, tree)

	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--only", "orphans", "--project-root", projectRoot}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("exit %d\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "terminated 0 finished-cycle orphan process(es)") {
		t.Errorf("a live lane's process was a target:\n%s", stdout.String())
	}
	time.Sleep(100 * time.Millisecond)
	if !processAlive(pid) {
		t.Fatalf("the orphans scope stopped pid %d of a live lane", pid)
	}
	if _, err := os.Stat(tree); err != nil {
		t.Fatalf("the orphans scope touched the live lane's worktree: %v", err)
	}
}

func TestRunGC_OnlyOrphansDryRunNamesTheTargetAndStopsNothing(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	failOnSessionReap(t)
	pid := spawnOrphanIn(t, gcClosedOutCycleTree(t, projectRoot, 1703))

	var stdout, stderr bytes.Buffer
	runGC([]string{"--only", "orphans", "--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "WOULD-TERMINATE pid="+strconv.Itoa(pid)) {
		t.Fatalf("the preview does not name the orphan:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if !processAlive(pid) {
		t.Fatal("a preview stopped a process")
	}
}

func TestGCRun_OrphanScopeFailsWhenTheProjectRootIsUnresolvable(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)
	r.abs = func(string) (string, error) { return "", os.ErrPermission }
	if rc := r.orphanScope("rel"); rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
}

func TestRunGC_OnlyOrphansFailsWhenTheProcessListFails(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	failOnSessionReap(t)
	binDir := t.TempDir()
	fakeclitest.Install(t, filepath.Join(binDir, "lsof"), "#!/bin/sh\necho 'lsof: broken' >&2\nexit 2\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--only", "orphans", "--project-root", projectRoot}, nil, &stdout, &stderr); rc != 1 {
		t.Fatalf("rc = %d, want 1 on a failed process list\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "evolve gc: process error:") {
		t.Fatalf("stderr = %q, want the process error named", stderr.String())
	}
}
