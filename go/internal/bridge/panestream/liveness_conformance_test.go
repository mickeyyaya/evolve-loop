package panestream

import "testing"

// Names every detector type for apicover and pins LivenessProbe conformance at compile time.
var (
	_ LivenessProbe = (*DefaultDetector)(nil)
	_ LivenessProbe = (*ClaudeDetector)(nil)
	_ LivenessProbe = (*OllamaDetector)(nil)
	_ LivenessProbe = (*AgyDetector)(nil)
)

func TestLivenessProbe_StrategyConformance(t *testing.T) {
	cases := []struct {
		cli   string
		probe LivenessProbe
	}{
		{"codex", NewDefaultDetector(3)},
		{"claude", NewClaudeDetector(3)},
		{"ollama", NewOllamaDetector(3)},
		{"agy", NewAgyDetector(3)},
	}
	for _, c := range cases {
		t.Run(c.cli, func(t *testing.T) {
			p := Profiles[c.cli]
			c.probe.Assess(testdataFrame(t, c.cli+"/thinking.txt"), p) // prime
			state, conf := c.probe.Assess(testdataFrame(t, c.cli+"/answer.txt"), p)
			if state != LivenessConverging {
				t.Errorf("[%s] thinking→answer through LivenessProbe: got %v, want LivenessConverging", c.cli, state)
			}
			if conf < 0 || conf > 1 {
				t.Errorf("[%s] confidence %v out of [0,1]", c.cli, conf)
			}
		})
	}
}
