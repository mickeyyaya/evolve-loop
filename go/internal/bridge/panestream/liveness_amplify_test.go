package panestream

import (
	"fmt"
	"strings"
	"testing"
)

// Adversarial amplification tests for the LivenessDetector strategy layer (cycle-423).
// Written by the Test Amplifier — black-box view, spec only, no implementation reading.
//
// Coverage gaps targeted (not reached by ACS predicates C423_001–C423_014):
//   A1  Stall counter RESET on content growth (AC5 only tests monotonic accumulation)
//   A2  Confidence ∈ [0,1] for ALL state paths (AC6 only checks Converging via DetectorFor)
//   A3  stallThreshold=1 boundary (AC5 uses threshold=2 and tests N-1/N)
//   A4  ClaudeDetector: DECREASING token → falls back to default (AC11=increasing, AC12=static)
//   A5  ClaudeDetector: very large token value — no overflow, valid confidence
//   A6  DetectorFor: unknown/empty profile — no panic, returns valid probe
//   A7  Many quiet (Idle) frames: never reaches Hung (stall counter must not increment on Idle)
//   A8  Upload-direction (↑) token counter: not a Converging signal for ClaudeDetector

// TestAmp_DefaultDetector_StallCounterResetsOnContent verifies the "consecutive"
// invariant in the stall-threshold contract: injecting a content-growth frame after
// (N-1) stalls must reset the counter so the agent needs another full stallThreshold
// stalls to reach Hung. C423_005 only tests monotonic accumulation to threshold.
// A reset-bug would cause premature Hung after any content-then-stall sequence.
func TestAmp_DefaultDetector_StallCounterResetsOnContent(t *testing.T) {
	profile := Profiles["claude"]
	const stall = 3
	det := NewDefaultDetector(stall)

	// Prime on content.
	det.Assess("⏺ initial output\n❯ \n", profile)

	// Accumulate (stall-1) stall intervals — must NOT reach Hung yet.
	spinner := "⏺ initial output\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n"
	for i := 0; i < stall-1; i++ {
		s, _ := det.Assess(spinner, profile)
		if s == LivenessHung {
			t.Fatalf("stall interval %d/%d: must NOT be Hung yet (threshold=%d)", i+1, stall-1, stall)
		}
	}

	// Content growth must reset the counter.
	det.Assess("⏺ initial output\n⏺ new answer line\n❯ \n", profile)

	// After reset, (stall-1) further stalls must AGAIN be non-Hung.
	spinner2 := "⏺ initial output\n⏺ new answer line\n✽ Thinking… (20s · ↓ 200 tokens)\n❯ \n"
	for i := 0; i < stall-1; i++ {
		s, _ := det.Assess(spinner2, profile)
		if s == LivenessHung {
			t.Fatalf("post-reset stall interval %d/%d: stall counter must have reset on content growth; got Hung at only %d stall(s), threshold=%d", i+1, stall-1, i+1, stall)
		}
	}

	// Now hit threshold again to confirm counting still works after reset.
	s, _ := det.Assess(spinner2, profile)
	if s != LivenessHung {
		t.Errorf("post-reset stall × %d: got %v, want LivenessHung (threshold=%d should be reached again)", stall, s, stall)
	}
}

// TestAmp_DefaultDetector_ConfidenceAlwaysInRange verifies that for every
// LivenessState path the returned confidence value is always in [0.0, 1.0].
// C423_006 only checks this for the Converging path via DetectorFor + real frames.
func TestAmp_DefaultDetector_ConfidenceAlwaysInRange(t *testing.T) {
	profile := Profiles["claude"]
	det := NewDefaultDetector(2)

	det.Assess("⏺ content\n❯ \n", profile) // prime

	cases := []struct {
		tag  string
		pane string
	}{
		{"idle (quiet, same content)", "⏺ content\n❯ \n"},
		{"busy-stagnant (spinner, count=1)", "⏺ content\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n"},
		{"hung (spinner, count=2=threshold)", "⏺ content\n✽ Thinking… (10s · ↓ 100 tokens)\n❯ \n"},
		{"converging (new content line)", "⏺ content\n⏺ new output\n❯ \n"},
	}
	for _, c := range cases {
		t.Run(c.tag, func(t *testing.T) {
			_, conf := det.Assess(c.pane, profile)
			if conf < 0 || conf > 1.0 {
				t.Errorf("[%s] confidence %v out of [0,1]", c.tag, conf)
			}
		})
	}
}

// TestAmp_DefaultDetector_StallThresholdOneBoundary verifies the degenerate
// boundary stallThreshold=1: the FIRST busy-but-stagnant interval must immediately
// classify as Hung. C423_005 uses threshold=2 and tests the (N-1)/N boundary;
// threshold=1 is the limiting case where BusyButStagnant and Hung collapse.
func TestAmp_DefaultDetector_StallThresholdOneBoundary(t *testing.T) {
	profile := Profiles["claude"]
	det := NewDefaultDetector(1) // threshold=1: first stall → Hung immediately

	det.Assess("⏺ content\n❯ \n", profile) // prime
	s, _ := det.Assess("⏺ content\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n", profile)
	if s != LivenessHung {
		t.Errorf("stallThreshold=1: first stall interval got %v, want LivenessHung", s)
	}
}

// TestAmp_ClaudeDetector_DecreasingTokenFallsBackToDefault verifies that a
// DECREASING token counter does NOT elevate confidence above the default detector.
// AC11 covers increasing tokens; AC12 covers static tokens; this is the missing
// negative variant: a decreasing counter (terminal wrap / corrupted output) must
// not read as a monotonic growth signal.
func TestAmp_ClaudeDetector_DecreasingTokenFallsBackToDefault(t *testing.T) {
	profile := Profiles["claude"]
	det := NewClaudeDetector(3)
	base := NewDefaultDetector(3)

	// Strictly decreasing token sequence.
	frames := []string{
		"✽ Generating… (5s · ↓ 300 tokens)\n❯ \n",
		"✽ Generating… (10s · ↓ 200 tokens)\n❯ \n",
		"✽ Generating… (15s · ↓ 100 tokens)\n❯ \n",
	}
	for _, f := range frames[:2] {
		det.Assess(f, profile)
		base.Assess(f, profile)
	}
	_, baseConf := base.Assess(frames[2], profile)
	_, claudeConf := det.Assess(frames[2], profile)

	if claudeConf > baseConf {
		t.Errorf("decreasing tokens: claude confidence %v > default %v; a decreasing counter must not elevate confidence above the default detector", claudeConf, baseConf)
	}
}

// TestAmp_ClaudeDetector_LargeTokenNoOverflow verifies that a very large token
// count does not cause integer overflow, panic, or out-of-range confidence.
// C423_013 tests malformed formats; this tests a valid format with extreme numeric
// values that could overflow int32 or miscalculate if parsed with the wrong type.
func TestAmp_ClaudeDetector_LargeTokenNoOverflow(t *testing.T) {
	profile := Profiles["claude"]
	det := NewClaudeDetector(3)

	largeFrames := []string{
		"✽ Generating… (5s · ↓ 999999999 tokens)\n❯ \n",
		"✽ Generating… (10s · ↓ 9999999999 tokens)\n❯ \n",
	}
	det.Assess(largeFrames[0], profile) // prime

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("large token value caused panic: %v", r)
		}
	}()
	_, conf := det.Assess(largeFrames[1], profile)
	if conf < 0 || conf > 1.0 {
		t.Errorf("large token value: confidence %v out of [0,1]", conf)
	}
}

// TestAmp_DetectorFor_UnknownProfileReturnsProbe verifies that DetectorFor does
// not panic and returns a non-nil, callable probe for an unrecognized CLI name.
// AC6 only covers the 4 known CLIs; an unknown profile must fall back safely
// (presumably to DefaultDetector) rather than panicking or returning nil.
func TestAmp_DetectorFor_UnknownProfileReturnsProbe(t *testing.T) {
	unknown := PaneProfile{Name: "unknown-future-cli", BoundaryMarker: "$ "}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectorFor(unknown profile) panicked: %v", r)
		}
	}()

	probe := DetectorFor(unknown)
	if probe == nil {
		t.Fatal("DetectorFor(unknown profile) = nil; want non-nil probe (safe default expected)")
	}

	// Verify the probe is callable and returns valid values.
	probe.Assess("some content\n$ \n", unknown) // prime
	state, conf := probe.Assess("some more content\n$ \n", unknown)

	if conf < 0 || conf > 1.0 {
		t.Errorf("unknown-profile probe: confidence %v out of [0,1]", conf)
	}
	validStates := map[LivenessState]bool{
		LivenessConverging:      true,
		LivenessBusyButStagnant: true,
		LivenessIdle:            true,
		LivenessHung:            true,
	}
	if !validStates[state] {
		t.Errorf("unknown-profile probe: returned invalid LivenessState %v", state)
	}
}

// TestAmp_DefaultDetector_QuietFramesNeverHung verifies that feeding N > stallThreshold
// quiet frames (no busy affordance, same content) never produces Hung.
// Quiet intervals classify as Idle — the stall counter must NOT increment on Idle,
// only on BusyButStagnant (busy+no-content). C423_004 checks 1 quiet interval;
// this verifies the invariant holds across many repetitions.
func TestAmp_DefaultDetector_QuietFramesNeverHung(t *testing.T) {
	profile := Profiles["claude"]
	const stall = 2
	det := NewDefaultDetector(stall)

	det.Assess("⏺ content\n❯ \n", profile) // prime

	// Feed (stall+5) quiet frames: same content, no spinner/affordance.
	quietFrame := "⏺ content\n❯ \n"
	for i := 0; i < stall+5; i++ {
		s, _ := det.Assess(quietFrame, profile)
		if s == LivenessHung {
			t.Fatalf("quiet frame %d: got Hung; Idle frames must not increment the stall counter (only BusyButStagnant should)", i+1)
		}
		if s != LivenessIdle {
			t.Errorf("quiet frame %d: got %v, want LivenessIdle", i+1, s)
		}
	}
}

// TestAmp_ClaudeDetector_UploadArrowNotConvergenceSignal verifies that a token
// counter using the ↑ (upload/prompt) direction does NOT elevate the ClaudeDetector
// above the default. ADR-0047 classifies BOTH ↑ and ↓ as chrome affordance;
// only ↓ (response generation) is a download-growth signal. A monotonically
// increasing ↑ counter must not be misread as response generation evidence.
func TestAmp_ClaudeDetector_UploadArrowNotConvergenceSignal(t *testing.T) {
	profile := Profiles["claude"]
	det := NewClaudeDetector(3)
	base := NewDefaultDetector(3)

	// Monotonically increasing ↑ (upload) counter.
	uploadFrames := []string{
		"✽ Kneading… (5s · ↑ 50 tokens)\n❯ \n",
		"✽ Kneading… (10s · ↑ 150 tokens)\n❯ \n",
		"✽ Kneading… (15s · ↑ 300 tokens)\n❯ \n",
	}
	for _, f := range uploadFrames[:2] {
		det.Assess(f, profile)
		base.Assess(f, profile)
	}
	_, baseConf := base.Assess(uploadFrames[2], profile)
	_, claudeConf := det.Assess(uploadFrames[2], profile)

	if claudeConf > baseConf {
		t.Errorf("upload-arrow (↑) tokens: claude conf %v > default %v; ↑ direction must not be treated as response-convergence evidence", claudeConf, baseConf)
	}
}

// TestAmp_DefaultDetector_LargeFrameNoTimeout verifies that a very large rendered
// pane (2000+ content lines) does not panic, stack-overflow, or produce out-of-range
// confidence. No ACS predicate covers large-scale inputs; a naive O(n²) string-scan
// would degrade severely on panes with accumulated thousands of output lines.
func TestAmp_DefaultDetector_LargeFrameNoTimeout(t *testing.T) {
	profile := Profiles["claude"]
	det := NewDefaultDetector(3)

	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&sb, "⏺ Tool output line %d: processing repository data...\n", i)
	}
	sb.WriteString("❯ \n")
	bigFrame := sb.String()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("large frame caused panic: %v", r)
		}
	}()

	det.Assess(bigFrame, profile) // prime

	var sb2 strings.Builder
	for i := 0; i < 2001; i++ {
		fmt.Fprintf(&sb2, "⏺ Tool output line %d: processing repository data...\n", i)
	}
	sb2.WriteString("❯ \n")

	state, conf := det.Assess(sb2.String(), profile)
	if conf < 0 || conf > 1.0 {
		t.Errorf("large frame: confidence %v out of [0,1]", conf)
	}
	if state != LivenessConverging {
		t.Errorf("large frame with one new content line: got %v, want LivenessConverging", state)
	}
}
