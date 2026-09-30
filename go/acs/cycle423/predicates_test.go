//go:build acs

package cycle423

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func readTestdataFrame(t *testing.T, root, relPath string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "go", "internal", "bridge", "panestream", "testdata", relPath))
	if err != nil {
		t.Fatalf("readTestdataFrame(%q): %v", relPath, err)
	}
	return string(b)
}

func TestC423_001_LivenessStateEnumExists(t *testing.T) {
	states := []panestream.LivenessState{
		panestream.LivenessConverging,
		panestream.LivenessBusyButStagnant,
		panestream.LivenessIdle,
		panestream.LivenessHung,
	}
	if len(states) != 4 {
		t.Fatal("unreachable: enum compile-check guard")
	}
}

func TestC423_002_DefaultDetectorConvergingAllCLIs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cases := []struct {
		cli         string
		thinkFrame  string
		answerFrame string
	}{
		{"claude", "claude/thinking.txt", "claude/answer.txt"},
		{"codex", "codex/thinking.txt", "codex/answer.txt"},
		{"agy", "agy/thinking.txt", "agy/answer.txt"},
		{"ollama", "ollama/thinking.txt", "ollama/answer.txt"},
	}
	for _, c := range cases {
		t.Run(c.cli, func(t *testing.T) {
			profile := panestream.Profiles[c.cli]
			det := panestream.NewDefaultDetector(3)
			think := readTestdataFrame(t, root, c.thinkFrame)
			answer := readTestdataFrame(t, root, c.answerFrame)
			det.Assess(think, profile)
			state, _ := det.Assess(answer, profile)
			if state != panestream.LivenessConverging {
				t.Errorf("[%s] thinking→answer: got state %v, want LivenessConverging", c.cli, state)
			}
		})
	}
}

func TestC423_003_BusyNoContentIsBusyButStagnant(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewDefaultDetector(3)
	det.Assess("⏺ agent output here\n❯ \n", profile)
	spinnerFrame := "⏺ agent output here\n✽ Inferring… (5s · ↓ 10 tokens)\n❯ \n"
	state, _ := det.Assess(spinnerFrame, profile)
	if state == panestream.LivenessConverging {
		t.Errorf("busy-but-no-content frame classified Converging (got %v); spinner-only delta must NOT equal Converging", state)
	}
	if state != panestream.LivenessBusyButStagnant {
		t.Errorf("busy-but-no-content frame: got %v, want LivenessBusyButStagnant", state)
	}
}

func TestC423_004_QuietFrameIsIdleNotHung(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewDefaultDetector(3)

	det.Assess("⏺ some output\n❯ \n", profile)
	state, _ := det.Assess("⏺ some output\n❯ \n", profile)
	if state == panestream.LivenessHung {
		t.Errorf("single quiet interval must NOT be Hung (got %v); Hung requires %d stall intervals", state, 3)
	}
	if state != panestream.LivenessIdle {
		t.Errorf("single quiet interval: got %v, want LivenessIdle", state)
	}

	det2 := panestream.NewDefaultDetector(3)
	det2.Assess("❯ \n", profile)
	state2, _ := det2.Assess("❯ \n", profile)
	if state2 == panestream.LivenessHung {
		t.Errorf("empty-frame single interval must NOT be Hung (got %v)", state2)
	}
}

func TestC423_005_HungAfterStallThreshold(t *testing.T) {
	profile := panestream.Profiles["claude"]
	const stall = 2
	det := panestream.NewDefaultDetector(stall)
	det.Assess("⏺ initial content\n❯ \n", profile)
	spinnerFrame := "⏺ initial content\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n"
	s1, _ := det.Assess(spinnerFrame, profile)
	if s1 == panestream.LivenessHung {
		t.Fatalf("interval 1/%d: must NOT be Hung yet (got %v)", stall, s1)
	}
	spinnerFrame2 := "⏺ initial content\n✽ Thinking… (10s · ↓ 120 tokens)\n❯ \n"
	s2, _ := det.Assess(spinnerFrame2, profile)
	if s2 != panestream.LivenessHung {
		t.Errorf("interval 2/%d: got %v, want LivenessHung (stall threshold reached)", stall, s2)
	}
}

func TestC423_006_DetectorForRegistryAllCLIs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	validStates := map[panestream.LivenessState]bool{
		panestream.LivenessConverging:      true,
		panestream.LivenessBusyButStagnant: true,
		panestream.LivenessIdle:            true,
		panestream.LivenessHung:            true,
	}
	for cli, profile := range panestream.Profiles {
		t.Run(cli, func(t *testing.T) {
			probe := panestream.DetectorFor(profile)
			if probe == nil {
				t.Fatalf("DetectorFor(%q) = nil, want non-nil LivenessProbe", cli)
			}
			think := readTestdataFrame(t, root, cli+"/thinking.txt")
			answer := readTestdataFrame(t, root, cli+"/answer.txt")
			probe.Assess(think, profile)
			state, conf := probe.Assess(answer, profile)
			if !validStates[state] {
				t.Errorf("[%s] DetectorFor returned invalid state %v", cli, state)
			}
			if conf < 0 || conf > 1.0 {
				t.Errorf("[%s] confidence %v out of [0,1] range", cli, conf)
			}
		})
	}
}

func TestC423_007_NoCliNameInStopReview(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		acsassert.FileNotContains(t, path, `"`+cli+`"`)
	}
}

func TestC423_008_StopEventHasStateField(t *testing.T) {
	ev := bridge.StopEvent{}
	ev.State = panestream.LivenessConverging
	if ev.State != panestream.LivenessConverging {
		t.Fatal("unreachable: StopEvent.State field compile check")
	}
}

func TestC423_009_ConvergingExtendsUnconditionally(t *testing.T) {
	r := bridge.NewDeterministicReviewer(2)
	cases := []int{0, 1, 2, 3, 9}
	for _, attempt := range cases {
		ev := bridge.StopEvent{State: panestream.LivenessConverging, Attempt: attempt}
		verdict := r.Review(ev)
		if verdict.Action != bridge.ReviewExtend {
			t.Errorf("Converging at attempt=%d (maxExtends=2): got %q, want ReviewExtend — a converging agent must never be capped", attempt, verdict.Action)
		}
	}
}

func TestC423_010_HungFastFailsUnderMaxExtends(t *testing.T) {
	r := bridge.NewDeterministicReviewer(6)
	ev := bridge.StopEvent{State: panestream.LivenessHung, Attempt: 1}
	verdict := r.Review(ev)
	if verdict.Action == bridge.ReviewExtend {
		t.Errorf("Hung at attempt=1 (under maxExtends=6): got ReviewExtend, want non-extend (fast-fail path); Hung must not wait for the ~30-min backstop")
	}
}

func TestC423_011_IncreasingTokensHigherConfConverging(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewClaudeDetector(3)
	base := panestream.NewDefaultDetector(3)

	frames := []string{
		"✽ Generating… (5s · ↓ 50 tokens)\n❯ \n",
		"✽ Generating… (10s · ↓ 150 tokens)\n❯ \n",
		"✽ Generating… (15s · ↓ 300 tokens)\n❯ \n",
	}
	for i, f := range frames[:2] {
		base.Assess(f, profile)
		det.Assess(f, profile)
		_ = i
	}
	baseState, baseConf := base.Assess(frames[2], profile)
	claudeState, claudeConf := det.Assess(frames[2], profile)

	if claudeState != panestream.LivenessConverging {
		t.Errorf("increasing tokens: claude detector got %v, want LivenessConverging (token growth proves live work)", claudeState)
	}
	if claudeConf <= baseConf {
		t.Errorf("increasing tokens: claude confidence %v not > default %v (base state=%v)", claudeConf, baseConf, baseState)
	}
}

func TestC423_012_StaticTokenFallsBackToDefault(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewClaudeDetector(3)
	base := panestream.NewDefaultDetector(3)

	staticFrame := "✽ Generating… (5s · ↓ 100 tokens)\n❯ \n"
	for range 3 {
		base.Assess(staticFrame, profile)
		det.Assess(staticFrame, profile)
	}
	baseState, baseConf := base.Assess(staticFrame, profile)
	claudeState, claudeConf := det.Assess(staticFrame, profile)

	if claudeState != baseState {
		t.Errorf("static token: claude state %v differs from default %v (static counter must not elevate)", claudeState, baseState)
	}
	if claudeConf > baseConf {
		t.Errorf("static token: claude confidence %v > default %v (static counter must not elevate confidence)", claudeConf, baseConf)
	}
}

func TestC423_013_MalformedTokenNoPanic(t *testing.T) {
	profile := panestream.Profiles["claude"]
	malformedFrames := []string{
		"✽ Generating… (↓ tokens)\n❯ \n",
		"✽ Generating… (5s · ↓ k tokens)\n❯ \n",
		"✽ Generating…\n❯ \n",
		"✽ ↓ not-a-number tokens\n❯ \n",
	}
	for _, frame := range malformedFrames {
		frame := frame
		t.Run("malformed", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("malformed token frame panicked: %v (frame=%q)", r, frame)
				}
			}()
			base := panestream.NewDefaultDetector(3)
			det := panestream.NewClaudeDetector(3)
			base.Assess("⏺ content\n❯ \n", profile)
			det.Assess("⏺ content\n❯ \n", profile)
			_, baseConf := base.Assess(frame, profile)
			_, claudeConf := det.Assess(frame, profile)
			if claudeConf > baseConf {
				t.Errorf("malformed frame elevated confidence: claude %v > default %v (frame=%q)", claudeConf, baseConf, frame)
			}
		})
	}
}

func TestC423_014_NonClaudeDetectorUnaffectedByClaudeLayer(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, cli := range []string{"codex", "agy", "ollama"} {
		t.Run(cli, func(t *testing.T) {
			profile := panestream.Profiles[cli]
			det := panestream.DetectorFor(profile)
			base := panestream.NewDefaultDetector(3)
			think := readTestdataFrame(t, root, cli+"/thinking.txt")
			answer := readTestdataFrame(t, root, cli+"/answer.txt")
			base.Assess(think, profile)
			det.Assess(think, profile)
			baseState, baseConf := base.Assess(answer, profile)
			detState, detConf := det.Assess(answer, profile)
			if detState != baseState {
				t.Errorf("[%s] DetectorFor state %v ≠ DefaultDetector %v (claude layer must not contaminate non-claude CLIs)", cli, detState, baseState)
			}
			if detConf != baseConf {
				t.Errorf("[%s] DetectorFor confidence %v ≠ DefaultDetector %v (claude layer must not elevate non-claude confidence)", cli, detConf, baseConf)
			}
		})
	}
}
