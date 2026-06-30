package panestream

import (
	"strings"
	"testing"
)

// agy_liveness_amplify_test.go — adversarial amplification for AgyDetector (cycle-425).
// Written by the Test Amplifier — black-box view, spec-only, no implementation reading.
//
// Coverage gaps targeted beyond C425_004–C425_006 and the ACS predicates:
//
//	A1  Prime call on generating frame returns base behavior (not Converging uplift)
//	A2  Partial spinner text ("⣯ Generat") is NOT a convergence signal
//	A3  Spinner embedded mid-line (not standalone) is NOT detected
//	A4  "esc to cancel" alone (no spinner) is NOT a convergence signal
//	A5  Spinner→answer→spinner oscillation: Converging correctly re-fires on spinner return
//	A6  Multiple consecutive generating frames each return Converging (signal persists)
//	A7  Malformed edge cases: no panic, confidence never meaningfully above DefaultDetector
//	A8  AgyDetector with non-agy profile: no panic, graceful fallback
//	A9  Spinner overrides high stallThreshold — Converging is independent of stall config
//	A10 Confidence strictly > DefaultDetector on generating frame (direct construction path)
//	A11 Extended answer-frame parity across 10+ iterations (long-run composition stability)

const (
	agyBoundaryLine = ">"               // exact boundary marker for agy profile (BoundaryExact=true)
	agySpinnerText  = "⣯ Generating..." // the complete spinner signal per spec
	agyEscText      = "esc to cancel"   // agy busy affordance (busyAffordanceRE match)
)

// agyFrame constructs a minimal agy pane frame from content lines.
// Appends the exact boundary marker ">" as the final prompt line (BoundaryExact=true).
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

// TestAmp_AgyDetector_PrimeCallReturnsBaseBehavior verifies the "prime: returns base"
// invariant: the FIRST Assess call on ANY frame — including a generating frame — must
// return the same (state, confidence) as DefaultDetector on the same first call.
// C425_004 only tests the second call (after prime establishes baseline); this pins the
// prime contract. A bug here would cause spurious Converging on the very first frame.
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

// TestAmp_AgyDetector_PartialSpinnerNotConverging verifies that a truncated spinner
// ("⣯ Generat" without the trailing "ing...") is NOT treated as a convergence signal.
// Partial match would be a gaming vulnerability: a frame whose content happens to start
// with the spinner's rune prefix could fake convergence. The full token is required.
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

	// If AgyDetector fires Converging AND has meaningfully higher confidence than
	// DefaultDetector, the spinner layer incorrectly matched the partial token.
	if detState == LivenessConverging && detConf > baseConf+0.15 {
		t.Errorf("partial spinner %q: AgyDetector returned Converging (conf=%.2f > default+0.15=%.2f); "+
			"incomplete spinner text must not trigger the ⣯ Generating... convergence signal", partialSpinner, detConf, baseConf+0.15)
	}
}

// TestAmp_AgyDetector_SpinnerEmbeddedMidLineNotDetected verifies that the spinner
// token embedded within a longer line ("prefix ⣯ Generating... suffix") is NOT treated
// as a convergence signal. Spec: "complete trimmed line" — the spinner must stand alone
// on the line after trimming. A naive strings.Contains implementation would fail here.
func TestAmp_AgyDetector_SpinnerEmbeddedMidLineNotDetected(t *testing.T) {
	p := Profiles["agy"]
	primeFrame := agyFrame("> what is tmux?")
	// Spinner embedded mid-line — not a standalone line when trimmed.
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

// TestAmp_AgyDetector_EscToCancelAloneNotConverging verifies that agy's busy affordance
// "esc to cancel" alone — without "⣯ Generating..." — does NOT trigger AgyDetector's
// Converging uplift. PaneBusy and AgyDetector's convergence signal are independent:
// the spinner is the required signal, not any busy chrome. A bug here could cause
// false-positive convergence on any agy busy-but-stagnant turn that lacks the spinner.
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

// TestAmp_AgyDetector_OscillationSpinnerAnswerSpinner verifies that AgyDetector
// correctly oscillates between Converging (generating) and non-Converging (answer)
// across a realistic turn lifecycle. The key invariant: on answer frames, AgyDetector
// must be BYTE-IDENTICAL to DefaultDetector (AC2); on spinner frames, it must upgrade
// to Converging while DefaultDetector does not.
func TestAmp_AgyDetector_OscillationSpinnerAnswerSpinner(t *testing.T) {
	p := Profiles["agy"]
	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")
	answerFrame := testdataFrame(t, "agy/answer.txt")

	det.Assess(answerFrame, p)  // prime
	base.Assess(answerFrame, p) // prime (oracle)

	// Step 1: spinner → AgyDetector must Converge with uplift; base must not.
	s1, c1 := det.Assess(thinkingFrame, p)
	bs1, _ := base.Assess(thinkingFrame, p)
	if s1 != LivenessConverging {
		t.Errorf("oscillation [step 1, spinner]: got %v (conf %.2f), want LivenessConverging", s1, c1)
	}
	if s1 == bs1 {
		t.Errorf("oscillation [step 1]: AgyDetector and DefaultDetector both returned %v; "+
			"AgyDetector must upgrade to Converging on spinner while DefaultDetector does not", s1)
	}

	// Step 2: answer frame — must be BYTE-IDENTICAL to DefaultDetector (AC2).
	// DefaultDetector legitimately returns Converging here (content changed); the invariant is
	// parity with DefaultDetector, not a fixed expected state.
	s2Det, c2Det := det.Assess(answerFrame, p)
	s2Base, c2Base := base.Assess(answerFrame, p)
	if s2Det != s2Base {
		t.Errorf("oscillation [step 2, answer after spinner]: AgyDetector %v != DefaultDetector %v; "+
			"answer frame must be byte-identical to DefaultDetector (AC2)", s2Det, s2Base)
	}
	if c2Det != c2Base {
		t.Errorf("oscillation [step 2]: conf %.2f != DefaultDetector %.2f; must be byte-identical", c2Det, c2Base)
	}

	// Step 3: spinner again — must STILL Converge (no persistent state corruption).
	s3, c3 := det.Assess(thinkingFrame, p)
	if s3 != LivenessConverging {
		t.Errorf("oscillation [step 3, spinner again after answer]: got %v (conf %.2f), want LivenessConverging; "+
			"spinner signal must re-fire after an intervening answer frame", s3, c3)
	}
	if c3 < 0.9 {
		t.Errorf("oscillation [step 3]: conf=%.2f, want >=0.9 (spinner must restore full uplift)", c3)
	}
}

// TestAmp_AgyDetector_RepeatedSpinnerFramesAllConverging verifies that K consecutive
// generating frames each return Converging after prime. The signal must persist across
// repeated calls, not fire only on the first observation or degrade over time.
func TestAmp_AgyDetector_RepeatedSpinnerFramesAllConverging(t *testing.T) {
	p := Profiles["agy"]
	det := NewAgyDetector(3)
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")

	det.Assess(thinkingFrame, p) // prime

	const reps = 6
	for i := 1; i <= reps; i++ {
		state, conf := det.Assess(thinkingFrame, p)
		if state != LivenessConverging {
			t.Errorf("rep %d/%d: got %v, want LivenessConverging; "+
				"spinner must produce Converging on each subsequent call, not just the first", i, reps, state)
		}
		if conf < 0.9 {
			t.Errorf("rep %d/%d: conf=%.2f, want ≥0.9 (confidence must not degrade on repetition)", i, reps, conf)
		}
	}
}

// TestAmp_AgyDetector_MalformedEdgeCasesNoPanic verifies that malformed, partial,
// or degenerate frames do not panic and do not significantly elevate confidence
// above DefaultDetector. Mirrors TestOllamaDetector_Malformed for the agy strategy.
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

// TestAmp_AgyDetector_WithNonAgyProfile verifies that an AgyDetector used with a
// non-agy profile (codex) does not panic and behaves byte-identically to DefaultDetector.
// Codex pane frames do not contain "⣯ Generating...", so the agy spinner layer must
// never fire, and all assessments must fall through to the default path.
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

// TestAmp_AgyDetector_SpinnerOverridesHighStallThreshold verifies that the spinner
// convergence signal fires regardless of the underlying stallThreshold. With a very high
// threshold (100), the DefaultDetector would almost never reach LivenessHung — but the
// spinner layer must still independently classify LivenessConverging. This ensures the
// two classification paths (spinner-based vs stall-based) are orthogonal.
func TestAmp_AgyDetector_SpinnerOverridesHighStallThreshold(t *testing.T) {
	p := Profiles["agy"]
	det := NewAgyDetector(100) // extreme threshold: DefaultDetector almost never Hung
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")

	det.Assess(thinkingFrame, p) // prime

	state, conf := det.Assess(thinkingFrame, p)
	if state != LivenessConverging {
		t.Errorf("spinner with stallThreshold=100: got %v, want LivenessConverging; "+
			"spinner convergence must be independent of the stall threshold — spinner layer must override", state)
	}
	if conf < 0.9 {
		t.Errorf("spinner with stallThreshold=100: conf=%.2f, want ≥0.9 (no degradation from high threshold)", conf)
	}
}

// TestAmp_AgyDetector_GeneratingConfidenceStrictlyAboveDefault directly verifies
// the confidence uplift via fresh AgyDetector construction (not via DetectorFor).
// C425_004 checks conf≥0.9; C425_006 checks uplift via DetectorFor registry path.
// This test confirms the uplift applies when using NewAgyDetector directly — the
// improvement must not require going through the registry path.
func TestAmp_AgyDetector_GeneratingConfidenceStrictlyAboveDefault(t *testing.T) {
	p := Profiles["agy"]
	thinkingFrame := testdataFrame(t, "agy/thinking.txt")

	det := NewAgyDetector(3)
	base := NewDefaultDetector(3)

	det.Assess(thinkingFrame, p)  // prime
	base.Assess(thinkingFrame, p) // prime

	_, detConf := det.Assess(thinkingFrame, p)
	_, baseConf := base.Assess(thinkingFrame, p)

	if detConf <= baseConf {
		t.Errorf("NewAgyDetector generating frame: conf %.2f must be strictly > DefaultDetector %.2f; "+
			"spinner uplift must apply via direct NewAgyDetector construction, not only via DetectorFor", detConf, baseConf)
	}
}

// TestAmp_AgyDetector_ExtendedAnswerFrameParityManyIterations verifies that AgyDetector
// and DefaultDetector remain byte-identical across 10+ consecutive non-generating frames.
// C425_005 uses 4 calls; this extends it to catch drift that only manifests over many
// iterations — e.g. a stall counter that diverges due to a composition wiring bug where
// the stallThreshold isn't correctly forwarded to the underlying DefaultDetector.
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
