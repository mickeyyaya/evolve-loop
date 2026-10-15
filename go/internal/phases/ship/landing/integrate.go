package landing

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

type Integration struct {
	Branch      string
	CycleBranch string
	Commit      string
	Binary      string
	Fleet       bool
	Log         func(string)
}

func (l *Landing) CheckFastForward(ctx context.Context, req Integration) error {
	exit, err := l.git(ctx, []string{"merge-base", "--is-ancestor", req.Branch, req.Commit}, io.Discard, io.Discard)
	if err == nil && exit == 0 {
		return nil
	}
	return l.diverged(req, exit, err)
}

func (l *Landing) Integrate(ctx context.Context, req Integration) {
	if exit, err := l.git(ctx, []string{"checkout", "HEAD", "--", req.Binary}, io.Discard, io.Discard); exit != 0 || err != nil {
		l.warn("Landing.Integrate", CodeBinaryResetFailed,
			fmt.Sprintf("could not reset %s to HEAD (exit=%d, err=%v); ff-merge may still fail if it is dirty", req.Binary, exit, err),
			map[string]string{shiperr.StepKey: stepIntegrate, "path": req.Binary, shiperr.GitRCKey: fmt.Sprintf("%d", exit), "git_err": errText(err)})
	}
	s := l.streams()
	exit, err := l.git(ctx, []string{"merge", "--ff-only", req.Commit}, s.Stdout, s.Stderr)
	if err != nil || exit != 0 {
		l.warn("Landing.Integrate", CodeAdvanceFailed,
			fmt.Sprintf("the fast-forward of %s to the pushed %s failed (exit=%d, err=%v); origin holds the commit, and the next landing fast-forwards %s to origin before its own check", req.Branch, req.Commit, exit, err, req.Branch),
			map[string]string{shiperr.StepKey: stepIntegrate, shiperr.BranchKey: req.Branch, "commit": req.Commit, shiperr.GitRCKey: fmt.Sprintf("%d", exit), "git_err": errText(err)})
		emitLog(req.Log, fmt.Sprintf("[ship] WARN: the fast-forward of %s to the pushed %s failed (rc=%d); the next landing fast-forwards %s to origin before its own check", req.Branch, req.Commit, exit, req.Branch))
		return
	}
	emitLog(req.Log, fmt.Sprintf("[ship]   OK: ff-merged %s into %s", req.CycleBranch, req.Branch))
}

func (l *Landing) diverged(req Integration, exit int, err error) *shiperr.ShipError {
	kv := []string{shiperr.GitRCKey, fmt.Sprintf("%d", exit), "git_err", errText(err), shiperr.CycleBranchKey, req.CycleBranch, shiperr.BranchKey, req.Branch}
	if req.Fleet {
		return fail(stepIntegrate, shiperr.CodeGitFleetRebaseNeeded, shiperr.ShipClassTransient,
			fmt.Sprintf("ship: fleet ff-merge %s into %s diverged (a peer cycle moved %s mid-pipeline); rebase + re-verify the merged tree, then re-ship", req.CycleBranch, req.Branch, req.Branch),
			kv...)
	}
	return fail(stepIntegrate, shiperr.CodeGitFFMergeDiverged, shiperr.ShipClassPrecondition,
		fmt.Sprintf("ship: ff-merge %s into %s failed (rc=%d; divergent history): %v", req.CycleBranch, req.Branch, exit, err),
		kv...)
}
