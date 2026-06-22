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

func TestBuildSelfCheck_ClearsStaleArtifactOnPass(t *testing.T) {
	// A prior failed attempt wrote the artifact; the retry's changed package now
	// PASSES. The artifact must be cleared so the toolchain gate (which reads it)
	// does not loop forever on a stale failure. Regression for the gate-hardening
	// after relaunch cycle 12 shipped vet-failing code.
	wt := initGitWorktree(t)
	fp := filepath.Join(wt, "go", "internal", "foo", "foo.go")
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-seed a stale failure artifact.
	if err := os.MkdirAll(filepath.Join(wt, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(wt, ".evolve", "build-selfcheck.json")
	if err := os.WriteFile(stale, []byte(`[{"pkg":"./internal/foo","output":"old failure"}]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	buildSelfCheckRunner = func(_ context.Context, _, _ string) (string, bool) { return "", true } // all pass now

	(&Orchestrator{}).buildSelfCheck(context.Background(), wt)

	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale build-selfcheck artifact must be cleared when the rebuild passes")
	}
}

func TestBuildSelfCheck_NoGoChanges_ClearsStaleArtifact(t *testing.T) {
	// A non-Go-only cycle must still clear a stale failure artifact (a non-Go
	// change cannot be a Go regression). Guards the unconditional clear that sits
	// above the `len(pkgs)==0` early return.
	wt := initGitWorktree(t)
	if err := os.WriteFile(filepath.Join(wt, "notes.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wt, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(wt, ".evolve", "build-selfcheck.json")
	if err := os.WriteFile(stale, []byte(`[{"pkg":"./internal/foo","output":"old"}]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	(&Orchestrator{}).buildSelfCheck(context.Background(), wt)

	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale artifact must be cleared even when no Go packages changed")
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

// TestSanitizeEnv_StripsEvolveRuntimeFlags is the pure classifier for the env
// the self-check's `go test` subprocess runs under: the campaign's per-run
// EVOLVE_* flags (EVOLVE_FLEET etc.) must be removed so tests run in their
// default (CI-like) config, while everything else is preserved. Prefix match,
// not substring (NOTEVOLVE_X is kept).
func TestSanitizeEnv_StripsEvolveRuntimeFlags(t *testing.T) {
	in := []string{"EVOLVE_FLEET=1", "PATH=/bin", "EVOLVE_FLEET_SCOPE=a,b", "HOME=/h", "NOTEVOLVE_FLEET=keep"}
	got := sanitizeEnv(in)
	want := []string{"PATH=/bin", "HOME=/h", "NOTEVOLVE_FLEET=keep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sanitizeEnv = %v, want %v (drop EVOLVE_* by prefix, keep the rest)", got, want)
	}
}

// TestRealGoUnitTest_SanitizesCampaignEnv proves end-to-end that the self-check
// does NOT leak the campaign's runtime env into its `go test` subprocess. A live
// cycle surfaced this: the self-check inherited EVOLVE_FLEET=1, which flips
// internal/bridge's fleet-mode worktree guard into a false failure. The temp
// package's test fails iff it observes EVOLVE_FLEET — so realGoUnitTest passing
// proves the env was sanitized.
func TestRealGoUnitTest_SanitizesCampaignEnv(t *testing.T) {
	t.Setenv("EVOLVE_FLEET", "1") // campaign runtime flag present in the parent env
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(mod, "envcheck")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `package envcheck

import (
	"os"
	"testing"
)

func TestNoFleetLeak(t *testing.T) {
	if os.Getenv("EVOLVE_FLEET") != "" {
		t.Fatal("EVOLVE_FLEET leaked into the go test subprocess")
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "envcheck_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, ok := realGoUnitTest(context.Background(), mod, "./envcheck"); !ok {
		t.Fatalf("self-check must run go test in a sanitized env (no EVOLVE_* leak), output:\n%s", out)
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
