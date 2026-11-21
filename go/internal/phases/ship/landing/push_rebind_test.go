package landing

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const (
	localMainRef   = "refs/heads/" + testBranch
	staleLocalMain = "5555555555555555555555555555555555555555"
)

func rejectedWorktreePush(localMainRelation int) *fakeGit {
	f := newFakeGit()
	f.on(worktreePush, scripted{exit: 1, stderr: raceRejection})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" "+testHead, scripted{exit: 1})
	f.on("rev-parse "+localMainRef, scripted{stdout: staleLocalMain + "\n"})
	f.on("merge-base --is-ancestor "+staleLocalMain+" "+testOriginRef, scripted{exit: localMainRelation})
	return f
}

func TestRepairPush_RejectedWithLocalMainBehindOriginRoutesToRebind(t *testing.T) {
	f := rejectedWorktreePush(0)
	l, _ := newLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	se := wantShipError(t, err, shiperr.CodeGitFleetRebaseNeeded, shiperr.ShipClassTransient, "")
	if se.Debug["repair_outcome"] != "needs-rebind" || out.RepairOutcome != RepairNeedsRebind {
		t.Errorf("Debug %v, result %+v: a local main behind origin must name the rebind outcome", se.Debug, out)
	}
	if se.Debug["origin_ref"] != testOriginRef || se.Debug["commit"] != testHead {
		t.Errorf("Debug %v must name the origin ref and the commit", se.Debug)
	}
	if strings.Count(strings.Join(f.calls, "\n"), "push origin") != 1 {
		t.Errorf("no retry push on a rebind route: %v", f.calls)
	}
}

func TestRepairPush_RejectedWithTrulyDivergedOriginStaysNeedsReaudit(t *testing.T) {
	f := rejectedWorktreePush(1)
	l, _ := newLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition, "")
	if se.Debug["repair_outcome"] != "needs-reaudit" || out.RepairOutcome != RepairNeedsReaudit {
		t.Errorf("Debug %v, result %+v: a local main that is not behind origin stays needs-reaudit", se.Debug, out)
	}
}

func TestRepairPush_LocalMainEqualToOriginIsNotBehindAndStaysNeedsReaudit(t *testing.T) {
	f := newFakeGit()
	f.on(worktreePush, scripted{exit: 1, stderr: raceRejection})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" "+testHead, scripted{exit: 1})
	f.on("rev-parse "+localMainRef, scripted{stdout: testOriginRef + "\n"})
	l, _ := newLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition, "")
	if out.RepairOutcome != RepairNeedsReaudit {
		t.Errorf("result %+v: equal refs are not a strict ancestor", out)
	}
}

func TestRepairPush_ARebindProbeThatFailsKeepsNeedsReaudit(t *testing.T) {
	f := newFakeGit()
	f.on(worktreePush, scripted{exit: 1, stderr: raceRejection})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" "+testHead, scripted{exit: 1})
	f.on("rev-parse "+localMainRef, scripted{exit: 128})
	l, _ := newLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition, "")
	if out.RepairOutcome != RepairNeedsReaudit {
		t.Errorf("result %+v: an unreadable local main never routes to the rebind", out)
	}
}
