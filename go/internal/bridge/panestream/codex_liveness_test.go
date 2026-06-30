package panestream

// codex_liveness_test.go — regression tests for codex's weak-signal liveness
// closure (T3, pre-GREEN: DefaultDetector already classifies correctly).
//
// codex has no busy affordance (PaneBusy always returns false on codex frames),
// so a stalled codex pane resolves to LivenessIdle (not LivenessHung). LivenessHung
// requires busy=true accumulating stalls — an invariant that codex can never satisfy.
// These tests document this as INTENDED behavior and pin it against regression.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func codexFrame(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "testdata", "codex", name))
	if err != nil {
		t.Fatalf("codexFrame(%q): %v", name, err)
	}
	return string(b)
}

// TestCodexLiveness_ThinkingToAnswerConverging verifies that a codex session
// transitioning from the thinking frame to an answer frame (new stable content)
// yields LivenessConverging. This is the positive/happy-path case: the DefaultDetector
// growth-velocity path fires when content lines appear between frames.
func TestCodexLiveness_ThinkingToAnswerConverging(t *testing.T) {
	p := Profiles["codex"]
	det := NewDefaultDetector(3)
	think := codexFrame(t, "thinking.txt")
	answer := codexFrame(t, "answer.txt")
	det.Assess(think, p) // prime
	state, conf := det.Assess(answer, p)
	if state != LivenessConverging {
		t.Errorf("thinking→answer: got %v, want LivenessConverging", state)
	}
	if conf < 0 || conf > 1 {
		t.Errorf("thinking→answer: confidence %v out of [0,1]", conf)
	}
}

// TestCodexLiveness_PrimingNotHung asserts the first Assess call (prime) on a
// codex frame never returns LivenessHung, regardless of stallThreshold. Codex
// panes cannot reach Hung because PaneBusy is always false, but the prime guard
// would also prevent it even if busy were somehow true.
func TestCodexLiveness_PrimingNotHung(t *testing.T) {
	p := Profiles["codex"]
	answer := codexFrame(t, "answer.txt")
	// Use stallThreshold=1 so Hung fires as early as possible if the invariant breaks.
	det := NewDefaultDetector(1)
	state, conf := det.Assess(answer, p)
	if state == LivenessHung {
		t.Errorf("prime call must NOT return LivenessHung (got %v)", state)
	}
	if conf < 0 || conf > 1 {
		t.Errorf("prime confidence %v out of [0,1]", conf)
	}
}

// TestCodexLiveness_StalledIdleNotHung is the load-bearing negative test:
// repeated identical codex answer frames (stalled, no content growth, no busy
// affordance) must yield LivenessIdle across ≥3 intervals and NEVER LivenessHung.
// Hung requires busy=true accumulating stalls, but PaneBusy always returns false
// for codex frames — so Hung is structurally unreachable, documented here as
// the intended weak-signal closure.
func TestCodexLiveness_StalledIdleNotHung(t *testing.T) {
	p := Profiles["codex"]
	// Verify codex frames are never-busy (prerequisite of the invariant).
	answer := codexFrame(t, "answer.txt")
	if PaneBusy(answer, p) {
		t.Fatal("precondition: codex/answer.txt must not be busy (no busy affordance)")
	}

	det := NewDefaultDetector(3) // hangAfter=3: earliest possible Hung
	det.Assess(answer, p)        // prime

	// ≥3 identical stall intervals — each resets stalls to 0 because !busy.
	for i := 1; i <= 5; i++ {
		state, conf := det.Assess(answer, p)
		if state == LivenessHung {
			t.Errorf("stall interval %d: got LivenessHung — codex cannot reach Hung (no busy affordance)", i)
		}
		if state != LivenessIdle {
			t.Errorf("stall interval %d: got %v, want LivenessIdle (no new content, no busy signal)", i, state)
		}
		if conf < 0 || conf > 1 {
			t.Errorf("stall interval %d: confidence %v out of [0,1]", i, conf)
		}
	}
}

// TestCodexLiveness_ConfidenceInRange asserts confidence is always in [0,1]
// for all three codex testdata frames (thinking, answer, final) across both
// the prime and subsequent assess calls.
func TestCodexLiveness_ConfidenceInRange(t *testing.T) {
	p := Profiles["codex"]
	frames := []string{"thinking.txt", "answer.txt", "final.txt"}
	for _, f := range frames {
		t.Run(f, func(t *testing.T) {
			det := NewDefaultDetector(3)
			content := codexFrame(t, f)
			_, conf1 := det.Assess(content, p)
			if conf1 < 0 || conf1 > 1 {
				t.Errorf("prime confidence %v out of [0,1]", conf1)
			}
			_, conf2 := det.Assess(content, p)
			if conf2 < 0 || conf2 > 1 {
				t.Errorf("second call confidence %v out of [0,1]", conf2)
			}
		})
	}
}
