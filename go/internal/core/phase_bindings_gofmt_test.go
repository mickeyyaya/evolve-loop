package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

func TestNormalizeBuildWorktree_GofmtsAfterNonBuildPhase(t *testing.T) {
	wt := t.TempDir()
	goDir := filepath.Join(wt, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fp := filepath.Join(goDir, "amp_test.go")
	dirty := "package sample\n\nfunc Sample() int {\n\tx :=  1\n\treturn x\n}\n"
	want := "package sample\n\nfunc Sample() int {\n\tx := 1\n\treturn x\n}\n"
	if err := os.WriteFile(fp, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}

	// next=audit is a NON-build phase; the soft-reset is skipped but gofmt must
	// still run so test-amplification's output is clean before the audit gate.
	(&Orchestrator{}).normalizeBuildWorktree(context.Background(), PhaseAudit, CycleState{ActiveWorktree: wt}, "")

	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("ReadFile after normalizeBuildWorktree: %v", err)
	}
	if string(got) != want {
		t.Fatalf("non-build phase did not gofmt the worktree (test-amplification gap):\n%q", string(got))
	}
}
