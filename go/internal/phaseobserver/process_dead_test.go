package phaseobserver

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func runWithProcessAlive(t *testing.T, alive bool) (events string, killCalls int) {
	t.Helper()
	ws := tempWorkspace(t)
	var mu sync.Mutex
	kills := 0
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1,
		Phase: "build", Agent: "builder",
		// A huge StallS leaves the process probe as the only rule that can fire.
		PollS: 1, StallS: 99999, EOFGraceS: 9999,
		StallPolicy:  recovery.NewChainStallPolicy(6),
		ProcessAlive: func(pgid int) bool { return alive },
		KillPgrp: func(int, syscall.Signal) error {
			mu.Lock()
			kills++
			mu.Unlock()
			return nil
		},
		StopAfterMS: 800,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events file: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return string(raw), kills
}

func TestRun_DeadProcessFiresOnceAndKills(t *testing.T) {
	t.Parallel()
	events, kills := runWithProcessAlive(t, false)

	n := strings.Count(events, `"process_dead"`)
	if n == 0 {
		t.Fatalf("RED (cycle-274): dead process group produced no process_dead INCIDENT — pane/log echo is the only liveness signal today:\n%s", events)
	}
	if n > 2 { // one envelope names the kind at most twice (type and payload)
		t.Errorf("process_dead emitted %d times — must fire ONCE, not per tick:\n%s", n, events)
	}
	if !strings.Contains(events, "kill_retry") {
		t.Errorf("stall policy verdict (kill_retry) missing from the incident envelope:\n%s", events)
	}
	if kills == 0 {
		t.Errorf("kill_retry with a pgid present must invoke KillPgrp (record-reflects-reality)")
	}
}

func TestRun_LiveProcessNoIncident(t *testing.T) {
	t.Parallel()
	events, _ := runWithProcessAlive(t, true)
	if strings.Contains(events, "process_dead") {
		t.Errorf("live process must produce no process_dead incident:\n%s", events)
	}
}
