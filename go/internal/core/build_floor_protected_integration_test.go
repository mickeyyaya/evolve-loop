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

func TestProtectedSurfaceFloorChecks_RefusesTheCycle1689Shape(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	wt := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", wt}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
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
	write("go/internal/core/cyclerun.go", "package core\n")
	write("go/internal/core/lost_landing_floor.go", "package core\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")

	write("go/internal/core/lost_landing_floor.go", "package core\n\n// reshaped\n")
	write("go/internal/core/cyclerun.go", "package core\n\n// call site dropped a field\n")
	run("add", "-A")
	run("commit", "-q", "-m", "builder work")
	write("go/internal/core/new_untracked.go", "package core\n")

	member := func(p string) bool { return p == "go/internal/core/cyclerun.go" }
	got := ProtectedSurfaceFloorChecks(member)(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Worktree: wt, WorktreeBaseSHA: base,
	})
	if len(got) != 1 || !strings.Contains(got[0], "go/internal/core/cyclerun.go") {
		t.Fatalf("the committed protected change is named, alone: %v", got)
	}
	// No cycle base recorded: the floor falls back to the HEAD diff plus untracked files.
	write("go/internal/core/cyclerun.go", "package core\n\n// an uncommitted shell edit\n")
	got = ProtectedSurfaceFloorChecks(member)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: wt})
	if len(got) != 1 || !strings.Contains(got[0], "go/internal/core/cyclerun.go") {
		t.Fatalf("the HEAD-fallback path set names the uncommitted protected edit: %v", got)
	}
}

func TestProtectedSurfaceFloorChecks_SeesARenameOutOfTheSurface(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	wt := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", wt}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	body := "package core\n\n" + strings.Repeat("// a pinned guard assertion\n", 20)
	p := filepath.Join(wt, "go/internal/core/orchestrator_guard_test.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("mv", "go/internal/core/orchestrator_guard_test.go", "go/internal/core/orchestrator_pin_test.go")
	git("commit", "-q", "-m", "a harmless-looking split")

	member := func(p string) bool { return p == "go/internal/core/orchestrator_guard_test.go" }
	got := ProtectedSurfaceFloorChecks(member)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: wt, WorktreeBaseSHA: base})
	if len(got) != 1 || !strings.Contains(got[0], "go/internal/core/orchestrator_guard_test.go") {
		t.Fatalf("a rename out of the protected surface is judged by its OLD path: %v", got)
	}
}
