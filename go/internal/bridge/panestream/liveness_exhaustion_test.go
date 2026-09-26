package panestream

import "testing"

var _ LivenessProbe = (*ExhaustionProbe)(nil)

func TestExhaustionProbe_OverridesLiveness(t *testing.T) {
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota (exceeded|reached)`}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	probe.Assess("some earlier output\n", p) // prime the inner delta
	wall := "some earlier output\n⚠ Individual quota reached. Resets in 52h\n"
	got, conf := probe.Assess(wall, p)
	if got != LivenessExhausted {
		t.Fatalf("got %v, want LivenessExhausted (a quota wall must override liveness)", got)
	}
	if conf < 0 || conf > 1 {
		t.Errorf("confidence %v out of [0,1]", conf)
	}
}

func TestExhaustionProbe_DelegatesWhenNoMatch(t *testing.T) {
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	inner := NewDefaultDetector(3)
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	f1, f2 := "hello\n", "hello\nworld\n"
	inner.Assess(f1, p)
	probe.Assess(f1, p) // prime both identically
	wantState, wantConf := inner.Assess(f2, p)
	gotState, gotConf := probe.Assess(f2, p)
	if gotState != wantState || gotConf != wantConf {
		t.Errorf("no-match delegate: got (%v,%v), want (%v,%v)", gotState, gotConf, wantState, wantConf)
	}
}

func TestExhaustionProbe_EmptyPatternNeverWalls(t *testing.T) {
	p := PaneProfile{Name: "codex", ExhaustedRegex: ""}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	if got, _ := probe.Assess("⚠ Individual quota reached\n", p); got == LivenessExhausted {
		t.Errorf("empty pattern must never wall; got %v", got)
	}
}

func TestExhaustionProbe_InvalidPatternFailsOpen(t *testing.T) {
	p := PaneProfile{Name: "x", ExhaustedRegex: "([unclosed"}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	if got, _ := probe.Assess("([unclosed literal appears in the pane\n", p); got == LivenessExhausted {
		t.Errorf("invalid pattern must fail open; got %v", got)
	}
}

func TestExhaustionProbe_MatchesAcrossCalls(t *testing.T) {
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	probe.Assess("working line 1\n", p)
	probe.Assess("working line 2\n", p)
	if got, _ := probe.Assess("⚠ quota reached now\n", p); got != LivenessExhausted {
		t.Fatalf("got %v, want LivenessExhausted on a later matching frame", got)
	}
}
