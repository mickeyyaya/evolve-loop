package phaseobserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// scopeFromStartedEvent runs the observer to shutdown and returns the scope its observer_started envelope recorded.
func scopeFromStartedEvent(t *testing.T, configured Scope) string {
	t.Helper()
	ws := tempWorkspace(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		Scope:       configured,
		PollS:       1,
		StallS:      9999,
		EOFGraceS:   9999,
		Now:         func() time.Time { return now },
		StopAfterMS: 60,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events file: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var env map[string]any
		if json.Unmarshal([]byte(line), &env) != nil {
			continue
		}
		if env["type"] != "observer_started" {
			continue
		}
		data, _ := env["data"].(map[string]any)
		scope, _ := data["scope"].(string)
		return scope
	}
	t.Fatalf("no observer_started event found in:\n%s", raw)
	return ""
}

func TestScope_CycleAndPhaseRoundTripThroughObserver(t *testing.T) {
	t.Parallel()
	if ScopeCycle == ScopePhase {
		t.Fatalf("ScopeCycle (%q) and ScopePhase (%q) must be distinct enum members", ScopeCycle, ScopePhase)
	}
	if got := scopeFromStartedEvent(t, ScopeCycle); got != string(ScopeCycle) {
		t.Errorf("explicit ScopeCycle: observer_started recorded scope=%q, want %q", got, string(ScopeCycle))
	}
	if got := scopeFromStartedEvent(t, ScopePhase); got != string(ScopePhase) {
		t.Errorf("explicit ScopePhase: observer_started recorded scope=%q, want %q", got, string(ScopePhase))
	}
}

func TestScope_EmptyDefaultsToPhase(t *testing.T) {
	t.Parallel()
	var unset Scope
	if got := scopeFromStartedEvent(t, unset); got != string(ScopePhase) {
		t.Errorf("empty Scope must default to ScopePhase; observer recorded scope=%q, want %q", got, string(ScopePhase))
	}
}

func TestDefaultProcessAlive_LiveVsDead(t *testing.T) {
	t.Parallel()
	// The probe signals -pgid, so the live fixture must be a process-group id, not a pid.
	livePgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatalf("Getpgid(self): %v", err)
	}
	if !DefaultProcessAlive(livePgid) {
		t.Errorf("DefaultProcessAlive(%d) = false for the test's own live process group; want true", livePgid)
	}
	// A pgid above the kernel's pid_max cannot name a live group.
	const bogusPgid = 0x7FFFFFFF
	if DefaultProcessAlive(bogusPgid) {
		t.Errorf("DefaultProcessAlive(%d) = true for a non-existent process group; want false", bogusPgid)
	}
}
