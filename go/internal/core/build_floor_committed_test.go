//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultBuildFloorChecks_SeesCommittedBuilderWork(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	wt := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(wt, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	write("go/go.mod", "module floorfixture\n\ngo 1.23\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	baseOut, err := exec.Command("git", "-C", wt, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))

	// The builder adds a package whose test fails and commits it, as its protocol requires.
	write("go/bad/bad.go", "package bad\n\nfunc Two() int { return 3 }\n")
	write("go/bad/bad_test.go", "package bad\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) {\n\tif Two() != 2 {\n\t\tt.Fatal(\"Two() != 2\")\n\t}\n}\n")
	run("add", "-A")
	run("commit", "-q", "-m", "builder work")

	if ps := changedGoTestPackages(changedWorktreePaths(context.Background(), wt)); len(ps) != 0 {
		t.Fatalf("precondition: HEAD-diff must be empty after the builder commit; got %v", ps)
	}
	fails := DefaultBuildFloorChecks(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Worktree: wt, WorktreeBaseSHA: base,
	})
	if len(fails) != 1 || !strings.Contains(fails[0], "bad") {
		t.Fatalf("base-diff floor must catch the committed failing package; got %v", fails)
	}
	write("go/.apicover-enforce", "./bad\n")
	write("go/bad/extra.go", "package bad\n\n// Unnamed is exported but no test names it.\nfunc Unnamed() int { return 1 }\n")
	run("add", "-A")
	run("commit", "-q", "-m", "unnamed export")
	// Make the unit test pass so only the naming defect remains.
	write("go/bad/bad.go", "package bad\n\nfunc Two() int { return 2 }\n")
	run("add", "-A")
	run("commit", "-q", "-m", "fix Two")
	fails = DefaultBuildFloorChecks(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Worktree: wt, WorktreeBaseSHA: base,
	})
	if len(fails) != 1 || !strings.Contains(fails[0], "Unnamed") {
		t.Fatalf("floor must catch the unnamed export in an enforced changed package; got %v", fails)
	}
	write("go/bad/extra_test.go", "package bad\n\nimport \"testing\"\n\nfunc TestUnnamed(t *testing.T) {\n\tif Unnamed() != 1 {\n\t\tt.Fatal()\n\t}\n}\n")
	run("add", "-A")
	run("commit", "-q", "-m", "name it")

	if fails := DefaultBuildFloorChecks(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Worktree: wt, WorktreeBaseSHA: base,
	}); len(fails) != 0 {
		t.Fatalf("green committed work must pass the floor; got %v", fails)
	}
}
