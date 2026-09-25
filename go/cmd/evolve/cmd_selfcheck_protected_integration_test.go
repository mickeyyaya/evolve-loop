//go:build integration

package main

// cmd_selfcheck_protected_integration_test.go — F37 wiring proof: the ONE
// production floor composition both roots run refuses the cycle-1689 diff
// through the REAL control-plane manifest (guards.IsProtectedSurface), not a
// stub — go/internal/core/cyclerun.go is a manifest member.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestProductionBuildFloorChecks_RefusesAProtectedFileThroughTheRealManifest(t *testing.T) {
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
	write("go/go.mod", "module floorfixture\n\ngo 1.23\n")
	write("go/internal/core/cyclerun.go", "package core\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	write("go/internal/core/cyclerun.go", "package core\n\n// the builder reached the control plane through a shell tool\n")
	// A placeholder token in the build report proves the deterministic engine
	// is composed too — a pointer pin alone cannot (architecture review F37 M2).
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("# Build Report\n\n`go test -count=1 ./...` → FULLSUITE_PLACEHOLDER\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := productionBuildFloorChecks(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseBuild), Worktree: wt, WorktreeBaseSHA: base, Workspace: ws,
	})
	var protected, placeholder int
	for _, f := range got {
		if strings.Contains(f, "protected control-plane path changed: go/internal/core/cyclerun.go") {
			protected++
		}
		if strings.Contains(f, "FULLSUITE_PLACEHOLDER") {
			placeholder++
		}
	}
	if protected != 1 || placeholder != 1 || !strings.Contains(got[0], "protected control-plane path changed") {
		t.Fatalf("the production floor names the protected path once through the real manifest, FIRST, and the engine's placeholder finding too; got %v", got)
	}
}
