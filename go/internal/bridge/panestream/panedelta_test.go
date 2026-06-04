package panestream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFrame loads a committed real capture-pane snapshot from testdata/. The
// frames are local copies of the source-of-truth captures under
// knowledge-base/research/tmux-live-capture-2026-06-04/frames/ so the test path
// is stable and does not depend on a fragile relative walk to the repo root.
func readFrame(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// TestPaneDelta_RealCaptureSequence is the deliverable's point: drive the
// extractor with three REAL capture-pane frames from a live claude tmux session
// and assert the answer flows through while the volatile UI never leaks.
func TestPaneDelta_RealCaptureSequence(t *testing.T) {
	d := &PaneDelta{}
	thinking := readFrame(t, "frame-1-thinking.txt")
	answer := readFrame(t, "frame-2-answer.txt")
	stable := readFrame(t, "frame-3-stable.txt")

	// Prime on the mid-thinking frame: returns nil (no new content yet, and the
	// boot chrome + echoed submitted prompt are absorbed into the baseline).
	if primed := d.Next(thinking, "❯"); primed != nil {
		t.Fatalf("priming call returned non-nil: %v", primed)
	}

	emitted := d.Next(answer, "❯")
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
	if more := d.Next(stable, "❯"); len(more) != 0 {
		t.Fatalf("re-feeding stable frame emitted %d new lines: %v", len(more), more)
	}
}

func TestPaneDelta_NoMarkerFallback(t *testing.T) {
	d := &PaneDelta{}
	// 5 content rows, no prompt marker → last 3 rows treated as volatile.
	frame := "line1\nline2\nline3\nfooter1\nfooter2\nfooter3\n"
	if primed := d.Next(frame, "❯"); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Grow by one stable line above the (still 3-row) volatile tail.
	grown := "line1\nline2\nline3\nNEWLINE\nfooter1\nfooter2\nfooter3\n"
	got := d.Next(grown, "❯")
	if len(got) != 1 || got[0] != "NEWLINE" {
		t.Fatalf("no-marker fallback emitted %v, want [NEWLINE]", got)
	}
}

func TestPaneDelta_ScrollShrinkReanchors(t *testing.T) {
	d := &PaneDelta{}
	// Prime with a tall stable region (5 lines above the empty input box).
	tall := "a\nb\nc\nd\ne\n❯ \n"
	if primed := d.Next(tall, "❯"); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Pane scrolled: stable region shrank to 2 lines (emitted=5 > len=2).
	short := "d\ne\n❯ \n"
	if got := d.Next(short, "❯"); got != nil {
		t.Fatalf("scroll-shrink emitted %v, want nil (re-anchor)", got)
	}
	// After re-anchor (emitted=2), one new stable line emits exactly once.
	grown := "d\ne\nf\n❯ \n"
	if got := d.Next(grown, "❯"); len(got) != 1 || got[0] != "f" {
		t.Fatalf("post-reanchor emitted %v, want [f]", got)
	}
}

func TestPaneDelta_IncrementalTwoLineGrowth(t *testing.T) {
	d := &PaneDelta{}
	base := "header\n❯ \n"
	if primed := d.Next(base, "❯"); primed != nil {
		t.Fatalf("prime returned %v, want nil", primed)
	}
	// Two content lines appear above the empty input box.
	grown := "header\n⏺ first\n  second\n❯ \n"
	got := d.Next(grown, "❯")
	if len(got) != 2 || got[0] != "⏺ first" || got[1] != "  second" {
		t.Fatalf("two-line growth emitted %v, want [⏺ first, second]", got)
	}
	// No further growth → nothing emitted.
	if more := d.Next(grown, "❯"); len(more) != 0 {
		t.Fatalf("no-growth re-feed emitted %d lines: %v", len(more), more)
	}
}

func TestPaneDelta_EmptyAndBlankOnlySnapshot(t *testing.T) {
	d := &PaneDelta{}
	// Blank-only snapshot → stableLines returns nil; prime sets emitted=0.
	if primed := d.Next("\n\n\n", "❯"); primed != nil {
		t.Fatalf("prime on blank returned %v, want nil", primed)
	}
	// A subsequent real frame then emits its stable content.
	got := d.Next("only\n❯ \n", "❯")
	if len(got) != 1 || got[0] != "only" {
		t.Fatalf("blank-prime then content emitted %v, want [only]", got)
	}
}
