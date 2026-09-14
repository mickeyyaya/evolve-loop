package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags
// reproduces cycle 1679 (2026-09-14): the lane ADDED go/acs/cycle1676
// (`//go:build acs`), the floor's default-context run could not see it
// (buildTagVisiblePackages drops a package with no GoFiles in that context),
// the audit passed, and the ship's added-test backstop was the first to run
// it — red, after the builder had handed the tree over. The floor now runs
// every added tag-gated package under its own tags, through the same seed
// and grouping the ship gate uses, and reports a red like any other floor
// failure, so the builder fixes it while it still owns the tree.
func TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags(t *testing.T) {
	wt := initGitWorktree(t)
	head := exec.Command("git", "rev-parse", "HEAD")
	head.Dir = wt
	baseOut, err := head.Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))
	fp := filepath.Join(wt, "go", "acs", "cycle9", "predicates_test.go")
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte("//go:build acs\n\npackage cycle9\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := buildSelfCheckRunner
	buildSelfCheckRunner = func(context.Context, string, string) (string, bool) { return "", true }
	prevTagged := buildSelfCheckTaggedRunner
	var gotPkg string
	var gotTags []string
	buildSelfCheckTaggedRunner = func(_ context.Context, _ string, pkg string, tags []string) (string, bool) {
		gotPkg, gotTags = pkg, tags
		return "=== RUN   TestRed\n--- FAIL: TestRed (0.00s)\nFAIL\n", false
	}
	t.Cleanup(func() { buildSelfCheckRunner = prev; buildSelfCheckTaggedRunner = prevTagged })
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: wt, ProjectRoot: wt, WorktreeBaseSHA: base}
	out := changedPackageFloorChecks(context.Background(), in, changedFloorPaths(context.Background(), in))
	if gotPkg != "./acs/cycle9" || strings.Join(gotTags, ",") != "acs" {
		t.Fatalf("the added acs package must run under its tags at the floor; ran pkg=%q tags=%v", gotPkg, gotTags)
	}
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "./acs/cycle9") || !strings.Contains(joined, "-tags acs") || !strings.Contains(joined, "unit tests FAIL") || !strings.Contains(joined, "TestRed") {
		t.Fatalf("a red tag-gated added package must be a floor failure naming package, tags and the failing test; got %q", out)
	}
}
