package inbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKindValid(t *testing.T) {
	for _, k := range []Kind{KindCommand, KindInterrupt, KindNudge, KindSystemRule} {
		if !k.Valid() {
			t.Errorf("Kind(%q).Valid() = false, want true", k)
		}
	}
	for _, k := range []Kind{"", "bogus", "Command", "system-rule"} {
		if Kind(k).Valid() {
			t.Errorf("Kind(%q).Valid() = true, want false", k)
		}
	}
}

func TestCursorOffsetReflectsBytesConsumed(t *testing.T) {
	ws := t.TempDir()
	c := NewCursor(ws, "build")
	if c.Offset() != 0 {
		t.Fatalf("fresh cursor Offset() = %d, want 0", c.Offset())
	}
	mustAppend(t, ws, "one")
	if _, err := c.Drain(); err != nil {
		t.Fatalf("drain: %v", err)
	}
	fi, err := os.Stat(Path(ws, "build"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Offset() != fi.Size() {
		t.Errorf("Offset() = %d after draining all lines, want file size %d", c.Offset(), fi.Size())
	}
}

func TestAppendMkdirError(t *testing.T) {
	// A regular file where Append needs to MkdirAll <file>/.bridge-inbox.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Append(blocker, "build", Envelope{Kind: KindCommand, Body: "x"}, fixedNow)
	if err == nil || !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("want mkdir error when workspace is a file, got %v", err)
	}
}

func TestDrainStatErrorPropagates(t *testing.T) {
	ws := t.TempDir()
	// Make .bridge-inbox a FILE so Stat(<ws>/.bridge-inbox/build.ndjson)
	// returns ENOTDIR — a non-ErrNotExist error Drain must surface, not swallow.
	if err := os.WriteFile(filepath.Join(ws, ".bridge-inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCursor(ws, "build")
	if _, err := c.Drain(); err == nil {
		t.Fatal("want a non-ErrNotExist stat error, got nil")
	}
}
