package panestream

import "testing"

func TestClassifyLine_Layers(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Layer
	}{
		{"tool call", "⏺ Bash(cd repo && go test ./...)", LayerContent},
		{"command output", "  ⎿  ===CYCLE-283 LESSON===", LayerContent},
		{"markdown bullet (NOT an ascii spinner)", "- Session persistence: tmux keeps state", LayerContent},
		{"prose", "Investigating the failure pile to pick the next task.", LayerContent},
		{"cycle-312 schlepping frame (corpus seed)", "· Schlepping… (50s · ↑ 3.1k tokens)", LayerAffordance},
		{"esc to interrupt", "  ⏵⏵ bypass permissions · esc to interrupt", LayerAffordance},
		{"down-arrow spinner stats", "✻ Coalescing… (7s · ↓ 347 tokens)", LayerAffordance},
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

	const content = "⏺ Bash(go test ./internal/bridge/...)"
	if !IsContentLine(content) {
		t.Error("a tool-call line must be content")
	}
	if isVolatileTailRow(content) {
		t.Error("a tool-call line must NOT be trimmed as volatile")
	}
}
