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
	if err := os.WriteFile(filepath.Join(ws, ".bridge-inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCursor(ws, "build")
	if _, err := c.Drain(); err == nil {
		t.Fatal("want a non-ErrNotExist stat error, got nil")
	}
}

func TestDrainOpenErrorPropagates(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: chmod 0000 does not deny open")
	}
	ws := t.TempDir()
	p := Path(ws, "build")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{\"kind\":\"command\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	c := NewCursor(ws, "build")
	if _, err := c.Drain(); err == nil {
		t.Fatal("want an open permission error, got nil")
	}
}

func TestAppendOpenFileErrorPropagates(t *testing.T) {
	ws := t.TempDir()
	p := Path(ws, "build")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Append(ws, "build", Envelope{Kind: KindCommand, Body: "x"}, fixedNow)
	if err == nil || !strings.Contains(err.Error(), "open") {
		t.Fatalf("want wrapped open error when inbox path is a dir, got %v", err)
	}
}
