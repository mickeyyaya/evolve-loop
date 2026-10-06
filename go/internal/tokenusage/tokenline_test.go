package tokenusage

import (
	"strings"
	"testing"
)

const agyTokenLineRegex = `(?m)^\s*▸\s*Thought for [^,\n]*,\s*(?P<count>[0-9][0-9,]*(?:\.[0-9]+)?)(?P<scale>k?)\s+tokens`

const agyFinalScrollback = `> Think carefully about the task.
▸ Thought for 14s, 1.5k tokens
  Analyzing specific cases for small values of n.
● Edit(/tmp/work/ways.txt) (ctrl+o to expand)
▸ Thought for 3s, 412 tokens
  DONE
────
>
────
? for shortcuts                       Gemini 3.8 Flash · high
`

func TestDefaultResolver_AgyThoughtLineIsMeasuredNotUncovered(t *testing.T) {
	res, err := DefaultResolver(t.TempDir())(Window{Driver: "agy-tmux", Scrollback: agyFinalScrollback, TokenLineRegex: agyTokenLineRegex})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceScrollbackPeak || res.Warn != "" {
		t.Fatalf("agy pane with thought lines: source=%q warn=%q, want scrollback_peak and no uncovered warning", res.Source, res.Warn)
	}
	if res.Usage.Output != 1500 {
		t.Errorf("Output = %d, want the 1.5k peak (1500)", res.Usage.Output)
	}
}

func TestDefaultResolver_TokenLinePatternIsDriverDataNotADriverName(t *testing.T) {
	res, _ := DefaultResolver(t.TempDir())(Window{Driver: "agy-tmux", Scrollback: agyFinalScrollback})
	if res.Source != SourceNone || !strings.Contains(res.Warn, "uncovered") {
		t.Errorf("without a declared token-line pattern the agy pane stays unmeasured: source=%q warn=%q", res.Source, res.Warn)
	}
}

func TestTokenLinePeakCollector_ParsesCommasAndScaleAndIgnoresUnanchoredText(t *testing.T) {
	cases := []struct {
		pane string
		want int
	}{
		{"▸ Thought for 12s, 1,234 tokens", 1234},
		{"  ▸ Thought for 1m 2s, 2.3k tokens", 2300},
		{"▸ Thought for 2s, 264 tokens\n▸ Thought for 9s, 88 tokens", 264},
		{"quoted: ▸ Thought for 2s, 9k tokens", 0},
		{"▸ Thought for 1s", 0},
		{"(7s · ↓ 347 tokens · thought for 3s)", 0},
	}
	for _, c := range cases {
		got := TokenLinePeakCollector(c.pane, agyTokenLineRegex)()
		if got.Usage.Output != c.want {
			t.Errorf("%q: output %d, want %d", c.pane, got.Usage.Output, c.want)
		}
		if (c.want == 0) != (got.Source == SourceNone) {
			t.Errorf("%q: source %q does not match the measured count", c.pane, got.Source)
		}
	}
	if got := TokenLinePeakCollector(agyFinalScrollback, "")(); got.Source != SourceNone {
		t.Errorf("an empty pattern must measure nothing, got %+v", got)
	}
}
