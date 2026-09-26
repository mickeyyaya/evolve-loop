package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderCoveringTests_TruncatesWithVisibleNote(t *testing.T) {
	var many []string
	for i := 0; i < 5000; i++ {
		many = append(many, "go/internal/pkg/a_very_long_package_name_test.go")
	}
	out, _ := renderCoveringTests(many)

	if len(out) > coveringTestsMaxBytes+256 {
		t.Fatalf("rendered corpus is %d bytes, want <= cap %d plus the note", len(out), coveringTestsMaxBytes)
	}
	if !strings.Contains(out, "TRUNCATED:") {
		t.Fatalf("truncated corpus carries no visible note — a trimmed list is indistinguishable from a complete one:\n%s", out)
	}
}

func TestRenderCoveringTests_EmitsEveryPathWhenUnderCap(t *testing.T) {
	out, _ := renderCoveringTests([]string{"go/internal/foo/foo_test.go", "go/internal/bar/bar_test.go"})

	for _, want := range []string{"go/internal/foo/foo_test.go", "go/internal/bar/bar_test.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered corpus is missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "TRUNCATED:") {
		t.Errorf("a two-path corpus was truncated:\n%s", out)
	}
}

func TestWriteCoveringTests_NoOpWithoutWorktreeOrWorkspace(t *testing.T) {
	ws := t.TempDir()
	writeCoveringTests(context.Background(), "", ws)
	if _, err := os.Stat(filepath.Join(ws, coveringTestsArtifact)); !os.IsNotExist(err) {
		t.Fatalf("artifact written with no worktree (err=%v) — nothing was derivable", err)
	}
	// No workspace: nowhere to write; must return without touching the filesystem.
	writeCoveringTests(context.Background(), t.TempDir(), "")
}
