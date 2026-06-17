package core

import (
	"os"
	"path/filepath"
	"testing"
)

// I9: after the build phase the orchestrator must deterministically gofmt -s the
// builder's output so a formatting lapse never reaches (and FAILs) the audit
// gofmt gate. normalizeBuildGofmt formats the worktree's go/ module in place.
func TestNormalizeBuildGofmt_FormatsWorktreeGoModule(t *testing.T) {
	wt := t.TempDir()
	goDir := filepath.Join(wt, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fp := filepath.Join(goDir, "sample.go")
	dirty := "package sample\n\nfunc Sample() int {\n\tx :=  1\n\treturn x\n}\n"
	want := "package sample\n\nfunc Sample() int {\n\tx := 1\n\treturn x\n}\n"
	if err := os.WriteFile(fp, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}

	normalizeBuildGofmt(wt)

	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("ReadFile after normalizeBuildGofmt: %v", err)
	}
	if string(got) != want {
		t.Fatalf("worktree go file not gofmt-normalized after normalizeBuildGofmt:\n%q", string(got))
	}
}

// An empty worktree (provisioning failed) must be a safe no-op, never a panic.
func TestNormalizeBuildGofmt_EmptyWorktreeIsNoOp(t *testing.T) {
	normalizeBuildGofmt("")
}
