package inbox

import "testing"

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
