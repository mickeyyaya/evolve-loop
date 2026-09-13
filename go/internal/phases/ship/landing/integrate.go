package landing

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// Integration is the ff-merge request: the cycle branch that lands on the
// integration branch, the tracked binary reset to HEAD before the merge (the
// host's belief — the leaf spells no path), and whether the ship runs under
// a fleet (the host reads EVOLVE_FLEET; the leaf never reads the environment).
type Integration struct {
	Branch      string
	CycleBranch string
	Binary      string
	// Fleet selects the divergence class: true → GIT_FLEET_REBASE_NEEDED /
	// transient (a peer lane moved main; core's rebase engine recovers),
	// false → GIT_FF_MERGE_DIVERGED / precondition (the in-ship repair ladder).
	Fleet bool
	// Log receives the `[ship]   OK: ff-merged` line; nil is the Null Object.
	Log func(string)
}

// Integrate resets the tracked binary (best-effort — a failure is
// SHIP_LANDING_BINARY_RESET_FAILED and the merge still runs), then fast-
// forwards the cycle branch into the integration branch on the operator
// streams. A merge that errs or exits non-zero is the class-by-Fleet
// ShipError with Debug{git_rc, git_err, cycle_branch, branch, step=integrate}.
func (l *Landing) Integrate(ctx context.Context, req Integration) error {
	if exit, err := l.git(ctx, []string{"checkout", "HEAD", "--", req.Binary}, io.Discard, io.Discard); exit != 0 || err != nil {
		l.warn("Landing.Integrate", CodeBinaryResetFailed,
			fmt.Sprintf("could not reset %s to HEAD (exit=%d, err=%v); ff-merge may still fail if it is dirty", req.Binary, exit, err),
			map[string]string{shiperr.StepKey: stepIntegrate, "path": req.Binary, shiperr.GitRCKey: fmt.Sprintf("%d", exit), "git_err": errText(err)})
	}
	s := l.streams()
	exit, err := l.git(ctx, []string{"merge", "--ff-only", req.CycleBranch}, s.Stdout, s.Stderr)
	if err != nil || exit != 0 {
		return l.diverged(req, exit, err)
	}
	emitLog(req.Log, fmt.Sprintf("[ship]   OK: ff-merged %s into %s", req.CycleBranch, req.Branch))
	return nil
}

// diverged classifies a failed ff-merge: under a fleet it is the expected
// concurrency case (transient, rebase + re-verify); otherwise the terminal
// precondition the collider ladder may repair.
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
