//go:build acs

// Package cycle423 materializes the cycle-423 acceptance criteria for three
// liveness-detection tasks: a DefaultDetector strategy (T1), reviewer integration
// that consumes LivenessState instead of raw booleans (T2), and a claude-specific
// token-counter layer composed over the default (T3).
//
// Goal: reduce cycle latency by making phase-agent liveness detection more
// accurate, via a clean abstract Strategy layer — driver-agnostic, zero per-CLI
// branching in the consumer (stopreview.go).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	liveness-detector-default-strategy (T1 — Medium):
//	  AC1  LivenessState enum {Converging|BusyButStagnant|Idle|Hung} exists      → C423_001 (RED)
//	  AC2  DefaultDetector → Converging for all 4 CLIs (thinking→answer)         → C423_002 (RED)
//	  AC3  DefaultDetector → BusyButStagnant for busy-but-no-content (negative)  → C423_003 (RED)
//	  AC4  DefaultDetector → Idle for quiet frame (edge: not Hung immediately)   → C423_004 (RED)
//	  AC5  DefaultDetector → Hung after stallThreshold consecutive stall rounds  → C423_005 (RED)
//	  AC6  DetectorFor registry returns LivenessProbe for all 4 CLIs             → C423_006 (RED)
//
//	liveness-detector-reviewer-integration (T2 — Medium):
//	  AC7  No CLI-name literal in stopreview.go (no-branch invariant)            → C423_007 (pre-existing GREEN)
//	  AC8  StopEvent.State field of type panestream.LivenessState exists          → C423_008 (RED)
//	  AC9  Converging → ReviewExtend past maxExtends (unconditional)             → C423_009 (RED)
//	  AC10 Hung → ReviewPause fast-fail under maxExtends                         → C423_010 (RED)
//
//	liveness-claude-tokencount-strategy (T3 — Small):
//	  AC11 Increasing ↓ token series → Converging with conf > DefaultDetector    → C423_011 (RED)
//	  AC12 Static token counter → same state as DefaultDetector (negative)       → C423_012 (RED)
//	  AC13 Malformed token line → no panic, default verdict (edge)               → C423_013 (RED)
//	  AC14 Non-claude CLIs via DetectorFor → same state as DefaultDetector       → C423_014 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C423_003 (busy-but-no-content ≠ Converging), C423_012 (static token = default)
//	Edge/OOD: C423_004 (quiet ≠ Hung on single interval), C423_013 (malformed = no panic)
//	Semantic:  14 distinct dimensions: enum / 4CLIs-Converging / BusyButStagnant /
//	           Idle / Hung / registry / no-CLI-branch / State-field /
//	           Converging-uncapped / Hung-fastfail / increasing-token /
//	           static-fallback / malformed-nopanic / non-claude-unaffected.
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C423_001–C423_006), manual=0, unverifiable=0 → total AC=6 ✓
//	T2: predicate=4 (C423_007–C423_010), manual=0, unverifiable=0 → total AC=4 ✓
//	T3: predicate=4 (C423_011–C423_014), manual=0, unverifiable=0 → total AC=4 ✓
package cycle423

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// readTestdataFrame loads a real capture-pane snapshot from the panestream testdata
// directory under the repo root. The testdata directory is at
// go/internal/bridge/panestream/testdata/<cli>/<frame>.txt.
func readTestdataFrame(t *testing.T, root, relPath string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "go", "internal", "bridge", "panestream", "testdata", relPath))
	if err != nil {
		t.Fatalf("readTestdataFrame(%q): %v", relPath, err)
	}
	return string(b)
}

// ===================== T1 — liveness-detector-default-strategy =====================

// TestC423_001_LivenessStateEnumExists asserts all four LivenessState constants exist in
// the panestream package. This compile-time check confirms the enum vocabulary Builder
// must provide. RED: panestream.LivenessState and its constants are absent → compile error.
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

// TestC423_002_DefaultDetectorConvergingAllCLIs asserts DefaultDetector classifies
// Converging for all four CLIs when primed on a thinking frame then fed an answer frame
// with new stable content lines. Closes the codex weak-signal gap: codex has no
// PaneBusy affordance on its idle boot frame (thinking.txt) — growth velocity alone
// must classify it Converging.
// RED: panestream.NewDefaultDetector absent → compile error.
func TestC423_002_DefaultDetectorConvergingAllCLIs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cases := []struct {
		cli         string
		thinkFrame  string
		answerFrame string
	}{
		{"claude", "claude/thinking.txt", "claude/answer.txt"},
		{"codex", "codex/thinking.txt", "codex/answer.txt"}, // idle boot → answer (weak-signal closure)
		{"agy", "agy/thinking.txt", "agy/answer.txt"},
		{"ollama", "ollama/thinking.txt", "ollama/answer.txt"},
	}
	for _, c := range cases {
		t.Run(c.cli, func(t *testing.T) {
			profile := panestream.Profiles[c.cli]
			det := panestream.NewDefaultDetector(3) // stallThreshold=3
			think := readTestdataFrame(t, root, c.thinkFrame)
			answer := readTestdataFrame(t, root, c.answerFrame)
			det.Assess(think, profile) // prime: absorb baseline, no state returned
			state, _ := det.Assess(answer, profile)
			if state != panestream.LivenessConverging {
				t.Errorf("[%s] thinking→answer: got state %v, want LivenessConverging", c.cli, state)
			}
		})
	}
}

// TestC423_003_BusyNoContentIsBusyButStagnant asserts that a busy pane with NO new content
// lines (spinner/affordance chrome only, no agent transcript) classifies as BusyButStagnant,
// NOT Converging. Negative test: the key anti-no-op check — a detector that returns Converging
// for chrome-only frames would accept a hung noisy spinner as "working."
// RED: panestream.NewDefaultDetector absent → compile error.
func TestC423_003_BusyNoContentIsBusyButStagnant(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewDefaultDetector(3)
	// Prime on a frame that has some content so the baseline is non-empty.
	det.Assess("⏺ agent output here\n❯ \n", profile)
	// Feed a frame where only chrome changed (spinner/affordance), same content.
	// IsAffordanceLine matches this line; IsContentLine does not.
	spinnerFrame := "⏺ agent output here\n✽ Inferring… (5s · ↓ 10 tokens)\n❯ \n"
	state, _ := det.Assess(spinnerFrame, profile)
	if state == panestream.LivenessConverging {
		t.Errorf("busy-but-no-content frame classified Converging (got %v); spinner-only delta must NOT equal Converging", state)
	}
	// The expected state is BusyButStagnant (busy=true via affordance, no content growth).
	if state != panestream.LivenessBusyButStagnant {
		t.Errorf("busy-but-no-content frame: got %v, want LivenessBusyButStagnant", state)
	}
}

// TestC423_004_QuietFrameIsIdleNotHung asserts that a quiet frame (no busy affordance,
// no new content) after a single interval classifies as Idle, NOT Hung. Edge test: Hung
// requires sustained stalling for stallThreshold intervals — a single quiet interval must
// NOT trigger the fast-fail path. Priming variant: priming an empty input box must also
// not produce Hung immediately.
// RED: panestream.NewDefaultDetector absent → compile error.
func TestC423_004_QuietFrameIsIdleNotHung(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewDefaultDetector(3) // stallThreshold=3

	// Variant A: quiet frame (same content, no busy signal).
	det.Assess("⏺ some output\n❯ \n", profile) // prime
	state, _ := det.Assess("⏺ some output\n❯ \n", profile)
	if state == panestream.LivenessHung {
		t.Errorf("single quiet interval must NOT be Hung (got %v); Hung requires %d stall intervals", state, 3)
	}
	if state != panestream.LivenessIdle {
		t.Errorf("single quiet interval: got %v, want LivenessIdle", state)
	}

	// Variant B: empty/priming input-box only.
	det2 := panestream.NewDefaultDetector(3)
	det2.Assess("❯ \n", profile) // prime on empty
	state2, _ := det2.Assess("❯ \n", profile)
	if state2 == panestream.LivenessHung {
		t.Errorf("empty-frame single interval must NOT be Hung (got %v)", state2)
	}
}

// TestC423_005_HungAfterStallThreshold asserts DefaultDetector classifies Hung after exactly
// stallThreshold consecutive busy-but-no-content intervals. Uses stallThreshold=2 so the test
// exercises the threshold boundary directly (interval N-1 = BusyButStagnant; interval N = Hung).
// RED: panestream.NewDefaultDetector absent → compile error.
func TestC423_005_HungAfterStallThreshold(t *testing.T) {
	profile := panestream.Profiles["claude"]
	const stall = 2
	det := panestream.NewDefaultDetector(stall)
	// Prime on a content frame.
	det.Assess("⏺ initial content\n❯ \n", profile)
	// Busy-but-stagnant interval 1 (stall_count=1 < threshold=2).
	spinnerFrame := "⏺ initial content\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n"
	s1, _ := det.Assess(spinnerFrame, profile)
	if s1 == panestream.LivenessHung {
		t.Fatalf("interval 1/%d: must NOT be Hung yet (got %v)", stall, s1)
	}
	// Busy-but-stagnant interval 2 (stall_count=2 = threshold → Hung).
	spinnerFrame2 := "⏺ initial content\n✽ Thinking… (10s · ↓ 120 tokens)\n❯ \n"
	s2, _ := det.Assess(spinnerFrame2, profile)
	if s2 != panestream.LivenessHung {
		t.Errorf("interval 2/%d: got %v, want LivenessHung (stall threshold reached)", stall, s2)
	}
}

// TestC423_006_DetectorForRegistryAllCLIs asserts DetectorFor returns a non-nil LivenessProbe
// for every CLI in panestream.Profiles and that calling Assess on the thinking→answer
// sequence produces a valid LivenessState. Behavioral: calls the registry, feeds real frames,
// asserts on the returned state.
// RED: panestream.DetectorFor and panestream.LivenessProbe absent → compile error.
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

// ===================== T2 — liveness-detector-reviewer-integration =====================

// TestC423_007_NoCliNameInStopReview asserts stopreview.go contains no hardcoded CLI-name
// string literals — the structural invariant proving the reviewer is driver-agnostic.
// This structural check is the ONLY mechanical way to enforce a design constraint
// (zero CLI-branch strings in the reviewer); it is not a source-grep predicate
// over the feature under test but a no-regression guard over the abstraction boundary.
// Declared // acs-predicate: config-check per the single-source waiver.
// pre-existing GREEN: current stopreview.go contains no CLI name strings.
func TestC423_007_NoCliNameInStopReview(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		acsassert.FileNotContains(t, path, `"`+cli+`"`)
	}
}

// TestC423_008_StopEventHasStateField asserts StopEvent carries a State field of type
// panestream.LivenessState — the primary compile-check proving T2 wiring is present.
// RED: StopEvent.State field absent → compile error.
func TestC423_008_StopEventHasStateField(t *testing.T) {
	ev := bridge.StopEvent{}
	ev.State = panestream.LivenessConverging
	if ev.State != panestream.LivenessConverging {
		t.Fatal("unreachable: StopEvent.State field compile check")
	}
}

// TestC423_009_ConvergingExtendsUnconditionally asserts the reviewer returns ReviewExtend
// for State=Converging even at an attempt number past maxExtends. A converging agent
// must NEVER be capped — this is the fix for cycles 311/312 where a producing scout
// was killed mid-work by the backstop.
// RED: bridge.NewDeterministicReviewer or StopEvent.State absent → compile error.
func TestC423_009_ConvergingExtendsUnconditionally(t *testing.T) {
	r := bridge.NewDeterministicReviewer(2) // maxExtends=2
	cases := []int{0, 1, 2, 3, 9}           // 3 and 9 are past maxExtends=2
	for _, attempt := range cases {
		ev := bridge.StopEvent{State: panestream.LivenessConverging, Attempt: attempt}
		verdict := r.Review(ev)
		if verdict.Action != bridge.ReviewExtend {
			t.Errorf("Converging at attempt=%d (maxExtends=2): got %q, want ReviewExtend — a converging agent must never be capped", attempt, verdict.Action)
		}
	}
}

// TestC423_010_HungFastFailsUnderMaxExtends asserts the reviewer returns a non-Extend action
// for State=Hung even at attempt=1 (well under maxExtends=6). Hung must fast-fail BEFORE
// the 30-min backstop — this prevents a genuinely-hung agent from holding the loop for
// ~maxExtends×300s when it is clearly stalled.
// RED: bridge.NewDeterministicReviewer or StopEvent.State absent → compile error.
func TestC423_010_HungFastFailsUnderMaxExtends(t *testing.T) {
	r := bridge.NewDeterministicReviewer(6) // maxExtends=6
	ev := bridge.StopEvent{State: panestream.LivenessHung, Attempt: 1}
	verdict := r.Review(ev)
	if verdict.Action == bridge.ReviewExtend {
		t.Errorf("Hung at attempt=1 (under maxExtends=6): got ReviewExtend, want non-extend (fast-fail path); Hung must not wait for the ~30-min backstop")
	}
}

// ===================== T3 — liveness-claude-tokencount-strategy =====================

// TestC423_011_IncreasingTokensHigherConfConverging asserts that a strictly-increasing
// ↓ token counter observed over multiple intervals causes the claude-specific detector to
// classify Converging with higher confidence than the default detector, even when there
// are no new content lines (pure chrome). Activates the dormant extractTokenCount signal.
// RED: panestream.NewClaudeDetector absent → compile error.
func TestC423_011_IncreasingTokensHigherConfConverging(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewClaudeDetector(3)
	base := panestream.NewDefaultDetector(3)

	// Three consecutive frames with a strictly increasing ↓ token counter.
	// No content lines (pure chrome) so DefaultDetector sees BusyButStagnant.
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

	// Claude detector must classify Converging (monotonic token growth = live model output).
	if claudeState != panestream.LivenessConverging {
		t.Errorf("increasing tokens: claude detector got %v, want LivenessConverging (token growth proves live work)", claudeState)
	}
	// Claude must have strictly higher confidence than the default on these frames.
	if claudeConf <= baseConf {
		t.Errorf("increasing tokens: claude confidence %v not > default %v (base state=%v)", claudeConf, baseConf, baseState)
	}
}

// TestC423_012_StaticTokenFallsBackToDefault asserts a static (non-increasing) token counter
// does NOT elevate the claude detector above the default. Negative test: static count alone
// must not read as Converging — the token must be monotonically increasing.
// RED: panestream.NewClaudeDetector absent → compile error.
func TestC423_012_StaticTokenFallsBackToDefault(t *testing.T) {
	profile := panestream.Profiles["claude"]
	det := panestream.NewClaudeDetector(3)
	base := panestream.NewDefaultDetector(3)

	// Same token count on every frame = static counter.
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

// TestC423_013_MalformedTokenNoPanic asserts the claude detector never panics on malformed
// token lines and produces no confidence bump above the default. Edge/OOD test.
// RED: panestream.NewClaudeDetector absent → compile error.
func TestC423_013_MalformedTokenNoPanic(t *testing.T) {
	profile := panestream.Profiles["claude"]
	malformedFrames := []string{
		"✽ Generating… (↓ tokens)\n❯ \n",        // digits missing entirely
		"✽ Generating… (5s · ↓ k tokens)\n❯ \n", // k without leading digits
		"✽ Generating…\n❯ \n",                   // no token line at all
		"✽ ↓ not-a-number tokens\n❯ \n",         // non-numeric token field
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
			base.Assess("⏺ content\n❯ \n", profile) // prime
			det.Assess("⏺ content\n❯ \n", profile)
			_, baseConf := base.Assess(frame, profile)
			_, claudeConf := det.Assess(frame, profile)
			if claudeConf > baseConf {
				t.Errorf("malformed frame elevated confidence: claude %v > default %v (frame=%q)", claudeConf, baseConf, frame)
			}
		})
	}
}

// TestC423_014_NonClaudeDetectorUnaffectedByClaudeLayer asserts that codex, agy, and ollama
// detectors returned by DetectorFor produce the same LivenessState as NewDefaultDetector for
// the same frame sequence. The claude-specific token layer must NOT contaminate other CLIs.
// RED: panestream.DetectorFor or panestream.NewDefaultDetector absent → compile error.
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
			// Confidence must also match (no token-layer uplift on non-claude CLIs).
			if detConf != baseConf {
				t.Errorf("[%s] DetectorFor confidence %v ≠ DefaultDetector %v (claude layer must not elevate non-claude confidence)", cli, detConf, baseConf)
			}
		})
	}
}
