package phaseobserver

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names AND
// exercises the exported symbols apicover flagged UNCOVERED:
//
//	type  Scope, const ScopeCycle, const ScopePhase — via the observer's scope
//	      handling: Run defaults an empty Scope to ScopePhase (phaseobserver.go
//	      L207-209) and stamps string(cfg.Scope) into the observer_started event
//	      (L240). We drive Run end-to-end for each enum value and read the field
//	      back out of the events.ndjson the observer actually wrote.
//	func  DefaultProcessAlive — the production R3.4 liveness probe; invoked
//	      against a real, live process group (alive) and a bogus pgid (dead).
//
// (The dead ExitFatal=1 enum member — zero consumers tree-wide; Run returns only
// ExitOK/ExitInvalidArgs — was deleted rather than pinned with a value test.)

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// scopeFromStartedEvent runs the observer to shutdown and returns the "scope"
// field the observer_started envelope recorded — i.e. string(cfg.Scope) after
// Run applied its defaulting. This reads scope handling through real production
// code, never off a struct literal the test itself populated.
func scopeFromStartedEvent(t *testing.T, configured Scope) string {
	t.Helper()
	ws := tempWorkspace(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	shutdown := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		close(shutdown)
	}()
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		Scope:       configured,
		PollS:       1,
		StallS:      9999,
		EOFGraceS:   9999,
		Now:         func() time.Time { return now },
		ShutdownSig: shutdown,
		StopAfterMS: 200,
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

// TestScope_CycleAndPhaseRoundTripThroughObserver — ScopeCycle and ScopePhase
// (and the Scope type they inhabit) flow through Run into the recorded event.
// Contract: an explicit ScopeCycle is preserved verbatim; an explicit ScopePhase
// is preserved verbatim; and the two enum members are distinct.
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

// TestScope_EmptyDefaultsToPhase — the documented default (phaseobserver.go
// L40, L207-209): an unset Scope is normalized to ScopePhase by Run, observable
// in the event the observer emits. This exercises Scope's zero-value path
// through production defaulting rather than asserting it on a literal.
func TestScope_EmptyDefaultsToPhase(t *testing.T) {
	t.Parallel()
	var unset Scope // zero value ""
	if got := scopeFromStartedEvent(t, unset); got != string(ScopePhase) {
		t.Errorf("empty Scope must default to ScopePhase; observer recorded scope=%q, want %q", got, string(ScopePhase))
	}
}

// TestDefaultProcessAlive_LiveVsDead — the R3.4 liveness probe. signal-0 to a
// real, current process group must report alive; the same probe against a pgid
// that cannot exist must report dead. (DefaultProcessAlive sends to -pgid, so
// the argument must be a real process-GROUP id — we use the test's own pgrp.)
func TestDefaultProcessAlive_LiveVsDead(t *testing.T) {
	t.Parallel()
	livePgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatalf("Getpgid(self): %v", err)
	}
	if !DefaultProcessAlive(livePgid) {
		t.Errorf("DefaultProcessAlive(%d) = false for the test's own live process group; want true", livePgid)
	}
	// A pgid above the kernel pid_max ceiling cannot name any live group;
	// signal-0 there returns ESRCH, which the probe maps to dead.
	const bogusPgid = 0x7FFFFFFF
	if DefaultProcessAlive(bogusPgid) {
		t.Errorf("DefaultProcessAlive(%d) = true for a non-existent process group; want false", bogusPgid)
	}
}
