package panestream

import "testing"

func TestSignalCenter_ExhaustedOf(t *testing.T) {
	walled := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota (exceeded|reached)`}

	var nilSC *LivenessCenter
	if !nilSC.ExhaustedOf("⚠ Individual quota reached. Resets 52h\n", walled) {
		t.Error("nil-safe ExhaustedOf must detect a matching wall")
	}
	if nilSC.ExhaustedOf("working normally\n", walled) {
		t.Error("no wall in pane → false")
	}

	sc := NewLivenessCenter()
	if !sc.ExhaustedOf("⚠ Individual quota reached\n", walled) {
		t.Error("matching wall → true")
	}
	if sc.ExhaustedOf("⚠ Individual quota reached\n", PaneProfile{Name: "codex"}) {
		t.Error("empty ExhaustedRegex must never wall")
	}
	if sc.ExhaustedOf("([unclosed appears here", PaneProfile{ExhaustedRegex: "([unclosed"}) {
		t.Error("invalid pattern must fail open")
	}
	if sc.Aggregate() != 0 {
		t.Errorf("ExhaustedOf created a session (Aggregate=%v) — must be stateless like BusyOf", sc.Aggregate())
	}
}
