//go:build integration

package auditfixports

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const helperTest = "pkg/helper_test.go"

type liveCycle struct {
	tree      buildTree
	workspace string
	fixerRuns int
}

func (c *liveCycle) fixer(extra string) AgentRun {
	return func(_ context.Context, args []string, _ string) (SeatResult, error) {
		c.fixerRuns++
		wt := args[slices.Index(args, "--worktree")+1]
		for _, rel := range append([]string{helperTest}, extra) {
			if rel == "" {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(wt, rel)), 0o755); err != nil {
				return SeatResult{}, err
			}
			if err := os.WriteFile(filepath.Join(wt, rel), []byte("package pkg\n\nfunc countJSON() int { return 1 }\n"), 0o644); err != nil {
				return SeatResult{}, err
			}
		}
		return SeatResult{Verdict: core.VerdictPASS}, nil
	}
}

func reviewer(verdict string) AgentRun {
	return func(context.Context, []string, string) (SeatResult, error) {
		return SeatResult{Verdict: verdict, RunLabel: "agent-go-reviewer-it", Family: "agy"}, nil
	}
}

func newLiveCycle(t *testing.T) *liveCycle {
	t.Helper()
	c := &liveCycle{tree: newBuildTree(t), workspace: t.TempDir()}
	c.tree.repo.Write(helperTest, "package pkg\n")
	if err := os.WriteFile(filepath.Join(c.workspace, core.RunStateFile), []byte(`{"cycle_id":2035}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return c
}

func (c *liveCycle) handler(extra, verdict string) *auditfix.Handler {
	route := routes(map[string]string{FixerProfile: "claude", "go-reviewer": "agy"})
	return auditfix.New(auditfix.Config{Stage: policy.RetryLadderEnforce}, auditfix.Ports{
		Auditor:  Fixer{Run: c.fixer(extra), Route: route, ProjectRoot: c.tree.repo.Dir},
		Reviewer: Reviewer{Run: reviewer(verdict), Route: route, ProjectRoot: c.tree.repo.Dir, Profiles: []string{"go-reviewer"}},
		Gates:    Gates{ProjectRoot: c.tree.repo.Dir, ACS: func(acssuite.Options) (acssuite.Verdict, error) { return acssuite.Verdict{}, nil }},
		Worktree: Worktree{Git: gitexec.Isolated},
	})
}

func (c *liveCycle) input(choose bool) core.RungInput {
	return core.RungInput{
		Rung: retryladder.RungAuditFix, Cycle: 2035, Workspace: c.workspace, Worktree: c.tree.repo.Dir,
		Choice: retryladder.AuditFixChoice{Choose: choose, Input: retryladder.AuditFixInput{Bounds: retryladder.AuditFixBounds{MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, MaxTokens: 6000, NoNewProductionLogic: true}}},
		Failure: phasecontract.FailureBlock{Class: "code-audit-fail",
			Defects: []string{"H1: " + helperTest + ":1 countJSON is missing"}, Prescription: []string{"Add countJSON to " + helperTest}},
	}
}

func TestHandler_LiveGitPassedPatchStaysOnTheWorktree(t *testing.T) {
	t.Parallel()
	c := newLiveCycle(t)

	got, err := c.handler("", core.VerdictPASS).Handle(context.Background(), c.input(true))

	if err != nil || got != (core.RungResult{Handled: true, Reason: auditfix.ReasonPassed}) {
		t.Fatalf("Handle = %+v, %v, want a passed patch", got, err)
	}
	if body := readFile(t, filepath.Join(c.tree.repo.Dir, helperTest)); body == "package pkg\n" {
		t.Fatal("the passed patch was restored")
	}
	rec, _, err := auditfix.ReadRecord(c.workspace)
	if err != nil || len(rec.Patch.Changes) != 1 || rec.Patch.Changes[0].Path != helperTest || rec.Reviewer.Family != "agy" || rec.Auditor.Family != "claude" {
		t.Fatalf("record = %+v, %v, want the measured patch of the cited file and two families", rec, err)
	}
}

func TestHandler_LiveGitFailedReviewRestoresThenStops(t *testing.T) {
	t.Parallel()
	c := newLiveCycle(t)
	h := c.handler("", core.VerdictFAIL)

	first, err := h.Handle(context.Background(), c.input(true))

	if err != nil || !first.Escalate {
		t.Fatalf("Handle = %+v, %v, want an escalation", first, err)
	}
	if body := readFile(t, filepath.Join(c.tree.repo.Dir, helperTest)); body != "package pkg\n" {
		t.Fatalf("the refused patch stayed: %q", body)
	}
	second, err := h.Handle(context.Background(), c.input(false))
	if err != nil || !second.Stop || second.Reason != auditfix.ReasonExhausted || c.fixerRuns != 1 {
		t.Fatalf("second Handle = %+v, %v after %d fixer runs, want a stop with no new fix", second, err, c.fixerRuns)
	}
}

func TestHandler_LiveGitUncitedFileIsRefusedAndRemoved(t *testing.T) {
	t.Parallel()
	c := newLiveCycle(t)

	got, err := c.handler("pkg/extra.go", core.VerdictPASS).Handle(context.Background(), c.input(true))

	if err != nil || !got.Escalate {
		t.Fatalf("Handle = %+v, %v, want an escalation for an uncited file", got, err)
	}
	if _, err := os.Stat(filepath.Join(c.tree.repo.Dir, "pkg/extra.go")); !os.IsNotExist(err) {
		t.Fatalf("the uncited file survived the restore: %v", err)
	}
	if rec, _, _ := auditfix.ReadRecord(c.workspace); rec.Refusal != auditfix.RefusalUncited {
		t.Fatalf("refusal = %q, want %s", rec.Refusal, auditfix.RefusalUncited)
	}
}
