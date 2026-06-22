package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func initGitWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"commit", "--allow-empty", "-q", "-m", "base"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = wt
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return wt
}

func TestBuildSelfCheck_WritesArtifactOnFailure(t *testing.T) {
	wt := initGitWorktree(t)
	fp := filepath.Join(wt, "go", "internal", "foo", "foo.go")
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	buildSelfCheckRunner = func(_ context.Context, _, pkg string) (string, bool) {
		return "--- FAIL: TestFoo", pkg != "./internal/foo" // foo fails, others pass
	}

	(&Orchestrator{}).buildSelfCheck(context.Background(), wt)

	data, err := os.ReadFile(filepath.Join(wt, ".evolve", "build-selfcheck.json"))
	if err != nil {
		t.Fatalf("build-selfcheck artifact not written: %v", err)
	}
	if !strings.Contains(string(data), "./internal/foo") {
		t.Fatalf("artifact must name the failing package: %s", data)
	}
}

func TestBuildSelfCheck_NoGoChangesIsNoOp(t *testing.T) {
	wt := initGitWorktree(t)
	if err := os.WriteFile(filepath.Join(wt, "notes.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	buildSelfCheckRunner = func(_ context.Context, _, _ string) (string, bool) { called = true; return "", true }

	(&Orchestrator{}).buildSelfCheck(context.Background(), wt)

	if called {
		t.Fatal("no changed go packages → runner must not run")
	}
	if _, err := os.Stat(filepath.Join(wt, ".evolve", "build-selfcheck.json")); err == nil {
		t.Fatal("no failures → no artifact must be written")
	}
}

func TestBuildSelfCheck_EmptyWorktreeIsNoOp(t *testing.T) {
	(&Orchestrator{}).buildSelfCheck(context.Background(), "") // must not panic
}

func TestRealGoUnitTest_DetectsPassAndFail(t *testing.T) {
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(pkg, body string) {
		dir := filepath.Join(mod, pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, pkg+"_test.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pass", "package pass\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n")
	write("fail", "package fail\nimport \"testing\"\nfunc TestBad(t *testing.T) { t.Fatal(\"boom\") }\n")

	if out, ok := realGoUnitTest(context.Background(), mod, "./pass"); !ok {
		t.Fatalf("passing package must report ok, output:\n%s", out)
	}
	if _, ok := realGoUnitTest(context.Background(), mod, "./fail"); ok {
		t.Fatal("failing package must report not-ok")
	}
}

// TestRealGoUnitTest_BuildTagExcludedIsNotFailure guards the false-positive a
// live cycle surfaced: the self-check runs untagged `go test` (by design — no
// integration tag), but every cycle materializes a //go:build-gated acceptance
// package (acs/cycleN). An untagged `go test` of such a package reports "build
// constraints exclude all Go files … [setup failed]" — nothing to unit-test, NOT
// a regression — so it must report ok, or the self-check WARNs on every cycle.
func TestRealGoUnitTest_BuildTagExcludedIsNotFailure(t *testing.T) {
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(mod, "tagged")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only file is behind a build tag, mirroring acs/cycleN's //go:build acs.
	body := "//go:build acs\n\npackage tagged\n\nimport \"testing\"\n\nfunc TestGated(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "tagged_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, ok := realGoUnitTest(context.Background(), mod, "./tagged"); !ok {
		t.Fatalf("build-tag-excluded package must report ok (nothing to unit-test), output:\n%s", out)
	}
}

// TestGoTestExcludedByBuildTags asserts the classifier separates a build-tag
// exclusion (nothing to test) from genuine failures (compile error / assertion)
// that must still be reported.
func TestGoTestExcludedByBuildTags(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"build-tag exclusion (package under test)", "# x/acs/cycle4\npackage x/acs/cycle4: build constraints exclude all Go files in /p\nFAIL\tx/acs/cycle4 [setup failed]\nFAIL\n", true},
		{"transitive import excluded — real build failure, not swallowed", "# p1\npackage p1 (test)\n\timports p2\n\timports p3: build constraints exclude all Go files in /p\nFAIL\tp1 [setup failed]\nFAIL\n", false},
		{"assertion failure", "--- FAIL: TestBad (0.00s)\n    boom\nFAIL\tx\t0.01s\nFAIL\n", false},
		{"compile error", "# x\nx.go:3:1: undefined: Foo\nFAIL\tx [build failed]\n", false},
		{"clean pass", "ok  \tx\t0.012s\n", false},
		{"no-test-files string is not an exclusion", "?   \tx\t[no test files]\n", false},
	}
	for _, c := range cases {
		if got := goTestExcludedByBuildTags(c.output); got != c.want {
			t.Errorf("%s: goTestExcludedByBuildTags = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestChangedGoTestPackages_DerivesUniqueModulePackages(t *testing.T) {
	paths := []string{
		"go/internal/bridge/driver_claudetmux.go",
		"go/internal/bridge/other.go", // same package → deduped
		"go/internal/cycleclassify/classify.go",
		"go/cmd/evolve/cmd_x_test.go", // _test.go still maps to its package
		"docs/foo.md",                 // non-.go → skipped
		"README.md",                   // skipped
		"go/go.mod",                   // not .go → skipped
		"landing/main.go",             // not under the go/ module → skipped
	}
	got := changedGoTestPackages(paths)
	want := []string{"./cmd/evolve", "./internal/bridge", "./internal/cycleclassify"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changedGoTestPackages = %v, want %v (unique, sorted, ./-prefixed)", got, want)
	}
}

func TestChangedGoTestPackages_EmptyWhenNoGoModuleChanges(t *testing.T) {
	if got := changedGoTestPackages([]string{"docs/x.md", "go/go.sum", "landing/x.go"}); len(got) != 0 {
		t.Fatalf("expected no packages, got %v", got)
	}
}

func TestRunBuildSelfCheck_CollectsOnlyFailingPackages(t *testing.T) {
	var ran []string
	run := func(_ context.Context, moduleDir, pkg string) (string, bool) {
		ran = append(ran, pkg)
		if moduleDir == "" {
			t.Fatal("runner must receive the module dir")
		}
		// bridge breaks a unit test; cycleclassify passes.
		if pkg == "./internal/bridge" {
			return "--- FAIL: TestClaudeTmux_CostGuards_BaseURL", false
		}
		return "ok", true
	}
	fails := runBuildSelfCheck(context.Background(), "/wt/go",
		[]string{"./internal/bridge", "./internal/cycleclassify"}, run)

	if len(ran) != 2 {
		t.Fatalf("every changed package must be tested, ran %v", ran)
	}
	if len(fails) != 1 || fails[0].Pkg != "./internal/bridge" {
		t.Fatalf("only the failing package must be reported, got %+v", fails)
	}
	if fails[0].Output == "" {
		t.Fatal("a failure must capture the test output for feedback")
	}
}

func TestRunBuildSelfCheck_AllGreenReportsNoFailures(t *testing.T) {
	run := func(_ context.Context, _, _ string) (string, bool) { return "ok", true }
	if fails := runBuildSelfCheck(context.Background(), "/wt/go", []string{"./internal/bridge"}, run); len(fails) != 0 {
		t.Fatalf("all-green must report no failures, got %+v", fails)
	}
}
