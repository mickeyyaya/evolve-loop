package panestream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFrame loads a committed real capture-pane snapshot from testdata/. The
// frames are local copies of the source-of-truth captures under
// knowledge-base/research/tmux-live-capture-2026-06-04/ so the test path is
// stable and does not depend on a fragile relative walk to the repo root.
func readFrame(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// claudeProfile is the profile under test in the legacy single-CLI cases.
var claudeProfile = Profiles["claude"]

// TestPaneDelta_RealCaptureSequence is the original deliverable's point: drive
// the extractor with three REAL capture-pane frames from a live claude tmux
// session and assert the answer flows through while the volatile UI never leaks.
func TestPaneDelta_RealCaptureSequence(t *testing.T) {
	d := &PaneDelta{}
	thinking := readFrame(t, "frame-1-thinking.txt")
	answer := readFrame(t, "frame-2-answer.txt")
	stable := readFrame(t, "frame-3-stable.txt")

	// Prime on the mid-thinking frame: returns nil (no new content yet, and the
	// boot chrome + echoed submitted prompt are absorbed into the baseline).
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
	// The echoed submitted-prompt line is pre-existing chrome absorbed by the
	// priming baseline — it must NOT be re-emitted as content.
	if strings.Contains(got, "List 3 short bullet points") {
		t.Fatalf("submitted-prompt echo leaked as content:\n%s", got)
	}

	// Re-feeding the stable frame (identical answer) emits nothing new.
	if more := d.Next(stable, claudeProfile); len(more) != 0 {
		t.Fatalf("re-feeding stable frame emitted %d new lines: %v", len(more), more)
	}
}

// TestPaneDelta_AllCLIs is the generalization deliverable: drive the extractor
// across the four supported tmux LLM CLIs with their REAL capture frames and
// assert (a) the answer content IS emitted, (b) the input-box / footer / spinner
// / separators are NEVER emitted, and (c) re-feeding the settled frame emits
// nothing new.
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

			// Prime on the thinking frame — emits nothing (baseline only).
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

			// Re-feeding the settled frame emits nothing new.
			if more := d.Next(final, c.profile); len(more) != 0 {
				t.Fatalf("[%s] re-feeding final frame emitted %d new lines: %v", c.cli, len(more), more)
			}
		})
	}
}

func TestPaneDelta_NoMarkerFallback(t *testing.T) {
	d := &PaneDelta{}
	// 5 content rows, no boundary marker → last 3 rows treated as volatile.
	frame := "line1\nline2\nline3\nfooter1\nfooter2\nfooter3\n"
	if primed := d.Next(frame, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Grow by one stable line above the (still 3-row) volatile tail.
	grown := "line1\nline2\nline3\nNEWLINE\nfooter1\nfooter2\nfooter3\n"
	got := d.Next(grown, claudeProfile)
	if len(got) != 1 || got[0] != "NEWLINE" {
		t.Fatalf("no-marker fallback emitted %v, want [NEWLINE]", got)
	}
}

func TestPaneDelta_ScrollShrinkReanchors(t *testing.T) {
	d := &PaneDelta{}
	// Prime with a tall stable region (5 lines above the empty input box).
	tall := "a\nb\nc\nd\ne\n❯ \n"
	if primed := d.Next(tall, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Pane scrolled: stable region shrank to 2 lines (emitted=5 > len=2).
	short := "d\ne\n❯ \n"
	if got := d.Next(short, claudeProfile); got != nil {
		t.Fatalf("scroll-shrink emitted %v, want nil (re-anchor)", got)
	}
	// After re-anchor (emitted=2), one new stable line emits exactly once.
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
	// Two content lines appear above the empty input box.
	grown := "header\n⏺ first\n  second\n❯ \n"
	got := d.Next(grown, claudeProfile)
	if len(got) != 2 || got[0] != "⏺ first" || got[1] != "  second" {
		t.Fatalf("two-line growth emitted %v, want [⏺ first, second]", got)
	}
	// No further growth → nothing emitted.
	if more := d.Next(grown, claudeProfile); len(more) != 0 {
		t.Fatalf("no-growth re-feed emitted %d lines: %v", len(more), more)
	}
}

func TestPaneDelta_EmptyAndBlankOnlySnapshot(t *testing.T) {
	d := &PaneDelta{}
	// Blank-only snapshot → stableLines returns nil; prime sets emitted=0.
	if primed := d.Next("\n\n\n", claudeProfile); primed != nil {
		t.Fatalf("prime on blank returned %v, want nil", primed)
	}
	// A subsequent real frame then emits its stable content.
	got := d.Next("only\n❯ \n", claudeProfile)
	if len(got) != 1 || got[0] != "only" {
		t.Fatalf("blank-prime then content emitted %v, want [only]", got)
	}
}

// TestPaneDelta_TopShiftEmitsNothing exercises the content-anchor path directly:
// when older scrollback / a prelude is prepended (every content line keeps its
// text but moves down), the delta must emit nothing rather than re-emit the
// bottom line. This is the real-frame `final.txt` failure mode.
func TestPaneDelta_TopShiftEmitsNothing(t *testing.T) {
	d := &PaneDelta{}
	if primed := d.Next("header\n❯ \n", claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Answer appears.
	got := d.Next("header\n⏺ one\n  two\n❯ \n", claudeProfile)
	if len(got) != 2 {
		t.Fatalf("answer emit = %v, want 2 lines", got)
	}
	// Now a prelude is prepended at the TOP; same answer content shifts down.
	shifted := "PRELUDE-A\nPRELUDE-B\nheader\n⏺ one\n  two\n❯ \n"
	if more := d.Next(shifted, claudeProfile); len(more) != 0 {
		t.Fatalf("top-shift emitted %d new lines: %v (want 0)", len(more), more)
	}
	// A genuinely new bottom line after the shift emits exactly once.
	grown := "PRELUDE-A\nPRELUDE-B\nheader\n⏺ one\n  two\n  three\n❯ \n"
	if g := d.Next(grown, claudeProfile); len(g) != 1 || g[0] != "  three" {
		t.Fatalf("post-shift growth emitted %v, want [  three]", g)
	}
}

// TestPaneDelta_AnchorScrolledOutFallsBackPositional covers the positional
// fallback when the anchored last-emitted line has scrolled entirely out of the
// pane (lastIndexOf returns -1).
func TestPaneDelta_AnchorScrolledOutFallsBackPositional(t *testing.T) {
	d := &PaneDelta{}
	if primed := d.Next("header\n❯ \n", claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Emit two lines; anchor becomes "  two".
	if got := d.Next("header\n⏺ one\n  two\n❯ \n", claudeProfile); len(got) != 2 {
		t.Fatalf("answer emit = %v, want 2", got)
	}
	// Pane scrolled so the anchor "  two" is gone; stable region is entirely new
	// content of greater length than emitted → positional fallback emits the
	// tail beyond the old emitted count.
	scrolled := "X1\nX2\nX3\nX4\nX5\n❯ \n"
	got := d.Next(scrolled, claudeProfile)
	if len(got) == 0 {
		t.Fatalf("anchor-gone fallback emitted nothing, want positional tail")
	}
	// Re-anchored on X5; re-feeding identical emits nothing.
	if more := d.Next(scrolled, claudeProfile); len(more) != 0 {
		t.Fatalf("re-feed after re-anchor emitted %v, want nil", more)
	}
}

// TestPaneDelta_AnchorGoneShrinkReanchors covers the shrink branch of the
// positional fallback: anchor not found AND the stable region shrank below the
// emitted baseline → re-anchor, emit nothing.
func TestPaneDelta_AnchorGoneShrinkReanchors(t *testing.T) {
	d := &PaneDelta{}
	tall := "a\nb\nc\nd\ne\n❯ \n"
	if primed := d.Next(tall, claudeProfile); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Grow once so emitted advances and anchor is set to a real line.
	if got := d.Next("a\nb\nc\nd\ne\nf\n❯ \n", claudeProfile); len(got) != 1 || got[0] != "f" {
		t.Fatalf("growth emit = %v, want [f]", got)
	}
	// Now the pane both lost the anchor "f" AND shrank below emitted (6 → 2).
	short := "p\nq\n❯ \n"
	if got := d.Next(short, claudeProfile); got != nil {
		t.Fatalf("anchor-gone shrink emitted %v, want nil (re-anchor)", got)
	}
	// After re-anchor on q, one new line emits once.
	if got := d.Next("p\nq\nr\n❯ \n", claudeProfile); len(got) != 1 || got[0] != "r" {
		t.Fatalf("post-reanchor emit = %v, want [r]", got)
	}
}

// TestIsVolatileTailRow_ContentRowKept pins that a real content row (not blank,
// not a separator, no status leader/fragment) is NOT trimmed.
func TestIsVolatileTailRow_ContentRowKept(t *testing.T) {
	if isVolatileTailRow("  - a real bullet of content") {
		t.Fatal("content row wrongly classified volatile")
	}
	// A mostly-separator row with a stray non-rule char is content, not a rule.
	if isVolatileTailRow("────x────") {
		t.Fatal("non-pure-rule row wrongly classified as separator")
	}
}

// TestStableLines_NoMarkerFewerThanFallbackRows covers the cut<0 guard: a
// no-marker frame shorter than volatileFallbackRows clamps the cut to 0.
func TestStableLines_NoMarkerFewerThanFallbackRows(t *testing.T) {
	// Two non-blank lines, no boundary marker, both volatile-ish footers →
	// cut = 2-3 = -1 → clamped to 0 → empty stable region.
	got := stableLines("Send a message\n? for shortcuts\n", Profiles["claude"])
	if len(got) != 0 {
		t.Fatalf("stableLines = %v, want empty (cut clamped to 0)", got)
	}
}

// TestIsVolatileTailRow_StatusFragmentNoLeader covers the statusRE branch for a
// row that matches a status fragment but does NOT start with a spinner leader.
func TestIsVolatileTailRow_StatusFragmentNoLeader(t *testing.T) {
	// Footer fragment, plain text (no ✽/✻/⣯/▸ leader).
	if !isVolatileTailRow("  ⏵⏵ bypass permissions on (shift+tab) · esc to interrupt") {
		t.Fatal("footer with 'esc to interrupt' not classified volatile via statusRE")
	}
}

// TestPaneDelta_Agy_BlockquoteNotBoundary proves that a markdown blockquote
// line ("> quoted text") inside agy answer content is NOT treated as the
// input-box boundary. Without BoundaryExact the prefix scan would pick it as
// the last boundary candidate and silently truncate everything after it.
func TestPaneDelta_Agy_BlockquoteNotBoundary(t *testing.T) {
	// Minimal agy-shaped pane: prompt echo, answer including a "> quoted" line,
	// then the empty ">" input box + footer.
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

	// The line after the blockquote must be present — if the blockquote were
	// chosen as the boundary, everything below it would be silently dropped.
	if !strings.Contains(got, "quoted wisdom") {
		t.Fatalf("blockquote line was truncated from content:\n%s", got)
	}
	if !strings.Contains(got, "Final point about panes") {
		t.Fatalf("content after blockquote was truncated:\n%s", got)
	}
	// The footer must never leak.
	if strings.Contains(got, "? for shortcuts") {
		t.Fatalf("footer leaked into content:\n%s", got)
	}
}

// TestProfiles_BoundaryMarkers pins the four boundary markers and the
// BoundaryExact flag so a profile edit that drops/typos either fails loudly.
func TestProfiles_BoundaryMarkers(t *testing.T) {
	type wantProfile struct {
		marker string
		exact  bool
	}
	want := map[string]wantProfile{
		"claude": {"❯", false},
		"codex":  {"›", false},
		"agy":    {">", true}, // plain ASCII marker — must use exact-match
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
