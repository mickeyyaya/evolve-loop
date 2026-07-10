package bridge

// autorespond_busyof_migration_test.go — RED tests for cycle-434 slice S4
// completion (s4-complete-residual-busy-callsites, Task 1): the
// autoResponder.tick busy-gate (autorespond.go:282) is one of the two
// surviving direct panestream.PaneBusy consumers the S4 charter targeted
// (scout finding F1). It must route through panestream.SignalCenter.BusyOf
// instead of calling panestream.PaneBusy inline, so the SignalCenter remains
// the sole liveness facade (ADR-0068) and no bridge consumer parses CLI
// chrome directly.
//
// TDD contract: written BEFORE the migration lands. AC1 pins the ALREADY-
// correct busy-gating VALUE through the tick() entry point specifically (the
// existing TestDecideAutoRespond_IdleGatesEscalateWhileBusy only exercises
// decideAutoRespond with a pre-computed bool, never tick()'s own PaneBusy
// call) — it may show pre-existing GREEN for "the value is right" since the
// migration is designed to be behavior-preserving (H1). AC3 is the
// discriminating RED test: it fails today because tick() still calls
// panestream.PaneBusy inline. DO NOT MODIFY THESE TESTS — Builder migrates
// the call site to make AC3 pass without breaking AC1.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate (AC1, positive):
// a pane matching an escalate-policy prompt (the agent quoting a banner in
// its own output, cycle-314 class) while ALSO carrying a live-turn affordance
// must NOT escalate — tick() must return rc 0 (noop), the busy-gated
// suppression decideAutoRespond performs when paneBusy is true.
func TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate(t *testing.T) {
	busyPane := "Which absolute path should I write the deliverable to?\n" +
		"⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"

	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{busyPane}}
	deps := Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.prompts = escalatePrompt

	_, rc := ar.tick(context.Background(), "s")
	if rc != 0 {
		t.Errorf("tick() on busy pane with escalate match: rc = %d, want 0 (busy must gate the escalate policy, not fire it)", rc)
	}
}

// TestAutoResponderTick_IdleGateViaCenter_Escalates (AC1, positive
// counterpart — discriminates a gate that ALWAYS suppresses from one that
// reads the real busy signal): the SAME escalate-matching text on an IDLE
// pane (no live-turn affordance) must still escalate — tick() must return
// rc 85.
func TestAutoResponderTick_IdleGateViaCenter_Escalates(t *testing.T) {
	idlePane := "Which absolute path should I write the deliverable to?\n" +
		"⏺ answer complete\n"

	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: []string{idlePane}}
	deps := Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.prompts = escalatePrompt

	_, rc := ar.tick(context.Background(), "s")
	if rc != 85 {
		t.Errorf("tick() on idle pane with escalate match: rc = %d, want 85 (idle must NOT be gated)", rc)
	}
}

// autorespondTickRegionSource extracts the busy-gate call site inside
// autoResponder.tick from autorespond.go, anchored on stable, unique
// surrounding lines, so a future reflow can't silently narrow the scanned
// region without also updating this test.
func autorespondTickRegionSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "autorespond.go"))
	if err != nil {
		t.Fatalf("read autorespond.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	start, end := -1, -1
	for i, ln := range lines {
		if start == -1 && strings.Contains(ln, "prevCounts := make(map[string]int, len(ar.counts))") {
			start = i
		}
		if start != -1 && strings.Contains(ln, "action, rc := decideAutoRespond(scanPane, ar.prompts, ar.counts, paneBusy)") {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatal("could not locate the tick() busy-gate region markers in autorespond.go")
	}
	return strings.Join(lines[start:end+1], "\n")
}

// TestAutoResponderTick_NoDirectChromeParse (AC3, negative — discriminating
// anti-gaming test): the tick() busy-gate region must no longer call
// panestream.PaneBusy( directly. This is the test that defeats the "keep the
// direct call AND also route through the center" cheapest fake — it fails
// today (RED) because autorespond.go:282 still calls panestream.PaneBusy(
// inline.
func TestAutoResponderTick_NoDirectChromeParse(t *testing.T) {
	region := autorespondTickRegionSource(t)
	if strings.Contains(region, "panestream.PaneBusy(") {
		t.Error("tick() busy-gate region still calls panestream.PaneBusy( directly — must read it via panestream.SignalCenter.BusyOf instead")
	}
}
