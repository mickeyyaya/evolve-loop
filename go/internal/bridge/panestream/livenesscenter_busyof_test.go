package panestream

import "testing"

func TestSignalCenter_BusyOf_MatchesStandalonePaneBusy(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestSignalCenter_BusyOf_EmptyPaneUnknownProfileNoPanic(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestSignalCenter_BusyOf_StatelessNoSessionMutation(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestSignalCenter_BusyOf_NilReceiverSafe(t *testing.T) {
	var sc *LivenessCenter
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
