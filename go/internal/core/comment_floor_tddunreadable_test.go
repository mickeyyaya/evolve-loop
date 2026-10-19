package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTddCommentFloorFailures_UnreadablePredicateFileWarnsAndReturnsNothing(t *testing.T) {
	t.Parallel()

	repo, _ := floorRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "go", "acs", "cycle1971", "dir_test.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	got := TddCommentFloorFailures(context.Background(), repo.Dir, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), []string{"go/acs/cycle1971/dir_test.go"}, &warn)
	if len(got) != 0 {
		t.Fatalf("got %v, want nil for an unreadable file", got)
	}
	if !strings.Contains(warn.String(), "[tdd-floor] WARN") {
		t.Fatalf("warn = %q, want the tdd-floor warning", warn.String())
	}
}
