package ship

import (
	"reflect"
	"testing"
)

// Cycle-653 second seam: ship-bind manifest reconciliation. Pure-function
// contract tests — the shadow wiring in shipFromWorktree only logs.

func TestExtractReportPaths(t *testing.T) {
	md := "# Build Report\n\n" +
		"## Changes\n" +
		"| File | Why |\n|---|---|\n" +
		"| `go/internal/core/worktree_clean.go` | new |\n" +
		"- edited go/internal/core/worktree.go.\n" +
		"```json\n{\"testFiles\": [\"go/internal/core/worktree_clean_test.go\"]}\n```\n" +
		"No paths here: standalone words, gofmt, HEAD.\n"
	got := extractReportPaths(md)
	want := []string{
		"go/internal/core/worktree.go",
		"go/internal/core/worktree_clean.go",
		"go/internal/core/worktree_clean_test.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractReportPaths = %v, want %v", got, want)
	}
}

func TestOutOfManifest_FlagsUndeclaredPaths(t *testing.T) {
	changed := []string{
		"go/internal/core/worktree_clean.go",
		"go/internal/recurrence/digest_test.go", // inherited orphan — undeclared
	}
	manifest := []string{"go/internal/core/worktree_clean.go"}
	got := outOfManifest(changed, manifest)
	want := []string{"go/internal/recurrence/digest_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outOfManifest = %v, want %v", got, want)
	}
}

func TestOutOfManifest_InManifestDiffHasNoFalsePositive(t *testing.T) {
	// Acceptance: clean worktree + in-manifest diff ships unchanged.
	changed := []string{
		"go/internal/core/worktree.go",
		"docs/adr/0071-clean-worktree.md",
	}
	manifest := []string{
		"go/internal/core/worktree.go",
		"docs/adr", // directory entry covers children by prefix
	}
	if got := outOfManifest(changed, manifest); len(got) != 0 {
		t.Fatalf("outOfManifest false positive: %v", got)
	}
	// Prefix must be component-wise: docs/adr does NOT cover docs/adr-other.md.
	if got := outOfManifest([]string{"docs/adr-other.md"}, manifest); len(got) != 1 {
		t.Fatalf("prefix match leaked across path component: %v", got)
	}
}
