package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

var errGitFault = errors.New("git fault")

const (
	faultBase = "1111111111111111111111111111111111111111"
	faultTree = "2222222222222222222222222222222222222222"
)

func healthyUnwindGit(fail string, body string) *scriptedGit {
	return &scriptedGit{respond: func(args []string) (string, int, error) {
		call := joined(args)
		if strings.HasPrefix(call, fail) {
			return "", 0, errGitFault
		}
		switch {
		case strings.HasPrefix(call, "merge-base"):
			return faultBase, 0, nil
		case strings.HasPrefix(call, "diff-tree"):
			return "D\x00.evolve/inbox/item.json\x00A\x00.evolve/inbox/consumed/item.json\x00", 0, nil
		case strings.HasPrefix(call, "show"):
			return body, 0, nil
		}
		return "", 0, nil
	}}
}

func TestUnwindDecline_AGitFaultIsAnErrorNotADecline(t *testing.T) {
	audited := AuditedChange{Base: faultBase, Tree: faultTree}
	for name, tc := range map[string]struct{ fail, body, want string }{
		"the status":         {fail: "status", want: "read the worktree status"},
		"the fork point":     {fail: "merge-base", want: "resolve the fork point"},
		"the tree lookup":    {fail: "rev-parse", want: "look up the audited tree"},
		"the diff":           {fail: "diff-tree", want: "diff the audited tree against HEAD"},
		"the consumed item":  {fail: "show", want: "read consumed item"},
		"an unreadable item": {fail: "none", body: "not json", want: "read consumed item"},
	} {
		t.Run(name, func(t *testing.T) {
			verdict, err := unwindDecline(context.Background(), "/wt", audited, healthyUnwindGit(tc.fail, tc.body).capture)

			if err == nil || !strings.Contains(err.Error(), tc.want) || verdict != (unwindVerdict{}) {
				t.Fatalf("unwindDecline = (%+v, %v), want an error naming %q and no verdict", verdict, err, tc.want)
			}
		})
	}
}

func TestCarryAuditedTree_AGitFaultIsAnError(t *testing.T) {
	for _, fail := range []string{"-c commit.gpgsign=false commit-tree", "reset"} {
		t.Run(fail, func(t *testing.T) {
			err := carryAuditedTree(context.Background(), "/wt", AuditedChange{Base: faultBase, Tree: faultTree}, healthyUnwindGit(fail, "").capture)

			if !errors.Is(err, errGitFault) {
				t.Fatalf("carryAuditedTree = %v, want the git fault", err)
			}
		})
	}
}

func TestPendRebasedChange_AnUnresolvedForkPointIsAnError(t *testing.T) {
	err := pendRebasedChange(context.Background(), "/wt", healthyUnwindGit("merge-base", "").capture)

	if !errors.Is(err, errGitFault) {
		t.Fatalf("pendRebasedChange = %v, want the git fault", err)
	}
}

func TestUnwindBeforeFleetRebase_NoWorktreeOrAGitFaultUnwindsNothing(t *testing.T) {
	row := LedgerEntry{Cycle: unwindCycle, RunID: unwindRunID, Role: "auditor", Kind: "agent_subprocess", WorktreeTreeSHA: faultTree}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{row}}, buildRunners(nil))
	for name, worktree := range map[string]string{"no worktree": "", "a worktree git cannot read": t.TempDir()} {
		t.Run(name, func(t *testing.T) {
			cs := CycleState{RunID: unwindRunID, ActiveWorktree: worktree, WorktreeBaseSHA: faultBase}

			if got := o.unwindBeforeFleetRebase(context.Background(), t.TempDir(), unwindCycle, cs); got != unwindNone {
				t.Fatalf("unwindBeforeFleetRebase = %v, want unwindNone", got)
			}
		})
	}
}
