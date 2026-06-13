package panestream

import "testing"

// TestClassifyLine_Layers pins the channel each representative pane line belongs
// to. The corpus seed is the REAL cycle-312 claude frame (`· Schlepping… (50s ·
// ↑ 3.1k tokens)`) that killed two soak cycles: it is the live-turn affordance —
// busy, but NOT progress.
func TestClassifyLine_Layers(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Layer
	}{
		// Content — agent transcript.
		{"tool call", "⏺ Bash(cd repo && go test ./...)", LayerContent},
		{"command output", "  ⎿  ===CYCLE-283 LESSON===", LayerContent},
		{"markdown bullet (NOT an ascii spinner)", "- Session persistence: tmux keeps state", LayerContent},
		{"prose", "Investigating the failure pile to pick the next task.", LayerContent},
		// Affordance — proves the turn is live.
		{"cycle-312 schlepping frame (corpus seed)", "· Schlepping… (50s · ↑ 3.1k tokens)", LayerAffordance},
		{"esc to interrupt", "  ⏵⏵ bypass permissions · esc to interrupt", LayerAffordance},
		{"down-arrow spinner stats", "✻ Coalescing… (7s · ↓ 347 tokens)", LayerAffordance},
		// Chrome — volatile, no liveness meaning.
		{"blank", "   ", LayerChrome},
		{"box-drawing separator", "──────────────────────", LayerChrome},
		{"braille spinner alone", "⠋", LayerChrome},
		{"ascii spinner alone", "- ", LayerChrome},
		{"deliberating clock", "Deliberating… 1m 2s", LayerChrome},
		{"bare token counter", "↑ 3.1k tokens", LayerChrome},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyLine(c.line); got != c.want {
				t.Fatalf("ClassifyLine(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}

// TestClassifyLine_DetectorsAgree is the load-bearing anti-divergence property
// (ADR-0047): the SAME spinner-stats line must read consistently across all
// three pane detectors — busy (liveness), NOT content (progress), and volatile
// (trim). Before the single classifier, `· Schlepping…` was busy to PaneBusy but
// content to cleanPane, so a working agent's ticking clock counted as progress
// and a stalled one's didn't — the exact bridge-busy / bench-evidence bug class.
func TestClassifyLine_DetectorsAgree(t *testing.T) {
	const schlepping = "· Schlepping… (50s · ↑ 3.1k tokens)"

	if ClassifyLine(schlepping) != LayerAffordance {
		t.Fatalf("schlepping must be Affordance")
	}
	if IsContentLine(schlepping) {
		t.Error("progress (cleanPane) must NOT count the spinner-stats line as content")
	}
	if !IsAffordanceLine(schlepping) {
		t.Error("liveness (PaneBusy) must read the spinner-stats line as the busy affordance")
	}
	if !isVolatileTailRow(schlepping) {
		t.Error("trim (PaneDelta) must treat the spinner-stats line as volatile")
	}
	if !PaneBusy(schlepping+"\n❯ \n", Profiles["claude"]) {
		t.Error("PaneBusy must read a pane showing the schlepping frame as busy")
	}

	// A real content line is the mirror: content, not affordance, not volatile.
	const content = "⏺ Bash(go test ./internal/bridge/...)"
	if !IsContentLine(content) {
		t.Error("a tool-call line must be content")
	}
	if isVolatileTailRow(content) {
		t.Error("a tool-call line must NOT be trimmed as volatile")
	}
}
