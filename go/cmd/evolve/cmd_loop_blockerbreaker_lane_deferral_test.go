package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeCycleTiming(t *testing.T, evolveDir string, cycle int, timing string) {
	t.Helper()
	d := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "phase-timing.json"), []byte(timing), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	laneDeferred  = `[{"phase":"start","verdict":"SKIPPED","abort_reason":"lane-worktree-deferred: git fetch origin main: rc=1: error: cannot lock ref 'refs/remotes/origin/main'"}]`
	quotaDeferred = `[{"phase":"build","verdict":"FAIL","abort_reason":"all-families-exhausted: drained"}]`
	shipped       = `[{"phase":"ship","verdict":"PASS"}]`
)

func TestBlockerBreakerHalt_ConsecutiveLaneDeferralsHaltWithTheGitCause(t *testing.T) {
	c, got, _ := recordingLoopSignals()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for cycle := 11; cycle <= 13; cycle++ {
		writeCycleTiming(t, evolveDir, cycle, laneDeferred)
	}

	rc, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard, c)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("three consecutive lane deferrals must halt: halted=%v rc=%d", halted, rc)
	}
	halts := loopHalts(*got)
	if len(halts) != 1 {
		t.Fatalf("one halt is ONE INCIDENT: %+v", halts)
	}
	if e := halts[0]; e.Code != CodeLoopPipelineBlockerHalt || e.Cycle != 13 || e.Fields["rule"] != "lane-deferrals" ||
		e.Fields["category"] != "lane-provisioning" || !strings.Contains(e.Reason, "cannot lock ref") {
		t.Errorf("the INCIDENT names the rule, the category and the git cause: %+v", e)
	}
	entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox"))
	found := false
	for _, e := range entries {
		found = found || strings.Contains(e.Name(), "pipeline-defect-lane-provisioning")
	}
	if !found {
		t.Error("the halt files its P0 pipeline-defect item")
	}
}

func TestBlockerBreakerHalt_ABrokenRunOrQuotaDeferralsDoNotHalt(t *testing.T) {
	cases := map[string][]string{
		"two deferrals, then a ship, then one":       {laneDeferred, laneDeferred, shipped, laneDeferred},
		"three quota deferrals are not provisioning": {quotaDeferred, quotaDeferred, quotaDeferred},
	}
	for name, timings := range cases {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		for i, timing := range timings {
			writeCycleTiming(t, evolveDir, 11+i, timing)
		}
		if _, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard, nil); halted {
			t.Errorf("%s: halted", name)
		}
	}
}

func TestBlockerBreakerHalt_AlternatingDeferralsAndFailuresHalt(t *testing.T) {
	c, got, _ := recordingLoopSignals()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 11, "scout|infra|aaa", "infra")
	writeCycleTiming(t, evolveDir, 12, laneDeferred)
	writeDigestFixture(t, evolveDir, 13, "build|verdict-fail|bbb", "verdict-fail")
	writeCycleTiming(t, evolveDir, 14, laneDeferred)
	writeDigestFixture(t, evolveDir, 15, "audit|verdict-fail|ccc", "verdict-fail")

	rc, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard, c)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("F D F D F must halt: halted=%v rc=%d", halted, rc)
	}
	if halts := loopHalts(*got); len(halts) != 1 || halts[0].Fields["rule"] != "consecutive-failures" {
		t.Errorf("want one consecutive-failures INCIDENT, got %+v", halts)
	}
}
