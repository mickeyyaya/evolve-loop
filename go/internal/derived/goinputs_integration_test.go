//go:build integration

package derived

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResolveGoInputs_ReadsTheRepository(t *testing.T) {
	dirs, err := ResolveGoInputs(context.Background(), repoRootForTest(t))
	if err != nil {
		t.Fatalf("ResolveGoInputs: %v", err)
	}
	for set, want := range map[GoInputSet][]string{
		SkillcheckClosure: {"go/internal/skillcheck", "go/internal/phasespec"},
		CodeRegistrars:    {"go/internal/core", "go/internal/signalcenter"},
	} {
		for _, dir := range want {
			if !slices.Contains(dirs[set], dir) {
				t.Errorf("%s = %v, want it to hold %s", set, dirs[set], dir)
			}
		}
	}
	if slices.Contains(dirs[SkillcheckClosure], "go/internal/core") {
		t.Error("the skillcheck closure holds go/internal/core, which skillcheck does not import")
	}
}

func TestResolveGoInputs_AWorktreeWithoutAModuleIsAnError(t *testing.T) {
	worktree := t.TempDir()
	if _, err := ResolveGoInputs(context.Background(), worktree); err == nil {
		t.Fatal("a worktree with no go directory: ResolveGoInputs = nil error, want one")
	}
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveGoInputs(context.Background(), worktree); err == nil || !strings.Contains(err.Error(), "go list") {
		t.Fatalf("a go directory with no module: ResolveGoInputs = %v, want a go list error", err)
	}
}

func TestResolveGoInputs_AGoListFailureKeepsTheRegistrars(t *testing.T) {
	worktree := t.TempDir()
	dir := filepath.Join(worktree, "go", "internal", "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc init() { RegisterCode(m, c, \"d\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs, err := ResolveGoInputs(context.Background(), worktree)

	if err == nil || !slices.Equal(dirs[CodeRegistrars], []string{"go/internal/a"}) {
		t.Fatalf("ResolveGoInputs = (%v, %v), want the registrars kept beside the go list error", dirs, err)
	}
}
