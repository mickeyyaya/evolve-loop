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

// Bare root-level filenames (no '/') that ARE declared must be extracted — else
// the manifest gate flags a legit CHANGELOG.md / go.mod change as out-of-
// manifest (noise in shadow, a FALSE-BLOCK under enforce). Prose tokens that
// merely contain a dot (e.g., version "1.0", "cfg.Now") must still NOT match:
// the extension allow-list is the discriminator.
func TestExtractReportPaths_BareRootFilenames(t *testing.T) {
	// The leading-dot `.goreleaser.yml` must be a CLEAN non-match — the bare-alt
	// must not truncate it to a bogus root filename (left-boundary check).
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
