package shipmanifest

import (
	"reflect"
	"testing"
)

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

func TestExtractReportPaths_BareRootFilenames(t *testing.T) {
	md := "bumped `CHANGELOG.md`, `go.mod`, `go.sum` and edited `go/internal/foo.go`, " +
		"plus the dotfile `.goreleaser.yml`; prose like e.g. and cfg.Now and 1.0 must not match."
	got := extractReportPaths(md)
	want := []string{"CHANGELOG.md", "go.mod", "go.sum", "go/internal/foo.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractReportPaths = %v, want %v", got, want)
	}
}

func TestOutOfManifest_FlagsUndeclaredPaths(t *testing.T) {
	changed := []string{
		"go/internal/core/worktree_clean.go",
		"go/internal/recurrence/digest_test.go",
	}
	manifest := []string{"go/internal/core/worktree_clean.go"}
	got := OutOfManifest(changed, manifest)
	want := []string{"go/internal/recurrence/digest_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OutOfManifest = %v, want %v", got, want)
	}
}

func TestOutOfManifest_InManifestDiffHasNoFalsePositive(t *testing.T) {
	changed := []string{
		"go/internal/core/worktree.go",
		"docs/adr/0071-clean-worktree.md",
	}
	manifest := []string{
		"go/internal/core/worktree.go",
		"docs/adr",
	}
	if got := OutOfManifest(changed, manifest); len(got) != 0 {
		t.Fatalf("OutOfManifest false positive: %v", got)
	}
	if got := OutOfManifest([]string{"docs/adr-other.md"}, manifest); len(got) != 1 {
		t.Fatalf("prefix match leaked across path component: %v", got)
	}
}
