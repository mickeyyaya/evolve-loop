package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type refusingCycleStateStorage struct{ recStorage }

func (s *refusingCycleStateStorage) WriteCycleState(context.Context, core.CycleState) error {
	return errors.New("cycle-state disk full")
}

func reentryAbortReason(t *testing.T, center *signalcenter.Center) string {
	t.Helper()
	events := center.Recent()
	if len(events) != 1 || events[0].Code != core.CodeRebaseReentryAborted {
		t.Fatalf("want exactly one re-entry abort event, got %+v", events)
	}
	return events[0].Reason
}

func resolveSharedConflict(t *testing.T, dir string) {
	t.Helper()
	if err := writeFileT(filepath.Join(dir, "shared.md"), "peer line\n"); err != nil {
		t.Fatal(err)
	}
}

func TestResumeFleetRebaseAfterDebugger_EachFailedStepEndsTheCycleNamingTheStep(t *testing.T) {
	cases := []struct {
		name       string
		arrange    func(t *testing.T, dir string)
		storage    core.Storage
		version    int
		wantReason string
	}{
		{"the carry cannot stage the resolution", func(t *testing.T, dir string) {
			resolveSharedConflict(t, dir)
			if err := os.WriteFile(filepath.Join(dir, ".git", "index.lock"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, &recStorage{}, 0, "carry the resolved tree: git add -u: exit"},
		{"the rebase fails without a conflict", func(t *testing.T, dir string) {
			runGitT(t, dir, "checkout", "-q", "main")
			if err := writeFileT(filepath.Join(dir, "peer-new.txt"), "main's file\n"); err != nil {
				t.Fatal(err)
			}
			runGitT(t, dir, "add", "peer-new.txt")
			runGitT(t, dir, "commit", "-q", "-m", "peer adds a file")
			runGitT(t, dir, "checkout", "-q", "cycle")
			resolveSharedConflict(t, dir)
			if err := writeFileT(filepath.Join(dir, "peer-new.txt"), "an untracked lane copy\n"); err != nil {
				t.Fatal(err)
			}
		}, &recStorage{}, 0, "the fleet rebase failed (infra)"},
		{"the rebased change cannot be pended", func(t *testing.T, dir string) {
			resolveSharedConflict(t, dir)
			hook := filepath.Join(dir, ".git", "hooks", "post-rewrite")
			body := "#!/bin/sh\n: > \"$(git rev-parse --git-dir)/refs/heads/cycle.lock\"\n"
			if err := os.WriteFile(hook, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
		}, &recStorage{}, 0, "pend the resolved change: git reset --soft"},
		{"the rebased explanation cannot be routed", func(t *testing.T, dir string) {
			resolveSharedConflict(t, dir)
		}, &refusingCycleStateStorage{}, 1, "the rebased explanation could not be routed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := initConflictRebaseRepoT(t)
			tc.arrange(t, dir)
			center := signalcenter.New()
			o := core.NewOrchestrator(tc.storage, &fakeLedger{}, newRunners(nil), core.WithSignalCenter(center))
			cs := core.CycleState{
				CycleID: 6, RunID: "run-6", ShipRecoveryCode: string(core.CodeGitFleetRebaseNeeded),
				ActiveWorktree: dir, WorkspacePath: t.TempDir(), ExplanationDocumentationVersion: tc.version,
			}

			next, resumed := o.ResumeFleetRebaseAfterDebuggerForTest(context.Background(), t.TempDir(), 6, &cs, core.PhaseBuild, 0, 1)

			if !resumed || next != core.PhaseEnd {
				t.Fatalf("re-entry = (%q, %v), want the cycle ended", next, resumed)
			}
			if reason := reentryAbortReason(t, center); !strings.Contains(reason, tc.wantReason) {
				t.Fatalf("abort reason %q lacks %q", reason, tc.wantReason)
			}
		})
	}
}
