package panestream

import (
	"strings"
	"testing"
)

const (
	agyBoundaryLine = ">"
	agySpinnerText  = "⣯ Generating..."
	agyEscText      = "esc to cancel"
)

// agyFrame appends the empty ">" input box to the content lines.
func agyFrame(contentLines ...string) string {
	var sb strings.Builder
	for _, line := range contentLines {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	sb.WriteString(agyBoundaryLine)
	sb.WriteByte('\n')
	return sb.String()
}

func TestAmp_AgyDetector_PrimeCallReturnsBaseBehavior(t *testing.T) {
	p := Profiles["agy"]
	generating := testdataFrame(t, "agy/thinking.txt")

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)

	detState, detConf := det.Assess(generating, p)    // prime
	baseState, baseConf := base.Assess(generating, p) // prime

	if detState != baseState {
		t.Errorf("prime call on generating frame: AgyDetector state %v ≠ DefaultDetector %v; "+
			"prime must return base behavior — no Converging uplift before baseline is established", detState, baseState)
	}
	if detConf != baseConf {
		t.Errorf("prime call on generating frame: AgyDetector conf %.2f ≠ DefaultDetector %.2f; "+
			"prime call must be byte-identical to DefaultDetector (spec: 'prime: returns base')", detConf, baseConf)
	}
}

func TestAmp_AgyDetector_PartialSpinnerNotConverging(t *testing.T) {
	p := Profiles["agy"]
	const partialSpinner = "⣯ Generat" // deliberately truncated — missing "ing..."
	primeFrame := agyFrame("> what is tmux?")
	partialFrame := agyFrame("> what is tmux?", partialSpinner)

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)
	det.Assess(primeFrame, p)
	base.Assess(primeFrame, p)

	detState, detConf := det.Assess(partialFrame, p)
	_, baseConf := base.Assess(partialFrame, p)

	if detState == LivenessConverging && detConf > baseConf+0.15 {
		t.Errorf("partial spinner %q: AgyDetector returned Converging (conf=%.2f > default+0.15=%.2f); "+
			"incomplete spinner text must not trigger the ⣯ Generating... convergence signal", partialSpinner, detConf, baseConf+0.15)
	}
}

func TestAmp_AgyDetector_SpinnerEmbeddedMidLineNotDetected(t *testing.T) {
	p := Profiles["agy"]
	primeFrame := agyFrame("> what is tmux?")
	embeddedFrame := agyFrame("> what is tmux?", "prefix text "+agySpinnerText+" suffix text")

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)
	det.Assess(primeFrame, p)
	base.Assess(primeFrame, p)

	detState, detConf := det.Assess(embeddedFrame, p)
	baseState, baseConf := base.Assess(embeddedFrame, p)

	if detState != baseState {
		t.Errorf("spinner embedded mid-line: AgyDetector state %v ≠ DefaultDetector %v; "+
			"spinner must be a standalone (trimmed) line to qualify as convergence signal — "+
			"mid-line presence must not trigger (would catch a strings.Contains shortcut bug)", detState, baseState)
	}
	if detConf > baseConf+0.10 {
		t.Errorf("spinner embedded mid-line: AgyDetector conf %.2f significantly above DefaultDetector %.2f; "+
			"embedded spinner must not elevate confidence above default path", detConf, baseConf)
	}
}

func TestAmp_AgyDetector_EscToCancelAloneNotConverging(t *testing.T) {
	p := Profiles["agy"]
	primeFrame := agyFrame("> what is tmux?")
	escOnlyFrame := agyFrame("> what is tmux?", agyEscText) // busy affordance, no spinner

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)
	det.Assess(primeFrame, p)
	base.Assess(primeFrame, p)

	detState, detConf := det.Assess(escOnlyFrame, p)
	baseState, baseConf := base.Assess(escOnlyFrame, p)

	if detState != baseState {
		t.Errorf("esc-to-cancel alone (no spinner): AgyDetector state %v ≠ DefaultDetector %v; "+
			"busy affordance alone must NOT trigger Converging — only ⣯ Generating... qualifies as "+
			"AgyDetector's convergence signal", detState, baseState)
	}
	if detConf > baseConf+0.05 {
		t.Errorf("esc-to-cancel alone: AgyDetector conf %.2f meaningfully above DefaultDetector %.2f; "+
			"no confidence uplift without spinner", detConf, baseConf)
	}
}

func TestAmp_AgyDetector_OscillationSpinnerAnswerSpinner(t *testing.T) {
	p := Profiles["agy"]
	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")
	answerFrame := testdataFrame(t, "agy/answer.txt")

	for i, frame := range []string{answerFrame, thinkingFrame, answerFrame, thinkingFrame} {
		detState, detConf := det.Assess(frame, p)
		baseState, baseConf := base.Assess(frame, p)
		if detState != baseState || detConf != baseConf {
			t.Errorf("step %d: AgyDetector (%v, %.2f) != DefaultDetector (%v, %.2f); a spinner frame is busy evidence, never a progress uplift",
				i, detState, detConf, baseState, baseConf)
		}
	}
}

func TestAmp_AgyDetector_RepeatedFrozenSpinnerFramesWalkToHung(t *testing.T) {
	p := Profiles["agy"]
	det := NewAgyDetector(3)
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")

	det.Assess(thinkingFrame, p)

	var state LivenessState
	for i := 1; i <= 6; i++ {
		state, _ = det.Assess(thinkingFrame, p)
		if state == LivenessConverging {
			t.Fatalf("rep %d: a frozen spinner frame read Converging; the spinner proves the TUI draws, not that the agent progresses", i)
		}
	}
	if state != LivenessHung {
		t.Errorf("after 6 identical spinner frames got %v, want Hung", state)
	}
}

func TestAmp_AgyDetector_MalformedEdgeCasesNoPanic(t *testing.T) {
	p := Profiles["agy"]
	cases := []struct {
		name  string
		frame string
	}{
		{"empty", ""},
		{"whitespace-only", "   \n  \n"},
		{"partial-spinner-only", "⣯ Generat\n>\n"},
		{"spinner-suffix-only", "Generating...\n>\n"}, // missing leading "⣯ " rune
		{"binary-garbage", "\x00\xff\xfe\n>\n"},
		{"newlines-only", "\n\n\n"},
		{"only-boundary", ">\n"},
		{"esc-only", agyEscText + "\n>\n"},
		{"spinner-mid-line-only", "x " + agySpinnerText + " y\n>\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("AgyDetector panicked on %q: %v", tc.name, r)
				}
			}()
			det := NewAgyDetector(3)
			base := NewDefaultDetector(3)
			det.Assess(tc.frame, p)  // prime
			base.Assess(tc.frame, p) // prime

			_, baseConf := base.Assess(tc.frame, p)
			_, detConf := det.Assess(tc.frame, p)

			if detConf < 0 || detConf > 1.0 {
				t.Errorf("AgyDetector [%s]: confidence %v out of [0,1]", tc.name, detConf)
			}
			if detConf > baseConf+0.05 {
				t.Errorf("AgyDetector [%s]: conf %.2f meaningfully above DefaultDetector %.2f; "+
					"malformed/edge frames must not elevate confidence", tc.name, detConf, baseConf)
			}
		})
	}
}

func TestAmp_AgyDetector_WithNonAgyProfile(t *testing.T) {
	codexProfile := Profiles["codex"]
	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)

	codexThink := testdataFrame(t, "codex/thinking.txt")
	codexAnswer := testdataFrame(t, "codex/answer.txt")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AgyDetector with codex profile panicked: %v", r)
		}
	}()

	det.Assess(codexThink, codexProfile)  // prime
	base.Assess(codexThink, codexProfile) // prime

	detState, detConf := det.Assess(codexAnswer, codexProfile)
	baseState, baseConf := base.Assess(codexAnswer, codexProfile)

	if detState != baseState || detConf != baseConf {
		t.Errorf("AgyDetector with codex profile: (%v, %.2f) ≠ DefaultDetector (%v, %.2f); "+
			"codex frames contain no agy spinner — AgyDetector must be byte-identical to DefaultDetector", detState, detConf, baseState, baseConf)
	}
}

func TestAmp_AgyDetector_FrozenSpinnerHonoursTheStallThreshold(t *testing.T) {
	p := Profiles["agy"]
	det := NewAgyDetector(100)
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")

	det.Assess(thinkingFrame, p)

	state, _ := det.Assess(thinkingFrame, p)
	if state != LivenessBusyButStagnant {
		t.Errorf("frozen spinner with stallThreshold=100: got %v, want BusyButStagnant until the threshold is reached", state)
	}
}

func TestAmp_AgyDetector_GeneratingConfidenceEqualsDefault(t *testing.T) {
	p := Profiles["agy"]
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)

	det.Assess(thinkingFrame, p)
	base.Assess(thinkingFrame, p)

	_, detConf := det.Assess(thinkingFrame, p)
	_, baseConf := base.Assess(thinkingFrame, p)

	if detConf != baseConf {
		t.Errorf("generating frame: conf %.2f must equal DefaultDetector %.2f; the spinner carries no progress uplift", detConf, baseConf)
	}
}

func TestAmp_AgyDetector_ExtendedAnswerFrameParityManyIterations(t *testing.T) {
	p := Profiles["agy"]
	answerFrame := testdataFrame(t, "agy/answer.txt")

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)

	det.Assess(answerFrame, p)  // prime
	base.Assess(answerFrame, p) // prime

	for i := 1; i <= 10; i++ {
		detState, detConf := det.Assess(answerFrame, p)
		baseState, baseConf := base.Assess(answerFrame, p)

		if detState != baseState {
			t.Errorf("answer-frame iteration %d/10: AgyDetector state %v ≠ DefaultDetector %v; "+
				"non-generating frames must be byte-identical across all iterations (composition stability)", i, detState, baseState)
		}
		if detConf != baseConf {
			t.Errorf("answer-frame iteration %d/10: AgyDetector conf %.4f ≠ DefaultDetector %.4f; "+
				"confidence must not drift from default path over repeated non-generating calls", i, detConf, baseConf)
		}
	}
}
