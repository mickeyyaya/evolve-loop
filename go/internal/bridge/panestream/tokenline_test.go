package panestream

import "testing"

const testTokenLineRegex = `(?m)^\s*▸\s*Thought for [^,\n]*,\s*(?P<count>[0-9][0-9,]*(?:\.[0-9]+)?)(?P<scale>k?)\s+tokens`

func TestTokenLinePeak_ReadsCountsScalesAndCommas(t *testing.T) {
	cases := []struct {
		pane string
		want int
	}{
		{"▸ Thought for 14s, 1.5k tokens", 1500},
		{"▸ Thought for 12s, 1,234 tokens", 1234},
		{"▸ Thought for 2s, 264 tokens\n▸ Thought for 9s, 88 tokens", 264},
		{"\x1b[2m▸ Thought for 2s, 9 tokens\x1b[0m", 9},
		{"echo: ▸ Thought for 2s, 9k tokens", 0},
		{"(7s · ↓ 347 tokens)", 0},
	}
	for _, c := range cases {
		if got := TokenLinePeak(c.pane, testTokenLineRegex); got != c.want {
			t.Errorf("%q: got %d, want %d", c.pane, got, c.want)
		}
	}
	if TokenLinePeak("▸ Thought for 2s, 9 tokens", "") != 0 {
		t.Error("an empty pattern measures nothing")
	}
	if TokenLinePeak("▸ Thought for 2s, 9 tokens", `Thought for \S+, ([0-9]+) tokens`) != 0 {
		t.Error("a pattern without a count group measures nothing")
	}
	if TokenLinePeak("Thought 7 tokens", `Thought (?P<count>[0-9]+) tokens`) != 7 {
		t.Error("the scale group is optional")
	}
	if tokenCount("1.2.3", "") != 0 {
		t.Error("an unparseable count is zero")
	}
}

func TestLastTokenLine_ReturnsTheNewestMatchTrimmed(t *testing.T) {
	pane := testdataFrame(t, "agy-1.2.17/answer.txt")
	if got := LastTokenLine(pane, testTokenLineRegex); got != "▸ Thought for 14s, 1.5k tokens" {
		t.Errorf("LastTokenLine = %q", got)
	}
	if got := LastTokenLine("▸ Thought for 1s, 5 tokens\n▸ Thought for 2s, 6 tokens\n", testTokenLineRegex); got != "▸ Thought for 2s, 6 tokens" {
		t.Errorf("LastTokenLine = %q, want the newest line", got)
	}
	if LastTokenLine("no tokens here", testTokenLineRegex) != "" || LastTokenLine("▸ Thought for 1s, 5 tokens", "") != "" {
		t.Error("no match or no pattern gives an empty line")
	}
}

func TestAgyDetector_ANewThoughtBlockIsProgressEvenUnderAFrozenTranscript(t *testing.T) {
	p := Profiles["agy"]
	p.BusyLineRegex, p.TokenLineRegex = testBusyLineRegex, testTokenLineRegex
	thinking := "> task\n▸ Thought for 2s, 264 tokens\n⣽  Working...\n────\n>\n────\nesc to cancel"
	nextBlock := "> task\n▸ Thought for 2s, 264 tokens\n▸ Thought for 9s, 88 tokens\n⢿  Working...\n────\n>\n────\nesc to cancel"

	det := NewAgyDetector(3)
	det.Assess(thinking, p)
	if state, _ := det.Assess(thinking, p); state == LivenessConverging {
		t.Fatal("an unchanged thought line is not progress")
	}
	state, conf := det.Assess(nextBlock, p)
	if state != LivenessConverging || conf < 0.95 {
		t.Errorf("a new thought block: got (%v, %.2f), want Converging at 0.95", state, conf)
	}
	if state, _ := det.Assess(nextBlock, p); state == LivenessConverging {
		t.Error("the same thought block seen twice is not progress again")
	}
}
