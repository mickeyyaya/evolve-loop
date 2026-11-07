package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func smWriteLaneLease(t *testing.T, repo, laneID string, heartbeat time.Time) {
	t.Helper()
	laneRunDir := filepath.Join(repo, ".evolve", "runs", laneID)
	if err := os.MkdirAll(laneRunDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(laneRunDir, runlease.Lease{RunID: laneID, OwnerPID: os.Getpid()}, heartbeat); err != nil {
		t.Fatalf("runlease.Write: %v", err)
	}
}

func smRequireNoRootCycleState(t *testing.T, repo string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "cycle-state.json")); !os.IsNotExist(err) {
		t.Fatalf("fixture must have no root cycle-state.json: %v", err)
	}
}

func TestSyncMain_RefusesOnLiveLaneLeaseWithoutRootCycleState(t *testing.T) {
	t.Parallel()
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	smWriteLaneLease(t, repo, "lane-a", time.Now())
	smRequireNoRootCycleState(t, repo)
	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != cliExitRefused {
		t.Fatalf("expected refusal (exit %d) while a lane lease is live, got exit %d\nstdout=%s\nstderr=%s", cliExitRefused, code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "run lease is live") {
		t.Errorf("refusal must say a run lease is live, got stderr=%q", errb.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged on refusal: before=%s after=%s", headBefore, got)
	}
	if _, err := os.Stat(filepath.Join(repo, "remote-change.txt")); !os.IsNotExist(err) {
		t.Errorf("nothing may be merged on refusal, remote-change.txt present: %v", err)
	}
}

func TestSyncMain_RefusesWhenOneOfSeveralLaneLeasesIsLive(t *testing.T) {
	t.Parallel()
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	smWriteLaneLease(t, repo, "lane-stale", time.Now().Add(-2*runlease.DefaultTTL))
	smWriteLaneLease(t, repo, "lane-live", time.Now())
	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != cliExitRefused {
		t.Fatalf("expected refusal while any lane lease is live, got exit %d\nstderr=%s", code, errb.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged on refusal: before=%s after=%s", headBefore, got)
	}
}

func TestSyncMain_MergesWhenNoRunIsLive(t *testing.T) {
	t.Parallel()
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	smWriteLaneLease(t, repo, "lane-stale", time.Now().Add(-2*runlease.DefaultTTL))
	if err := os.MkdirAll(filepath.Join(repo, ".evolve", "runs", "lane-no-lease"), 0o755); err != nil {
		t.Fatal(err)
	}
	smRequireNoRootCycleState(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("expected exit 0 with no live run, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "remote-change.txt")); err != nil {
		t.Errorf("origin's commit must be merged when no run is live: %v", err)
	}
}
