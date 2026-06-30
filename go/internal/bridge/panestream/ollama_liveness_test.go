package panestream

import (
	"testing"
)

// TestOllamaDetector_ThinkingConverging (eval AC1): OllamaDetector must return
// LivenessConverging at higher confidence than DefaultDetector when the pane
// contains the "Thinking..." header but no new content delta appeared.
func TestOllamaDetector_ThinkingConverging(t *testing.T) {
	p := Profiles["ollama"]
	base := NewDefaultDetector(3)
	det := NewOllamaDetector(3)

	// Minimal thinking frame: "Thinking..." is present; no idle "Send a message".
	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"

	// Prime both on the initial thinking frame.
	base.Assess(thinkingFrame, p)
	det.Assess(thinkingFrame, p)

	// Feed the same frame again: no new content lines → DefaultDetector BusyButStagnant.
	baseState, baseConf := base.Assess(thinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(thinkingFrame, p)

	if ollamaState != LivenessConverging {
		t.Errorf("OllamaDetector on Thinking... frame: got %v, want LivenessConverging (DefaultDetector got %v)", ollamaState, baseState)
	}
	if ollamaConf <= baseConf {
		t.Errorf("OllamaDetector confidence %v not > DefaultDetector %v on Thinking... frame; uplift required", ollamaConf, baseConf)
	}
}

// TestOllamaDetector_StaticFallsBack (eval AC2): when "Thinking..." is absent,
// OllamaDetector must produce byte-identical state+confidence to DefaultDetector.
func TestOllamaDetector_StaticFallsBack(t *testing.T) {
	p := Profiles["ollama"]
	base := NewDefaultDetector(3)
	det := NewOllamaDetector(3)

	// Completed answer frame: no "Thinking..." header, idle placeholder present.
	noThinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\n*   tmux is a terminal multiplexer.\n*   It keeps sessions alive.\n>>> Send a message (/? for help)\n"

	for range 3 {
		base.Assess(noThinkingFrame, p)
		det.Assess(noThinkingFrame, p)
	}
	baseState, baseConf := base.Assess(noThinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(noThinkingFrame, p)

	if ollamaState != baseState {
		t.Errorf("OllamaDetector (no Thinking...): state %v ≠ DefaultDetector %v", ollamaState, baseState)
	}
	if ollamaConf != baseConf {
		t.Errorf("OllamaDetector (no Thinking...): conf %v ≠ DefaultDetector %v", ollamaConf, baseConf)
	}
}

// TestOllamaDetector_Malformed (eval AC3): malformed/garbage headers must not
// panic and must not elevate confidence above default.
func TestOllamaDetector_Malformed(t *testing.T) {
	p := Profiles["ollama"]
	edgeCases := []struct {
		name  string
		frame string
	}{
		{"empty", ""},
		{"whitespace-only", "   \n  \n"},
		{"partial-thinking", "Thinking\n"},          // missing "..."
		{"thinking-as-suffix", "DeepThinking...\n"}, // not standalone
		{"pure-garbage", "\x00\xff\xfe\n"},
		{"done-thinking-only", "...done thinking.\n"},
	}
	for _, tc := range edgeCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("OllamaDetector panicked on edge case %q: %v", tc.name, r)
				}
			}()
			base := NewDefaultDetector(3)
			det := NewOllamaDetector(3)
			base.Assess(tc.frame, p) // prime
			det.Assess(tc.frame, p)
			_, baseConf := base.Assess(tc.frame, p)
			_, detConf := det.Assess(tc.frame, p)
			if detConf > baseConf {
				t.Errorf("edge case %q: OllamaDetector conf %v > default %v (must not elevate on malformed signal)", tc.name, detConf, baseConf)
			}
		})
	}
}

// TestDetectorFor_OllamaRoutesOllama (eval AC4 router check): DetectorFor must
// route "ollama" to the OllamaDetector (verified by confidence uplift on a
// thinking frame that DefaultDetector would not uplift).
func TestDetectorFor_OllamaRoutesOllama(t *testing.T) {
	p := Profiles["ollama"]
	probe := DetectorFor(p)
	if probe == nil {
		t.Fatal("DetectorFor(ollama) = nil")
	}
	base := NewDefaultDetector(0)

	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"
	probe.Assess(thinkingFrame, p)
	base.Assess(thinkingFrame, p)

	_, baseConf := base.Assess(thinkingFrame, p)
	_, probeConf := probe.Assess(thinkingFrame, p)

	if probeConf <= baseConf {
		t.Errorf("DetectorFor(ollama) confidence %v not > DefaultDetector %v on Thinking... frame; DetectorFor must route to OllamaDetector", probeConf, baseConf)
	}
}

// TestDetectorFor_CodexAgyUnchanged (eval AC4 non-regression): DetectorFor must
// still return DefaultDetector-equivalent for codex and agy (OllamaDetector is
// additive, not a replacement for those CLIs).
func TestDetectorFor_CodexAgyUnchanged(t *testing.T) {
	for _, cli := range []string{"codex", "agy"} {
		cli := cli
		t.Run(cli, func(t *testing.T) {
			p := Profiles[cli]
			probe := DetectorFor(p)
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
