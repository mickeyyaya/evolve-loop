package panestream

import "testing"

// TestPaneBusy_RealFrames validates the busy/idle classifier against the real
// captured frames for every CLI. The driver's correlation bracket keys
// idle_reached on this (the input-box marker persists during generation for
// claude/agy, so marker-presence cannot be the signal — see panedelta.go).
//
//   - claude/agy: busy via the interrupt/spinner affordance.
//   - ollama: busy via the absent idle placeholder ("Send a message"); its
//     "Thinking…" header persists into the answer frame so it is NOT a signal.
//   - codex: the captured frames carry NO busy affordance and no placeholder, so
//     codex reads not-busy in BOTH frames — a documented weak-signal degradation
//     (correlation cannot bracket codex; monitoring is unaffected).
func TestPaneBusy_RealFrames(t *testing.T) {
	cases := []struct {
		cli                   string
		wantThinking, wantAns bool
	}{
		{"claude", true, false},
		{"agy", true, false},
		{"ollama", true, false},
		{"codex", false, false}, // documented weak signal: no busy detection
	}
	for _, tc := range cases {
		p := Profiles[tc.cli]
		if got := PaneBusy(readFrame(t, tc.cli+"/thinking.txt"), p); got != tc.wantThinking {
			t.Errorf("%s thinking: PaneBusy=%v, want %v", tc.cli, got, tc.wantThinking)
		}
		if got := PaneBusy(readFrame(t, tc.cli+"/answer.txt"), p); got != tc.wantAns {
			t.Errorf("%s answer: PaneBusy=%v, want %v", tc.cli, got, tc.wantAns)
		}
		if got := PaneBusy(readFrame(t, tc.cli+"/final.txt"), p); got != false {
			t.Errorf("%s final: PaneBusy=%v, want false (idle)", tc.cli, got)
		}
	}
}
