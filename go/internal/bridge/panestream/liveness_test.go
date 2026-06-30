package panestream

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// liveness_test.go — Unit tests for DefaultDetector, ClaudeDetector, and DetectorFor.
// These supplement the ACS predicates in go/acs/cycle423/ with lower-level
// behavioral assertions over individual pane frame sequences.

func testdataFrame(t *testing.T, relPath string) string {
	t.Helper()
	// Locate testdata relative to this file.
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "testdata", relPath))
	if err != nil {
		t.Fatalf("testdataFrame(%q): %v", relPath, err)
	}
	return string(b)
}

// TestDefaultDetector_Converging asserts the four CLIs all classify Converging
// when new stable content appears (thinking → answer frame sequence).
func TestDefaultDetector_Converging(t *testing.T) {
	cases := []struct{ cli, think, answer string }{
		{"claude", "claude/thinking.txt", "claude/answer.txt"},
		{"codex", "codex/thinking.txt", "codex/answer.txt"},
		{"agy", "agy/thinking.txt", "agy/answer.txt"},
		{"ollama", "ollama/thinking.txt", "ollama/answer.txt"},
	}
	for _, c := range cases {
		t.Run(c.cli, func(t *testing.T) {
			det := NewDefaultDetector(3)
			p := Profiles[c.cli]
			det.Assess(testdataFrame(t, c.think), p) // prime
			state, _ := det.Assess(testdataFrame(t, c.answer), p)
			if state != LivenessConverging {
				t.Errorf("[%s] thinking→answer: got %v, want LivenessConverging", c.cli, state)
			}
		})
	}
}

// TestDefaultDetector_SpinnerOnlyIsNotConverging is the load-bearing negative
// test: a frame pair whose ONLY delta is spinner/chrome (no new content line)
// must NOT classify as Converging. This prevents gaming by treating any
// pane-diff as "progress" (the historic ticking-clock hole).
func TestDefaultDetector_SpinnerOnlyIsNotConverging(t *testing.T) {
	p := Profiles["claude"]
	det := NewDefaultDetector(3)
	// Prime on content frame.
	det.Assess("⏺ real content\n❯ \n", p)
	// Spinner-only frame: only the chrome/affordance changes, content is identical.
	spinner := "⏺ real content\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n"
	state, _ := det.Assess(spinner, p)
	if state == LivenessConverging {
		t.Errorf("spinner-only delta must NOT be Converging (got %v); growth velocity must count CONTENT lines", state)
	}
}

// TestDefaultDetector_PrimingNotHung asserts the first Assess call on an empty
// or minimal pane never returns LivenessHung.
func TestDefaultDetector_PrimingNotHung(t *testing.T) {
	p := Profiles["claude"]
	for _, frame := range []string{
		"❯ \n",
		"⏺ some content\n❯ \n",
	} {
		det := NewDefaultDetector(1) // stallThreshold=1 — soonest possible Hung
		det.Assess(frame, p)         // prime
		state, _ := det.Assess(frame, p)
		if state == LivenessHung {
			t.Errorf("single stall after prime must NOT be Hung (stallThreshold=1); got %v for %q", state, frame)
		}
	}
}

// TestDefaultDetector_HungAfterThreshold asserts Hung is emitted after exactly
// stallThreshold consecutive busy-stagnant intervals.
func TestDefaultDetector_HungAfterThreshold(t *testing.T) {
	p := Profiles["claude"]
	const stall = 2
	det := NewDefaultDetector(stall)
	det.Assess("⏺ initial\n❯ \n", p) // prime
	// Interval 1 — stalls=1 < 2, expect BusyButStagnant.
	s1, _ := det.Assess("⏺ initial\n✽ Thinking… (5s · ↓ 10 tokens)\n❯ \n", p)
	if s1 == LivenessHung {
		t.Fatalf("interval 1/%d: must NOT be Hung yet (got %v)", stall, s1)
	}
	// Interval 2 — stalls=2 ≥ 2, expect Hung.
	s2, _ := det.Assess("⏺ initial\n✽ Thinking… (8s · ↓ 25 tokens)\n❯ \n", p)
	if s2 != LivenessHung {
		t.Errorf("interval 2/%d: got %v, want LivenessHung", stall, s2)
	}
}

// TestDefaultDetector_IdleResetsStalls asserts that a non-busy quiet frame
// resets the stall counter so subsequent busy-stagnant intervals restart from 0
// rather than accumulating across an Idle gap.
func TestDefaultDetector_IdleResetsStalls(t *testing.T) {
	p := Profiles["claude"]
	det := NewDefaultDetector(2)
	det.Assess("⏺ content\n❯ \n", p) // prime
	// One busy-stagnant interval (stalls=1).
	det.Assess("⏺ content\n✽ Thinking… (3s · ↓ 5 tokens)\n❯ \n", p)
	// An idle frame (not busy) — resets stalls to 0.
	det.Assess("⏺ content\n❯ \n", p)
	// Another busy-stagnant interval (stalls=1 again, not 2).
	s, _ := det.Assess("⏺ content\n✽ Thinking… (3s · ↓ 5 tokens)\n❯ \n", p)
	if s == LivenessHung {
		t.Errorf("after idle reset, one more busy-stagnant interval must NOT be Hung (stallThreshold=2), got %v", s)
	}
}

// TestClaudeDetector_IncreasingTokensConverging asserts that a strictly-
// increasing ↓ token counter over multiple intervals yields Converging with
// higher confidence than DefaultDetector on the same frames (pure chrome frames
// where DefaultDetector would classify BusyButStagnant or Hung).
func TestClaudeDetector_IncreasingTokensConverging(t *testing.T) {
	p := Profiles["claude"]
	base := NewDefaultDetector(3)
	det := NewClaudeDetector(3)
	// Three frames with strictly increasing ↓ token counters, no content lines.
	frames := []string{
		"✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n",
		"✽ Thinking… (10s · ↓ 150 tokens)\n❯ \n",
		"✽ Thinking… (15s · ↓ 300 tokens)\n❯ \n",
	}
	// Feed frames[0:2] to both (prime + one more).
	for _, f := range frames[:2] {
		base.Assess(f, p)
		det.Assess(f, p)
	}
	_, baseConf := base.Assess(frames[2], p)
	claudeState, claudeConf := det.Assess(frames[2], p)
	if claudeState != LivenessConverging {
		t.Errorf("increasing tokens: want LivenessConverging, got %v", claudeState)
	}
	if claudeConf <= baseConf {
		t.Errorf("increasing tokens: claude conf %v must be > default conf %v", claudeConf, baseConf)
	}
}

// TestClaudeDetector_StaticTokenFallsBack asserts that a static (non-increasing)
// ↓ token counter produces the same state+confidence as DefaultDetector.
func TestClaudeDetector_StaticTokenFallsBack(t *testing.T) {
	p := Profiles["claude"]
	base := NewDefaultDetector(3)
	det := NewClaudeDetector(3)
	static := "✽ Thinking… (5s · ↓ 100 tokens)\n❯ \n"
	for range 3 {
		base.Assess(static, p)
		det.Assess(static, p)
	}
	baseState, baseConf := base.Assess(static, p)
	detState, detConf := det.Assess(static, p)
	if detState != baseState {
		t.Errorf("static token: claude state %v ≠ default %v", detState, baseState)
	}
	if detConf > baseConf {
		t.Errorf("static token: claude conf %v must not exceed default %v", detConf, baseConf)
	}
}

// TestClaudeDetector_MalformedTokenNoPanic asserts malformed token lines cause
// no panic and no confidence elevation above the default.
func TestClaudeDetector_MalformedTokenNoPanic(t *testing.T) {
	p := Profiles["claude"]
	malformed := []string{
		"✽ Thinking… (↓ tokens)\n❯ \n",
		"✽ Thinking… (5s · ↓ k tokens)\n❯ \n",
		"✽ Thinking…\n❯ \n",
		"✽ ↓ not-a-number tokens\n❯ \n",
	}
	for _, frame := range malformed {
		t.Run("malformed", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on malformed frame %q: %v", frame, r)
				}
			}()
			base := NewDefaultDetector(3)
			det := NewClaudeDetector(3)
			base.Assess("⏺ content\n❯ \n", p)
			det.Assess("⏺ content\n❯ \n", p)
			_, baseConf := base.Assess(frame, p)
			_, detConf := det.Assess(frame, p)
			if detConf > baseConf {
				t.Errorf("malformed frame %q: claude conf %v > default %v (must not elevate)", frame, detConf, baseConf)
			}
		})
	}
}

// TestDetectorFor_NonClaudeIsDefault asserts DetectorFor returns a detector
// byte-identical to DefaultDetector for non-claude CLIs.
func TestDetectorFor_NonClaudeIsDefault(t *testing.T) {
	for _, cli := range []string{"codex", "agy", "ollama"} {
		t.Run(cli, func(t *testing.T) {
			p := Profiles[cli]
			probe := DetectorFor(p)
			if probe == nil {
				t.Fatalf("DetectorFor(%q) = nil", cli)
			}
			base := NewDefaultDetector(0)
			think := testdataFrame(t, cli+"/thinking.txt")
			answer := testdataFrame(t, cli+"/answer.txt")
			probe.Assess(think, p)
			base.Assess(think, p)
			detState, detConf := probe.Assess(answer, p)
			baseState, baseConf := base.Assess(answer, p)
			if detState != baseState || detConf != baseConf {
				t.Errorf("[%s] DetectorFor: (%v,%.2f) ≠ DefaultDetector: (%v,%.2f)", cli, detState, detConf, baseState, baseConf)
			}
		})
	}
}

// TestDetectorFor_ClaudeIsClaudeDetector asserts DetectorFor returns a detector
// that activates the token-counter layer for claude (higher conf on increasing ↓ tokens).
func TestDetectorFor_ClaudeIsClaudeDetector(t *testing.T) {
	p := Profiles["claude"]
	probe := DetectorFor(p)
	base := NewDefaultDetector(0)
	frames := []string{
		"✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n",
		"✽ Thinking… (10s · ↓ 200 tokens)\n❯ \n",
	}
	probe.Assess(frames[0], p)
	base.Assess(frames[0], p)
	_, baseConf := base.Assess(frames[1], p)
	_, detConf := probe.Assess(frames[1], p)
	if detConf <= baseConf {
		t.Errorf("claude DetectorFor: conf %v must be > default %v on increasing tokens", detConf, baseConf)
	}
}
