package panestream

import "testing"

func TestPaneBusy_RealFrames(t *testing.T) {
	// codex's thinking.txt is its idle boot frame (the PaneDelta baseline), so its
	// generating frame is generating.txt.
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
	if PaneBusy(readFrame(t, "codex/thinking.txt"), Profiles["codex"]) {
		t.Error("codex/thinking.txt (idle boot frame) must read NOT busy")
	}
}

func TestPaneBusy_Codex0_139_Working(t *testing.T) {
	frame := readFrame(t, "codex/generating.txt")
	if !PaneBusy(frame, Profiles["codex"]) {
		t.Fatal("codex 0.139 generating frame (Working … esc to interrupt) must read busy")
	}
}

func TestPaneBusy_Claude2_1_173_SpinnerOnly(t *testing.T) {
	frame := readFrame(t, "claude/thinking-v2.1.173.txt")
	if !PaneBusy(frame, Profiles["claude"]) {
		t.Fatal("claude 2.1.173 generating frame (spinner stats line, no esc affordance) must read busy")
	}
}

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
