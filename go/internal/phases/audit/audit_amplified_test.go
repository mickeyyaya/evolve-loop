package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSkillsDriftCheckDefault_WithWorktree_NotNoOp(t *testing.T) {
	tmp := t.TempDir()
	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: tmp, ProjectRoot: ""})
	if err == nil {
		t.Error("want error from skillcheck.Check on empty Worktree dir; got nil (indicates unexpected no-op or incorrect guard)")
	}
	if got != nil {
		t.Errorf("want nil drift list on infra error; got %v", got)
	}
}

func TestGofmtCheckDefault_WithWorktree_PreferredOverProjectRoot(t *testing.T) {
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "go", "bad.go"),
		[]byte("package p\nfunc F( ){\nx:=1\n_=x\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cleanRoot, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cleanRoot, "go", "good.go"),
		[]byte("package p\n\nfunc G() {\n\tx := 1\n\t_ = x\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := gofmtCheckDefault(core.PhaseRequest{Worktree: worktree, ProjectRoot: cleanRoot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("want dirty file detected from Worktree (not bypassed by clean ProjectRoot); got none")
	}
}

func TestGofmtCheckDefault_WithWorktree_CleanWorktree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "clean.go"),
		[]byte("package p\n\nfunc H() {\n\ty := 2\n\t_ = y\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := gofmtCheckDefault(core.PhaseRequest{Worktree: root, ProjectRoot: ""})
	if err != nil {
		t.Fatalf("unexpected error for clean worktree: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 offenders for clean worktree, got %d: %v", len(got), got)
	}
}
