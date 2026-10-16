package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func stagedRepoF4(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("source.go", "package source\n")
	repo.Git("add", "source.go")
	return repo.Dir
}

func requestF4(worktree, workspace string) core.PhaseRequest {
	return core.PhaseRequest{Cycle: 1, RunID: "run-1", AuditRound: 1, Worktree: worktree, Workspace: workspace}
}

func TestPredicateTreeFor_EachCaptureFailureNamesItsStage(t *testing.T) {
	t.Parallel()
	t.Run("worktree is not a repository", func(t *testing.T) {
		t.Parallel()
		tree, err := predicateTreeFor(requestF4(t.TempDir(), t.TempDir()))
		if err == nil || !strings.Contains(err.Error(), "capture predicate tree") || tree != "" {
			t.Fatalf("tree=%q err=%v", tree, err)
		}
	})
	t.Run("a repository with no index is refused at the predicate tree", func(t *testing.T) {
		t.Parallel()
		root := gittest.Fixture(t).Dir
		tree, err := predicateTreeFor(requestF4(root, t.TempDir()))
		if err == nil || !strings.Contains(err.Error(), "capture predicate tree: treefence: snapshot tree: gitexec: copy index") || tree != "" {
			t.Fatalf("tree=%q err=%v", tree, err)
		}
	})
	t.Run("missing identity is refused before any git runs", func(t *testing.T) {
		t.Parallel()
		for _, req := range []core.PhaseRequest{
			{RunID: "r", AuditRound: 1, Worktree: "w"},
			{Cycle: 1, AuditRound: 1, Worktree: "w"},
			{Cycle: 1, RunID: "r", Worktree: "w"},
			{Cycle: 1, RunID: "r", AuditRound: 1},
		} {
			if _, err := predicateTreeFor(req); err == nil || !strings.Contains(err.Error(), "requires cycle, run, round, and worktree identity") {
				t.Errorf("req %+v: err = %v", req, err)
			}
		}
	})
}

func TestSealPredicateEvidence_RefusesAChangedTreeAndAMissingVerdict(t *testing.T) {
	t.Parallel()
	root := stagedRepoF4(t)
	ws := t.TempDir()
	req := requestF4(root, ws)
	tree, err := predicateTreeFor(req)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("capture fails", func(t *testing.T) {
		t.Parallel()
		if err := sealPredicateEvidence(core.PhaseRequest{}, tree); err == nil || !strings.Contains(err.Error(), "requires cycle") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the tree moved while the host verified", func(t *testing.T) {
		t.Parallel()
		if err := sealPredicateEvidence(req, "0000000000000000000000000000000000000001"); err == nil || !strings.Contains(err.Error(), "worktree changed during host verification") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the verdict the generator should have written is absent", func(t *testing.T) {
		t.Parallel()
		if err := sealPredicateEvidence(req, tree); err == nil || !os.IsNotExist(unwrapF4(err)) {
			t.Fatalf("err = %v", err)
		}
	})
}

func unwrapF4(err error) error {
	for {
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		err = u.Unwrap()
	}
}

func TestPrepareEvidence_HostPredicateFaultsFailTheAuditWithTheCause(t *testing.T) {
	t.Parallel()
	t.Run("an evidence hook without a verdict generator", func(t *testing.T) {
		t.Parallel()
		called := false
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("## Verdict\n**PASS**\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a := &auditClassification{
			hooks: hooks{predicateEvidence: func(core.PhaseRequest) (func() error, error) { called = true; return nil, nil }},
			req:   core.PhaseRequest{Workspace: ws},
		}

		a.prepareEvidence()

		if called {
			t.Error("the evidence hook must not run without a generator")
		}
		if a.predicateErr == nil || !strings.Contains(a.predicateErr.Error(), "host predicate generator is not configured") {
			t.Fatalf("predicateErr = %v", a.predicateErr)
		}
		if a.verdict != core.VerdictFAIL || !containsF4(a.overrodeBy, "host predicate execution") {
			t.Errorf("verdict=%s overrodeBy=%v", a.verdict, a.overrodeBy)
		}
	})
	t.Run("a verdict path that cannot be inspected", func(t *testing.T) {
		t.Parallel()
		blocker := filepath.Join(t.TempDir(), "workspace-is-a-file")
		if err := os.WriteFile(blocker, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		ran := false
		a := &auditClassification{
			hooks: hooks{genVerdict: func(core.PhaseRequest) error { ran = true; return nil }},
			req:   core.PhaseRequest{Workspace: blocker},
		}

		a.prepareEvidence()

		if ran {
			t.Error("the generator must not run when the verdict path cannot be inspected")
		}
		if a.predicateErr == nil || a.verdict != core.VerdictFAIL {
			t.Fatalf("predicateErr=%v verdict=%s", a.predicateErr, a.verdict)
		}
	})
}

func TestPrepareEvidence_ProbeQuarantineFailureIsAWarningNotAFailure(t *testing.T) {
	t.Parallel()
	root := gittest.Fixture(t).Dir
	probe := filepath.Join(root, "probe_test.go")
	ws := t.TempDir()
	prompt := filepath.Join(ws, auditPromptArtifact)
	if err := os.WriteFile(prompt, []byte("prompt"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(prompt, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(probe, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, auditProbesDir), []byte("a file where the probe store belongs"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &auditClassification{req: core.PhaseRequest{Cycle: 3, Worktree: root, Workspace: ws}}

	a.prepareEvidence()

	var warned bool
	for _, d := range a.diagnostics {
		if d.Severity == "warning" && strings.Contains(d.Message, "probe quarantine:") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("diagnostics = %+v", a.diagnostics)
	}
	if _, err := os.Stat(probe); err != nil {
		t.Errorf("a probe that could not be preserved must not be removed: %v", err)
	}
}

func containsF4(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
