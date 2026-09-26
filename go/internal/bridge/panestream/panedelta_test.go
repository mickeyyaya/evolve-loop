package panestream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFrame loads a real capture copied from the tmux live-capture research notes in docs/research.
func readFrame(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

var claudeProfile = Profiles["claude"]

func TestPaneDelta_RealCaptureSequence(t *testing.T) {
	d := &PaneDelta{}
	thinking := readFrame(t, "frame-1-thinking.txt")
	answer := readFrame(t, "frame-2-answer.txt")
	stable := readFrame(t, "frame-3-stable.txt")

	if primed := d.Next(thinking, claudeProfile); primed != nil {
		t.Fatalf("priming call returned non-nil: %v", primed)
	}

	emitted := d.Next(answer, claudeProfile)
	got := strings.Join(emitted, "\n")
	t.Logf("emitted answer lines:\n%s", got)

	for _, want := range []string{"Terminal multiplexer", "Session persistence", "Split panes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("answer bullet %q not emitted:\n%s", want, got)
		}
	}
	for _, vol := range []string{"esc to interrupt", "bypass permissions", "⏵⏵"} {
		if strings.Contains(got, vol) {
			t.Fatalf("volatile UI leaked: %q in\n%s", vol, got)
		}
	}
	if strings.Contains(got, "List 3 short bullet points") {
		t.Fatalf("submitted-prompt echo leaked as content:\n%s", got)
	}

	if more := d.Next(stable, claudeProfile); len(more) != 0 {
		t.Fatalf("re-feeding stable frame emitted %d new lines: %v", len(more), more)
	}
}

func TestPaneDelta_AllCLIs(t *testing.T) {
	cases := []struct {
		cli         string
		profile     PaneProfile
		wantAnswer  []string
		mustNotLeak []string
	}{
		{
			cli:         "claude",
			profile:     Profiles["claude"],
			wantAnswer:  []string{"Terminal multiplexer", "Session persistence", "Split panes"},
			mustNotLeak: []string{"esc to interrupt", "bypass permissions", "❯"},
		},
		{
			cli:         "codex",
			profile:     Profiles["codex"],
			wantAnswer:  []string{"terminal multiplexer"},
			mustNotLeak: []string{"gpt-5.5 medium", "›  ", "Summarize recent"},
		},
		{
			cli:         "agy",
			profile:     Profiles["agy"],
			wantAnswer:  []string{"Terminal Multiplexer", "Session Persistence"},
			mustNotLeak: []string{"? for shortcuts", "────"},
		},
		{
			cli:         "ollama",
			profile:     Profiles["ollama"],
			wantAnswer:  []string{"terminal multiplexer"},
			mustNotLeak: []string{">>> Send a message"},
		},
	}

	for _, c := range cases {
		t.Run(c.cli, func(t *testing.T) {
			d := &PaneDelta{}
			thinking := readFrame(t, filepath.Join(c.cli, "thinking.txt"))
			answer := readFrame(t, filepath.Join(c.cli, "answer.txt"))
			final := readFrame(t, filepath.Join(c.cli, "final.txt"))

			if primed := d.Next(thinking, c.profile); primed != nil {
				t.Fatalf("[%s] priming call returned non-nil: %v", c.cli, primed)
			}

			emitted := d.Next(answer, c.profile)
			got := strings.Join(emitted, "\n")
			t.Logf("[%s] emitted answer lines:\n%s", c.cli, got)

			lowGot := strings.ToLower(got)
			for _, want := range c.wantAnswer {
				if !strings.Contains(lowGot, strings.ToLower(want)) {
					t.Fatalf("[%s] answer content %q not emitted:\n%s", c.cli, want, got)
				}
			}
			for _, leak := range c.mustNotLeak {
				if strings.Contains(got, leak) {
					t.Fatalf("[%s] volatile/UI leaked: %q in\n%s", c.cli, leak, got)
				}
			}

			if more := d.Next(final, c.profile); len(more) != 0 {
				t.Fatalf("[%s] re-feeding final frame emitted %d new lines: %v", c.cli, len(more), more)
			}
		})
	}
}

func TestPaneDelta_NoMarkerFallback(t *testing.T) {
	d := &PaneDelta{}
	// No boundary marker, so the last volatileFallbackRows rows are volatile.
	frame := "line1\nline2\nline3\nfooter1\nfooter2\nfooter3\n"
	if primed := d.Next(frame, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	grown := "line1\nline2\nline3\nNEWLINE\nfooter1\nfooter2\nfooter3\n"
	got := d.Next(grown, claudeProfile)
	if len(got) != 1 || got[0] != "NEWLINE" {
		t.Fatalf("no-marker fallback emitted %v, want [NEWLINE]", got)
	}
}

func TestPaneDelta_ScrollShrinkReanchors(t *testing.T) {
	d := &PaneDelta{}
	tall := "a\nb\nc\nd\ne\n❯ \n"
	if primed := d.Next(tall, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	short := "d\ne\n❯ \n"
	if got := d.Next(short, claudeProfile); got != nil {
		t.Fatalf("scroll-shrink emitted %v, want nil (re-anchor)", got)
	}
	grown := "d\ne\nf\n❯ \n"
	if got := d.Next(grown, claudeProfile); len(got) != 1 || got[0] != "f" {
		t.Fatalf("post-reanchor emitted %v, want [f]", got)
	}
}

func TestPaneDelta_IncrementalTwoLineGrowth(t *testing.T) {
	d := &PaneDelta{}
	base := "header\n❯ \n"
	if primed := d.Next(base, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	grown := "header\n⏺ first\n  second\n❯ \n"
	got := d.Next(grown, claudeProfile)
	if len(got) != 2 || got[0] != "⏺ first" || got[1] != "  second" {
		t.Fatalf("two-line growth emitted %v, want [⏺ first, second]", got)
	}
	if more := d.Next(grown, claudeProfile); len(more) != 0 {
		t.Fatalf("no-growth re-feed emitted %d lines: %v", len(more), more)
	}
}

func TestPaneDelta_EmptyAndBlankOnlySnapshot(t *testing.T) {
	d := &PaneDelta{}
	if primed := d.Next("\n\n\n", claudeProfile); primed != nil {
		t.Fatalf("prime on blank returned %v, want nil", primed)
	}
	got := d.Next("only\n❯ \n", claudeProfile)
	if len(got) != 1 || got[0] != "only" {
		t.Fatalf("blank-prime then content emitted %v, want [only]", got)
	}
}

func TestPaneDelta_TopShiftEmitsNothing(t *testing.T) {
	d := &PaneDelta{}
	if primed := d.Next("header\n❯ \n", claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	got := d.Next("header\n⏺ one\n  two\n❯ \n", claudeProfile)
	if len(got) != 2 {
		t.Fatalf("answer emit = %v, want 2 lines", got)
	}
	shifted := "PRELUDE-A\nPRELUDE-B\nheader\n⏺ one\n  two\n❯ \n"
	if more := d.Next(shifted, claudeProfile); len(more) != 0 {
		t.Fatalf("top-shift emitted %d new lines: %v (want 0)", len(more), more)
	}
	grown := "PRELUDE-A\nPRELUDE-B\nheader\n⏺ one\n  two\n  three\n❯ \n"
	if g := d.Next(grown, claudeProfile); len(g) != 1 || g[0] != "  three" {
		t.Fatalf("post-shift growth emitted %v, want [  three]", g)
	}
}

func TestPaneDelta_AnchorScrolledOutFallsBackPositional(t *testing.T) {
	d := &PaneDelta{}
	if primed := d.Next("header\n❯ \n", claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	if got := d.Next("header\n⏺ one\n  two\n❯ \n", claudeProfile); len(got) != 2 {
		t.Fatalf("answer emit = %v, want 2", got)
	}
	scrolled := "X1\nX2\nX3\nX4\nX5\n❯ \n"
	got := d.Next(scrolled, claudeProfile)
	if len(got) == 0 {
		t.Fatalf("anchor-gone fallback emitted nothing, want positional tail")
	}
	if more := d.Next(scrolled, claudeProfile); len(more) != 0 {
		t.Fatalf("re-feed after re-anchor emitted %v, want nil", more)
	}
}

func TestPaneDelta_AnchorGoneShrinkReanchors(t *testing.T) {
	d := &PaneDelta{}
	tall := "a\nb\nc\nd\ne\n❯ \n"
	if primed := d.Next(tall, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	if got := d.Next("a\nb\nc\nd\ne\nf\n❯ \n", claudeProfile); len(got) != 1 || got[0] != "f" {
		t.Fatalf("growth emit = %v, want [f]", got)
	}
	short := "p\nq\n❯ \n"
	if got := d.Next(short, claudeProfile); got != nil {
		t.Fatalf("anchor-gone shrink emitted %v, want nil (re-anchor)", got)
	}
	if got := d.Next("p\nq\nr\n❯ \n", claudeProfile); len(got) != 1 || got[0] != "r" {
		t.Fatalf("post-reanchor emit = %v, want [r]", got)
	}
}

func TestIsVolatileTailRow_ContentRowKept(t *testing.T) {
	if isVolatileTailRow("  - a real bullet of content") {
		t.Fatal("content row wrongly classified volatile")
	}
	if isVolatileTailRow("────x────") {
		t.Fatal("non-pure-rule row wrongly classified as separator")
	}
}

func TestStableLines_NoMarkerFewerThanFallbackRows(t *testing.T) {
	got := stableLines("Send a message\n? for shortcuts\n", Profiles["claude"])
	if len(got) != 0 {
		t.Fatalf("stableLines = %v, want empty (cut clamped to 0)", got)
	}
}

func TestIsVolatileTailRow_StatusFragmentNoLeader(t *testing.T) {
	if !isVolatileTailRow("  ⏵⏵ bypass permissions on (shift+tab) · esc to interrupt") {
		t.Fatal("footer with 'esc to interrupt' not classified volatile via statusRE")
	}
}

func TestPaneDelta_Agy_BlockquoteNotBoundary(t *testing.T) {
	frame := strings.Join([]string{
		"> what is tmux?",
		"▸ Thought for 1s",
		"• tmux is a multiplexer.",
		"> A wise user once said this is quoted wisdom.", // blockquote IN content
		"• Final point about panes.",
		"────────────────────",
		">", // the real empty input box
		"? for shortcuts",
	}, "\n")

	d := &PaneDelta{}
	// Prime on a small frame so the subsequent full frame produces new content.
	_ = d.Next("> what is tmux?\n▸ Thinking...", Profiles["agy"])
	got := strings.Join(d.Next(frame, Profiles["agy"]), "\n")

	if !strings.Contains(got, "quoted wisdom") {
		t.Fatalf("blockquote line was truncated from content:\n%s", got)
	}
	if !strings.Contains(got, "Final point about panes") {
		t.Fatalf("content after blockquote was truncated:\n%s", got)
	}
	if strings.Contains(got, "? for shortcuts") {
		t.Fatalf("footer leaked into content:\n%s", got)
	}
}

func TestProfiles_BoundaryMarkers(t *testing.T) {
	type wantProfile struct {
		marker string
		exact  bool
	}
	want := map[string]wantProfile{
		"claude": {"❯", false},
		"codex":  {"›", false},
		"agy":    {">", true},
		"ollama": {">>>", false},
	}
	for cli, wp := range want {
		p, ok := Profiles[cli]
		if !ok {
			t.Fatalf("missing profile for %q", cli)
		}
		if p.BoundaryMarker != wp.marker {
			t.Fatalf("[%s] BoundaryMarker = %q, want %q", cli, p.BoundaryMarker, wp.marker)
		}
		if p.BoundaryExact != wp.exact {
			t.Fatalf("[%s] BoundaryExact = %v, want %v", cli, p.BoundaryExact, wp.exact)
		}
		if p.Name != cli {
			t.Fatalf("[%s] Name = %q, want %q", cli, p.Name, cli)
		}
	}
}

func TestPaneHasSubstantiveChange(t *testing.T) {
	const real = "❯ starting task\nTool: Read main.go\nplanning the change\n"
	cases := []struct {
		name      string
		prev, cur string
		want      bool
	}{
		{"identical", real, real, false},
		{
			"spinner frame advance only",
			real + "⠋ Deliberating… 1m 2s · ↓ 1.2k tokens\n",
			real + "⠙ Deliberating… 1m 2s · ↓ 1.2k tokens\n",
			false,
		},
		{
			"elapsed time only",
			real + "Deliberating… 1m 2s\n",
			real + "Deliberating… 1m 9s\n",
			false,
		},
		{
			"token counter only",
			real + "↓ 1.2k tokens\n",
			real + "↓ 4.7k tokens\n",
			false,
		},
		{
			"new real output line",
			real,
			real + "Tool: Write out.go\n",
			true,
		},
		{
			"changed real content",
			"❯ starting task\nTool: Read main.go\n",
			"❯ starting task\nTool: Read other.go\n",
			true,
		},
		{
			"mixed spinner advance plus new real output",
			real + "⠋ Deliberating… 1m 2s · ↓ 1.2k tokens\n",
			real + "⠙ Deliberating… 1m 5s · ↓ 1.4k tokens\nTool: Write out.go\n",
			true,
		},
		{"both empty", "", "", false},
		{"empty to real output", "", real, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PaneHasSubstantiveChange(c.prev, c.cur); got != c.want {
				t.Fatalf("PaneHasSubstantiveChange() = %v, want %v\nprev=%q\ncur=%q", got, c.want, c.prev, c.cur)
			}
		})
	}
}
