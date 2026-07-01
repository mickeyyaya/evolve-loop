package panestream

import "testing"

// liveness_exhaustion_test.go — the ExhaustionProbe Decorator (S1). A CLI that
// hits a quota/rate-limit wall MID-EXECUTION prints its error and returns to the
// REPL prompt without exiting; the re-printed error reads as new content, so the
// inner liveness detector calls it LivenessConverging ("real output is never
// stuck") and the reviewer extends forever — the livelock that hung agy 15+ min.
// The decorator makes the wall an ORTHOGONAL, dominating signal.

// Compile-time conformance: the decorator IS a LivenessProbe (composes over any).
var _ LivenessProbe = (*ExhaustionProbe)(nil)

// A pane matching the profile's ExhaustedRegex is LivenessExhausted, OVERRIDING
// whatever the inner probe would report — even a frame the inner detector would
// call Converging (new content), because that "content" is the quota error.
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

// No match → the decorator is transparent: it delegates to the inner probe
// byte-identically (same state, same confidence), so healthy sessions are
// unaffected.
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

// An empty ExhaustedRegex never walls (fail-open — the detector must never
// invent a wall for a CLI whose manifest defines no pattern, e.g. codex).
func TestExhaustionProbe_EmptyPatternNeverWalls(t *testing.T) {
	p := PaneProfile{Name: "codex", ExhaustedRegex: ""}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	if got, _ := probe.Assess("⚠ Individual quota reached\n", p); got == LivenessExhausted {
		t.Errorf("empty pattern must never wall; got %v", got)
	}
}

// An invalid (uncompilable) ExhaustedRegex fails open — never walls, never
// panics: the gate's own misconfiguration must not brick a session.
func TestExhaustionProbe_InvalidPatternFailsOpen(t *testing.T) {
	p := PaneProfile{Name: "x", ExhaustedRegex: "([unclosed"}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	if got, _ := probe.Assess("([unclosed literal appears in the pane\n", p); got == LivenessExhausted {
		t.Errorf("invalid pattern must fail open; got %v", got)
	}
}

// The decorator caches its compiled regex across calls with a stable pattern
// (the profile is constant per session) — a subsequent match still walls.
func TestExhaustionProbe_MatchesAcrossCalls(t *testing.T) {
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	probe := NewExhaustionProbe(NewDefaultDetector(3))
	probe.Assess("working line 1\n", p)
	probe.Assess("working line 2\n", p)
	if got, _ := probe.Assess("⚠ quota reached now\n", p); got != LivenessExhausted {
		t.Fatalf("got %v, want LivenessExhausted on a later matching frame", got)
	}
}
