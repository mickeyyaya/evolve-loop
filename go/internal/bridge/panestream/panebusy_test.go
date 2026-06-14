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
//   - codex: busy via the interrupt affordance too. codex 0.139 renders
//     "• Working (<dur> • esc to interrupt)" during a turn, which busyAffordanceRE
//     matches. (The prior fixture was the IDLE boot frame, which made codex look
//     like a weak-signal degradation; it is not — every CLI now has a positive
//     busy signal from a real generating frame, so correlation brackets all four.)
func TestPaneBusy_RealFrames(t *testing.T) {
	// genFrame is each CLI's GENERATING frame (busy=true). For claude/agy/ollama
	// that is thinking.txt; for codex it is generating.txt — codex's thinking.txt
	// is the IDLE boot frame (kept as the PaneDelta baseline), so a separate real
	// generating frame is captured for the busy contract.
	cases := []struct {
		cli, genFrame string
	}{
		{"claude", "thinking.txt"},
		{"agy", "thinking.txt"},
		{"ollama", "thinking.txt"},
		{"codex", "generating.txt"},
	}
	for _, tc := range cases {
		p := Profiles[tc.cli]
		if got := PaneBusy(readFrame(t, tc.cli+"/"+tc.genFrame), p); !got {
			t.Errorf("%s %s: PaneBusy=false, want true (generating frame must read busy)", tc.cli, tc.genFrame)
		}
		if got := PaneBusy(readFrame(t, tc.cli+"/answer.txt"), p); got {
			t.Errorf("%s answer: PaneBusy=true, want false", tc.cli)
		}
		if got := PaneBusy(readFrame(t, tc.cli+"/final.txt"), p); got {
			t.Errorf("%s final: PaneBusy=true, want false (idle)", tc.cli)
		}
	}
	// codex's idle boot frame must read NOT busy (it's the PaneDelta baseline).
	if PaneBusy(readFrame(t, "codex/thinking.txt"), Profiles["codex"]) {
		t.Error("codex/thinking.txt (idle boot frame) must read NOT busy")
	}
}

// TestPaneBusy_Codex0_139_Working — codex was previously treated as a busy
// "weak-signal degradation" because its fixture was the IDLE boot frame. A real
// codex 0.139 generating frame renders "• Working (<dur> • esc to interrupt)" —
// captured live (cycle-336 mutation-gate). Reading it as idle is the same wound
// as the claude case: a quiet-thinking codex builder/auditor → stall verdict →
// pause → exit=81, AND on the drive side the escalate-while-busy guard can't
// protect codex so a transient banner benches the whole family (the rate_limit
// false-positive). This pins codex's positive busy signal.
func TestPaneBusy_Codex0_139_Working(t *testing.T) {
	frame := readFrame(t, "codex/generating.txt")
	if !PaneBusy(frame, Profiles["codex"]) {
		t.Fatal("codex 0.139 generating frame (Working … esc to interrupt) must read busy")
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
