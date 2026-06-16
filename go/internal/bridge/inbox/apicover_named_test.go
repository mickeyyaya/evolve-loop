package inbox

import "testing"

// TestCursor_NewCursorStartsAtZeroOffset names the inbox.Cursor type (NewCursor
// returns *Cursor but the bare type is never named in a test) and pins the
// constructor's start-of-file contract: a fresh cursor is byte-offset 0 with its
// path wired to Path(workspace, agent), so the first Drain replays from the top.
func TestCursor_NewCursorStartsAtZeroOffset(t *testing.T) {
	ws := t.TempDir()
	got := NewCursor(ws, "build")
	want := Cursor{path: Path(ws, "build"), offset: 0}
	if *got != want {
		t.Fatalf("NewCursor = %+v, want %+v", *got, want)
	}
	if got.Offset() != 0 {
		t.Fatalf("Offset() = %d, want 0", got.Offset())
	}
}
