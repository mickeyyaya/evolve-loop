package panestream

import (
	"testing"
)

func TestOllamaDetector_ThinkingConverging(t *testing.T) {
	p := Profiles["ollama"]
	base := NewDefaultDetector(3)
	det := NewOllamaDetector(3)

	// Minimal thinking frame: "Thinking..." is present; no idle "Send a message".
	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"

	base.Assess(thinkingFrame, p)
	det.Assess(thinkingFrame, p)

	// Same frame again: no new content, so DefaultDetector reads BusyButStagnant.
	baseState, baseConf := base.Assess(thinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(thinkingFrame, p)

	if ollamaState != LivenessConverging {
		t.Errorf("OllamaDetector on Thinking... frame: got %v, want LivenessConverging (DefaultDetector got %v)", ollamaState, baseState)
	}
	if ollamaConf <= baseConf {
		t.Errorf("OllamaDetector confidence %v not > DefaultDetector %v on Thinking... frame; uplift required", ollamaConf, baseConf)
	}
}

func TestOllamaDetector_StaticFallsBack(t *testing.T) {
	p := Profiles["ollama"]
	base := NewDefaultDetector(3)
	det := NewOllamaDetector(3)

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

func TestOllamaDetector_Malformed(t *testing.T) {
	p := Profiles["ollama"]
	edgeCases := []struct {
		name  string
		frame string
	}{
		{"empty", ""},
		{"whitespace-only", "   \n  \n"},
		{"partial-thinking", "Thinking\n"},
		{"thinking-as-suffix", "DeepThinking...\n"},
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
