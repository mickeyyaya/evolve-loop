package phaseobserver

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge/inbox"
)

// phaseobserver_nudge_test.go — the soft-stall nudge hook: before the hard
// SIGTERM, the observer appends a single nudge envelope to the agent inbox so
// a draining *-tmux driver can prompt the agent to continue or finalize.

func TestRun_SoftStallNudge_AppendsOnceBelowKillThreshold(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		if callIdx <= 2 {
			return startTime
		}
		return startTime.Add(400 * time.Second) // > NudgeS(300), < StallS(600)
	}
	killCalls := 0
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, NudgeS: 300, EOFGraceS: 9999,
		Enforce: true,
		Now:     nowFn,
		KillPgrp: func(int, syscall.Signal) error {
			mu.Lock()
			killCalls++
			mu.Unlock()
			return nil
		},
		StopAfterMS: 800,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	mu.Lock()
	defer mu.Unlock()
	if killCalls != 0 {
		t.Errorf("kill must NOT fire below StallS; got %d", killCalls)
	}

	// Exactly one nudge envelope, regardless of how many poll ticks crossed
	// the threshold (nudged flag dedupes).
	envs, err := inbox.NewCursor(ws, "builder").Drain()
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("want exactly 1 nudge envelope, got %d: %+v", len(envs), envs)
	}
	if envs[0].Kind != inbox.KindNudge || envs[0].Source != "observer" {
		t.Errorf("unexpected nudge envelope: %+v", envs[0])
	}

	events, _ := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if !strings.Contains(string(events), "soft_stall_nudge") {
		t.Errorf("missing soft_stall_nudge event:\n%s", events)
	}
}
