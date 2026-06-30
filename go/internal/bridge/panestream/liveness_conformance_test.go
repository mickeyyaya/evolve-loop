package panestream

import "testing"

// liveness_conformance_test.go — names the LivenessProbe Strategy abstraction by
// IDENTIFIER for every per-CLI detector (cycles 423-425, ADR-0047). The prior
// behavioral tests only referenced the New* constructors, leaving the
// LivenessProbe interface and the four detector types unnamed in any test AST;
// apicover -enforce (Phase 5) flagged them as "no test names it". This test
// names them through the interface AND pins the contract they share.

// Compile-time conformance: every concrete detector must satisfy LivenessProbe,
// the single interface the stop-review reviewer depends on. If any detector's
// Assess signature drifts out of the contract, this fails to compile.
var (
	_ LivenessProbe = (*DefaultDetector)(nil)
	_ LivenessProbe = (*ClaudeDetector)(nil)
	_ LivenessProbe = (*OllamaDetector)(nil)
	_ LivenessProbe = (*AgyDetector)(nil)
)

// TestLivenessProbe_StrategyConformance asserts that, exercised ONLY through the
// LivenessProbe interface, every per-CLI strategy classifies a thinking→answer
// frame sequence (new stable content) as LivenessConverging with a confidence in
// [0,1]. This is the cross-strategy invariant the reviewer relies on: whichever
// detector the registry selects, real output growth is never read as stuck. Each
// probe is constructed by its own concrete type (so apicover sees the type name)
// but consumed behind the interface (so the test pins the contract, not the impl).
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
