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

// TestPaneBusy_Claude2_1_173_SpinnerOnly — the 2026-06-11 soak-killer: the
// claude 2.1.173 self-update REMOVED the "esc to interrupt" affordance from
// generating panes; the only busy chrome left is the spinner stats line
// ("✢ Kneading… (6s · ↓ 244 tokens · thinking with high effort)"). Reading
// that frame as idle re-opens the cycles-254/255 wound (8ce42d6d): quiet
// extended thinking → stall verdict → pause → exit=81 (cycles 286/288).
// Fixture captured live from claude 2.1.173.
func TestPaneBusy_Claude2_1_173_SpinnerOnly(t *testing.T) {
	frame := readFrame(t, "claude/thinking-v2.1.173.txt")
	if !PaneBusy(frame, Profiles["claude"]) {
		t.Fatal("claude 2.1.173 generating frame (spinner stats line, no esc affordance) must read busy")
	}
}

// TestPaneBusy_SpinnerStatsVariants pins the spinner stats line across the
// duration formats a turn passes through — seconds, minutes, and the
// hour-plus shapes a long agent turn reaches (a miss there re-opens the
// stall wound exactly for the longest, most expensive turns).
func TestPaneBusy_SpinnerStatsVariants(t *testing.T) {
	busyLines := []string{
		"✻ Bloviating… (4s · ↓ 50 tokens · thinking with high effort)",
		"✢ Kneading… (6s · ↓ 244 tokens · thinking with high effort)",
		"✻ Coalescing… (7s · ↓ 347 tokens · thought for 3s)",
		"· Evaporating… (44s · ↑ 3.1k tokens)",
		"✻ Synthesizing… (12m 34s · ↑ 40.2k tokens)",
		"✻ Persevering… (1h 5m · ↑ 100k tokens)",
	}
	for _, line := range busyLines {
		if !PaneBusy(line+"\n❯ \n", Profiles["claude"]) {
			t.Errorf("spinner variant must read busy: %q", line)
		}
	}
	idleLines := []string{
		"❯ \n  ⏵⏵ bypass permissions on (shift+tab to cycle)",
		"The cost line said it used 3.1k tokens overall.", // prose mentioning tokens, no structural spinner
	}
	for _, line := range idleLines {
		if PaneBusy(line, Profiles["claude"]) {
			t.Errorf("idle frame must NOT read busy: %q", line)
		}
	}
}
