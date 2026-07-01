package panestream

// signalcenter_busyof_test.go — RED tests for cycle-434 slice S4 completion
// (s4-complete-residual-busy-callsites, Task 1): a STATELESS busy projection
// on SignalCenter — BusyOf(rendered, profile) bool — that the two surviving
// direct panestream.PaneBusy consumers (autorespond.go's tick busy-gate,
// driver_tmux_repl.go's idle_reached busy/idle bracket) route through instead
// of calling PaneBusy directly. Unlike Busy(sessionKey) (S4/cycle-432), BusyOf
// takes NO session key and touches NO per-session state — Observe is never
// called, so routing these two call sites through it cannot pollute the
// checkpoint's Observe/Aggregate baseline (F3, scout finding — these sites
// fire at different loop points than the checkpoint's Observe).
//
// TDD contract: written BEFORE BusyOf exists. Compile-fails (BusyOf
// undefined) until Builder implements it. DO NOT MODIFY THESE TESTS —
// Builder implements to make them GREEN.

import "testing"

// TestSignalCenter_BusyOf_MatchesStandalonePaneBusy (AC1/AC2 preservation,
// positive): BusyOf must delegate to the SAME single definition PaneBusy
// (and the Busy(sessionKey) projection) already uses — never a
// reimplementation that can drift. Verified for both a busy (live-turn
// affordance) and an idle pane.
func TestSignalCenter_BusyOf_MatchesStandalonePaneBusy(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]
	busyPane := "Which absolute path should I write the deliverable to?\n⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"
	idlePane := "Which absolute path should I write the deliverable to?\n⏺ answer complete\n"

	if got, want := sc.BusyOf(busyPane, p), PaneBusy(busyPane, p); got != want {
		t.Errorf("BusyOf(busyPane) = %v, want %v (must match standalone PaneBusy)", got, want)
	}
	if got, want := sc.BusyOf(idlePane, p), PaneBusy(idlePane, p); got != want {
		t.Errorf("BusyOf(idlePane) = %v, want %v (must match standalone PaneBusy)", got, want)
	}
	if !sc.BusyOf(busyPane, p) {
		t.Fatal("fixture invalid: busyPane must read busy")
	}
	if sc.BusyOf(idlePane, p) {
		t.Fatal("fixture invalid: idlePane must read idle")
	}
}

// TestSignalCenter_BusyOf_EmptyPaneUnknownProfileNoPanic (AC4, edge/OOD): an
// empty rendered pane and/or an unknown profile (the zero-value PaneProfile a
// map miss on panestream.Profiles yields — exactly what autorespond.go's
// call site produces for an unrecognized ar.cli) must read not-busy and must
// never panic.
func TestSignalCenter_BusyOf_EmptyPaneUnknownProfileNoPanic(t *testing.T) {
	sc := NewSignalCenter()
	unknown := Profiles["does-not-exist"] // zero-value PaneProfile (map miss)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BusyOf panicked on empty pane / unknown profile: %v", r)
		}
	}()
	if sc.BusyOf("", Profiles["claude"]) {
		t.Error("BusyOf(\"\", claude) = true, want false (empty pane has no affordance)")
	}
	if sc.BusyOf("some pane content", unknown) {
		t.Error("BusyOf(content, unknown-profile) = true, want false (zero-value profile)")
	}
	if sc.BusyOf("", unknown) {
		t.Error("BusyOf(\"\", unknown-profile) = true, want false")
	}
}

// TestSignalCenter_BusyOf_StatelessNoSessionMutation (AC4/design-constraint
// F3, edge — discriminating): BusyOf must NOT go through Observe — calling it
// (repeatedly, with any session-shaped content) must never create a session
// entry, so Aggregate() stays at the empty-center zero value and a
// subsequent Busy(sessionKey)/Changed(sessionKey) for any key the caller
// might reuse still reads the unobserved default (false). This is what
// keeps the checkpoint's Observe/Aggregate baseline byte-identical when the
// two residual call sites are migrated (they fire at different loop points
// than the checkpoint's own Observe).
func TestSignalCenter_BusyOf_StatelessNoSessionMutation(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]
	busyPane := "⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"

	for i := 0; i < 3; i++ {
		sc.BusyOf(busyPane, p)
	}

	if got := sc.Aggregate(); got != 0 {
		t.Errorf("Aggregate() = %v after BusyOf-only calls, want 0 (empty center — BusyOf must not Observe)", got)
	}
	if sc.Busy("s") {
		t.Error("Busy(\"s\") = true after BusyOf-only calls, want false (no session was ever Observed)")
	}
	if sc.Changed("s") {
		t.Error("Changed(\"s\") = true after BusyOf-only calls, want false (no session was ever Observed)")
	}
}

// TestSignalCenter_BusyOf_NilReceiverSafe (AC4, edge): BusyOf is stateless —
// it must be safe to call on a nil *SignalCenter, so a caller holding an
// optional (possibly-nil) center reference (e.g. autorespond.go's
// ar.deps.LivenessCenter, which is nil outside the driver's Deps-injected
// test seam) never needs a nil guard before delegating.
func TestSignalCenter_BusyOf_NilReceiverSafe(t *testing.T) {
	var sc *SignalCenter
	p := Profiles["claude"]
	busyPane := "⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"
	idlePane := "⏺ answer complete\n"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BusyOf panicked on nil receiver: %v", r)
		}
	}()
	if got, want := sc.BusyOf(busyPane, p), PaneBusy(busyPane, p); got != want {
		t.Errorf("nil-receiver BusyOf(busyPane) = %v, want %v", got, want)
	}
	if got, want := sc.BusyOf(idlePane, p), PaneBusy(idlePane, p); got != want {
		t.Errorf("nil-receiver BusyOf(idlePane) = %v, want %v", got, want)
	}
}
