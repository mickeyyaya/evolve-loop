package panestream

import (
	"fmt"
	"strings"
	"testing"
)

func TestAmp_DefaultDetector_StallCounterResetsOnContent(t *testing.T) {
	profile := Profiles["claude"]
	const stall = 3
	det := NewDefaultDetector(stall)

	det.Assess("⏺ initial output\n❯ \n", profile) // prime

	spinner := "⏺ initial output\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n"
	for i := 0; i < stall-1; i++ {
		s, _ := det.Assess(spinner, profile)
		if s == LivenessHung {
			t.Fatalf("stall interval %d/%d: must NOT be Hung yet (threshold=%d)", i+1, stall-1, stall)
		}
	}

	det.Assess("⏺ initial output\n⏺ new answer line\n❯ \n", profile)

	spinner2 := "⏺ initial output\n⏺ new answer line\n✽ Thinking… (20s · ↓ 200 tokens)\n❯ \n"
	for i := 0; i < stall-1; i++ {
		s, _ := det.Assess(spinner2, profile)
		if s == LivenessHung {
			t.Fatalf("post-reset stall interval %d/%d: stall counter must have reset on content growth; got Hung at only %d stall(s), threshold=%d", i+1, stall-1, i+1, stall)
		}
	}

	s, _ := det.Assess(spinner2, profile)
	if s != LivenessHung {
		t.Errorf("post-reset stall × %d: got %v, want LivenessHung (threshold=%d should be reached again)", stall, s, stall)
	}
}

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

func TestAmp_DefaultDetector_StallThresholdOneBoundary(t *testing.T) {
	profile := Profiles["claude"]
	det := NewDefaultDetector(1) // threshold=1: first stall → Hung immediately

	det.Assess("⏺ content\n❯ \n", profile) // prime
	s, _ := det.Assess("⏺ content\n✽ Thinking… (5s · ↓ 50 tokens)\n❯ \n", profile)
	if s != LivenessHung {
		t.Errorf("stallThreshold=1: first stall interval got %v, want LivenessHung", s)
	}
}

func TestAmp_ClaudeDetector_DecreasingTokenFallsBackToDefault(t *testing.T) {
	profile := Profiles["claude"]
	det := NewClaudeDetector(3)
	base := NewDefaultDetector(3)

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

func TestAmp_DefaultDetector_QuietFramesNeverHung(t *testing.T) {
	profile := Profiles["claude"]
	const stall = 2
	det := NewDefaultDetector(stall)

	det.Assess("⏺ content\n❯ \n", profile) // prime

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

func TestAmp_ClaudeDetector_UploadArrowNotConvergenceSignal(t *testing.T) {
	profile := Profiles["claude"]
	det := NewClaudeDetector(3)
	base := NewDefaultDetector(3)

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
