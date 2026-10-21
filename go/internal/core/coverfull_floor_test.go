package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func tinyModule(t *testing.T, files map[string]string) string {
	t.Helper()
	moduleDir := filepath.Join(t.TempDir(), "go")
	writeFileAt(t, filepath.Join(moduleDir, "go.mod"), "module tiny\n\ngo 1.21\n")
	for name, body := range files {
		writeFileAt(t, filepath.Join(moduleDir, name), body)
	}
	return moduleDir
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitexec.Default(dir).Output(context.Background(), args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func TestAddedTaggedTestFailures_UnusableDiffAndBadConstraintAndExcludedFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := addedTaggedTestFailures(ctx, ReviewInput{Worktree: t.TempDir(), WorktreeBaseSHA: "deadbeef"}, "/m"); got != nil {
		t.Fatalf("non-git worktree = %v", got)
	}

	repo := initTestRepo(t)
	base := gitIn(t, repo, "rev-parse", "HEAD")
	in := ReviewInput{Worktree: repo, WorktreeBaseSHA: base}

	writeFileAt(t, filepath.Join(repo, "go", "pkg", "bad_test.go"), "//go:build (\n\npackage pkg\n")
	if got := addedTaggedTestFailures(ctx, in, filepath.Join(repo, "go")); got != nil {
		t.Fatalf("grouping failure must be non-fatal, got %v", got)
	}
	if err := os.Remove(filepath.Join(repo, "go", "pkg", "bad_test.go")); err != nil {
		t.Fatal(err)
	}

	writeFileAt(t, filepath.Join(repo, "go", "pkg", "win_test.go"), "//go:build requires_tmux\n\npackage pkg\n")
	if got := addedTaggedTestFailures(ctx, in, filepath.Join(repo, "go")); len(got) != 0 {
		t.Fatalf("excluded file must not fail the floor, got %v", got)
	}
}

func TestApicoverNamingFailures_TestFailureTruncatesAndReports(t *testing.T) {
	t.Parallel()
	moduleDir := tinyModule(t, map[string]string{
		"p/p.go":      "package p\n\nfunc A() int { return 1 }\n",
		"p/p_test.go": fmt.Sprintf("package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { t.Fatal(%q) }\n", strings.Repeat("x", 2000)),
	})
	got := apicoverNamingFailures(context.Background(), ReviewInput{}, moduleDir, []string{"./p"}, nil)
	if len(got) != 1 || !strings.Contains(got[0], "enforced package tests FAIL") || !strings.HasSuffix(got[0], "…") {
		t.Fatalf("got = %v", got)
	}
}

func TestApicoverNamingFailures_UnnamedExportsReportIsTruncated(t *testing.T) {
	t.Parallel()
	var src strings.Builder
	src.WriteString("package p\n\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&src, "func UnnamedExportedSymbolNumber%03d() int { return %d }\n", i, i)
	}
	moduleDir := tinyModule(t, map[string]string{
		"p/p.go":      src.String(),
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n",
	})
	got := apicoverNamingFailures(context.Background(), ReviewInput{}, moduleDir, []string{"./p"}, []string{"go/p/p.go"})
	if len(got) != 1 || !strings.Contains(got[0], "apicover naming floor") {
		t.Fatalf("got = %v", got)
	}
	if !strings.HasSuffix(got[0], "…") {
		t.Fatalf("report must be truncated: %d bytes", len(got[0]))
	}
}

func TestApicoverNamingFailures_CoverPlumbingFailureSkipsTheCheck(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent")
	moduleDir := tinyModule(t, map[string]string{
		"p/p.go":      "package p\n\nfunc A() int { return 1 }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
	})
	t.Setenv("TMPDIR", absent)
	if got := apicoverNamingFailures(context.Background(), ReviewInput{}, moduleDir, []string{"./p"}, nil); got != nil {
		t.Fatalf("plumbing failure must skip, got %v", got)
	}
}

func TestApicoverNamingFailures_MeasurementFailureSkipsTheCheck(t *testing.T) {
	t.Parallel()
	moduleDir := tinyModule(t, map[string]string{
		"p/p.go":        "package p\n\nfunc A() int { return 1 }\n",
		"p/p_test.go":   "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
		"p/bad_test.go": "//go:build ignore\n\nthis is not go\n",
	})
	if got := apicoverNamingFailures(context.Background(), ReviewInput{}, moduleDir, []string{"./p"}, nil); got != nil {
		t.Fatalf("measurement failure must skip, got %v", got)
	}
}

func TestPersonaBudgetFailures_LongOutputIsTruncated(t *testing.T) {
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	buildSelfCheckRunner = func(context.Context, string, string) (string, bool) { return strings.Repeat("y", 900), false }
	got := personaBudgetFailures(context.Background(), t.TempDir(), []string{"agents/evolve-builder.md"})
	if len(got) != 1 || !strings.HasSuffix(got[0], "…") {
		t.Fatalf("got = %v", got)
	}
}
