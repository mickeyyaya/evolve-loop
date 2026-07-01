package panestream

import "testing"

// signalcenter_exhaustedof_test.go — ExhaustedOf, the STATELESS fast-loop
// exhaustion check (the exhaustion twin of BusyOf). The driver's ~2s poll calls
// it to fast-fail a walled CLI immediately, instead of waiting for the next 300s
// stop-review checkpoint's Observe (the production gap the checkpoint-only path
// left). No session key, no state mutation, nil-safe.

func TestSignalCenter_ExhaustedOf(t *testing.T) {
	walled := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota (exceeded|reached)`}

	// nil-safe (mirrors BusyOf): a caller holding an optional center needs no guard.
	var nilSC *SignalCenter
	if !nilSC.ExhaustedOf("⚠ Individual quota reached. Resets 52h\n", walled) {
		t.Error("nil-safe ExhaustedOf must detect a matching wall")
	}
	if nilSC.ExhaustedOf("working normally\n", walled) {
		t.Error("no wall in pane → false")
	}

	sc := NewSignalCenter()
	if !sc.ExhaustedOf("⚠ Individual quota reached\n", walled) {
		t.Error("matching wall → true")
	}
	// Empty pattern → never walls (fail-open, e.g. codex has no exhausted_regex).
	if sc.ExhaustedOf("⚠ Individual quota reached\n", PaneProfile{Name: "codex"}) {
		t.Error("empty ExhaustedRegex must never wall")
	}
	// Invalid pattern → fail-open (never panics, never walls).
	if sc.ExhaustedOf("([unclosed appears here", PaneProfile{ExhaustedRegex: "([unclosed"}) {
		t.Error("invalid pattern must fail open")
	}
	// Stateless: it must NOT create a session (Aggregate stays empty, like BusyOf).
	if sc.Aggregate() != 0 {
		t.Errorf("ExhaustedOf created a session (Aggregate=%v) — must be stateless like BusyOf", sc.Aggregate())
	}
}
