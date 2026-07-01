package bridge

// driver_tmux_repl_idlereached_busyof_test.go — RED tests for cycle-434
// slice S4 completion (s4-complete-residual-busy-callsites, Task 1): the
// idle_reached correlation-span bracket (driver_tmux_repl.go:587) is the
// second surviving direct panestream.PaneBusy consumer the S4 charter
// targeted (scout finding F2). It must route through
// panestream.SignalCenter.BusyOf instead of calling panestream.PaneBusy
// inline, so the SignalCenter remains the sole liveness facade (ADR-0068).
//
// AC2 (the bracket fires idle_reached exactly once on a real busy→idle
// transition) is already pinned end-to-end against real captured claude
// frames by TestChannelE2E_RealFixtures_ClaudeSpan (channel_e2e_test.go) —
// pre-existing GREEN, unaffected by this migration because BusyOf delegates
// to the SAME PaneBusy definition (H1: verdict-identical). This file adds
// only the AC3 discriminating negative test.
//
// TDD contract: written BEFORE the migration lands. Fails today (RED)
// because the bracket still calls panestream.PaneBusy inline. DO NOT MODIFY
// THIS TEST — Builder migrates the call site to make it pass.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// idleReachedBracketRegionSource extracts the correlation-span busy/idle
// bracket from driver_tmux_repl.go, anchored on stable, unique surrounding
// lines, so a future reflow can't silently narrow the scanned region without
// also updating this test.
func idleReachedBracketRegionSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "driver_tmux_repl.go"))
	if err != nil {
		t.Fatalf("read driver_tmux_repl.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	start, end := -1, -1
	for i, ln := range lines {
		if start == -1 && strings.Contains(ln, "Bracket the open ask") {
			start = i
		}
		if start != -1 && strings.Contains(ln, `openCorrID = ""`) {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatal("could not locate the idle_reached bracket region markers in driver_tmux_repl.go")
	}
	return strings.Join(lines[start:end+1], "\n")
}

// TestRunTmuxREPL_NoDirectChromeParseAtIdleReachedBracket (AC3, negative —
// discriminating anti-gaming test): the idle_reached bracket must no longer
// call panestream.PaneBusy( directly. This is the test that defeats the
// "keep the direct call AND also route through the center" cheapest fake —
// it fails today (RED) because driver_tmux_repl.go:587 still calls
// panestream.PaneBusy( inline.
func TestRunTmuxREPL_NoDirectChromeParseAtIdleReachedBracket(t *testing.T) {
	region := idleReachedBracketRegionSource(t)
	if strings.Contains(region, "panestream.PaneBusy(") {
		t.Error("idle_reached bracket still calls panestream.PaneBusy( directly — must read it via panestream.SignalCenter.BusyOf instead")
	}
}
