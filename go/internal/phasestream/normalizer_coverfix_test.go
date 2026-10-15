package phasestream

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTailFileCoverfix_NoGrowthReturnsNothing(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, off := tailFile(p, 2)
	if lines != nil || off != 2 {
		t.Fatalf("unchanged file: got %v off %d", lines, off)
	}
}

func TestTailFileCoverfix_InvalidOffsetKeepsOffset(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, off := tailFile(p, -1)
	if lines != nil || off != -1 {
		t.Fatalf("seek failure must leave offset: got %v off %d", lines, off)
	}
}

func TestTailFileCoverfix_UnreadablePathKeepsOffset(t *testing.T) {
	t.Parallel()
	lines, off := tailFile(t.TempDir(), 0)
	if lines != nil || off != 0 {
		t.Fatalf("read failure must leave offset: got %v off %d", lines, off)
	}
}
