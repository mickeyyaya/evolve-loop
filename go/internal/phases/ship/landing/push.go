package landing

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// PushSite selects the rejection wording of the three push sites — the ONE
// enumerable table of their asymmetries (the worktree site names main's head
// in its message; push-only carries its own prefix).
type PushSite uint8

// The three push sites.
const (
	SiteDirect   PushSite = iota // gitops.go's direct path: "ship: git push failed (rc=%d): %v"
	SiteWorktree                 // the worktree integrate: `rev-parse HEAD` first, then "…; main is at %s: %v" + Debug head
	SitePushOnly                 // `evolve ship --push-only`: "ship --push-only: git push failed (rc=%d): %v"
)

// PushRequest is the push step's input: the branch, the site's wording, and
// the host's two guards on the inline repair — --dry-run and the once-per-Run
// ledger opts.repairAttempted[GIT_PUSH_REJECTED], which the host owns because
// it is shared with the repair ladder and survives the stage re-run.
type PushRequest struct {
	Branch string
	Site   PushSite
	// DryRun skips the repair (repairs mutate); the push itself still runs —
	// push-only under --dry-run pushes today, preserved verbatim.
	DryRun bool
	// RepairAttempted is the host's once-guard IN: true → the rejection is
	// returned untouched with zero repair probes.
	RepairAttempted bool
	// Log receives the `[ship] REPAIR:` lines in their positions; nil is the
	// Null Object.
	Log func(string)
}

// PushResult is what the push step derived: the landed head (empty when the
// push failed or HEAD was unreadable), whether the repair ran (written back
// by the host even on error — the repair sets it before its probes), and the
// repair's outcome.
type PushResult struct {
	Head            string
	RepairAttempted bool
	RepairOutcome   RepairOutcome
}

// RepairOutcome is the res.RepairOutcome vocabulary of the push repair,
// verbatim (postship_landing*_test.go and acs/cycle752 pin the strings).
type RepairOutcome string

// The push repair's outcomes.
const (
	RepairDeclined      RepairOutcome = "declined"
	RepairAlreadyPushed RepairOutcome = "already-pushed"
	RepairPushRetried   RepairOutcome = "push-retried"
	RepairNeedsReaudit  RepairOutcome = "needs-reaudit"
)

// Push pushes the branch on the operator streams. A rejection becomes the
// site's transient GIT_PUSH_REJECTED (step=push) and gets ONE inline fetch +
// fast-forward retry; once the push landed (first try, retried, or already
// on origin) the head is read for the result — an unreadable HEAD is
// SHIP_LANDING_HEAD_READ_FAILED and the ship proceeds with an empty Head.
func (l *Landing) Push(ctx context.Context, req PushRequest) (PushResult, error) {
	var out PushResult
	s := l.streams()
	exit, err := l.git(ctx, []string{"push", "origin", req.Branch}, s.Stdout, s.Stderr)
	if err != nil || exit != 0 {
		if rerr := l.repairPush(ctx, req, l.rejection(ctx, req.Site, req.Branch, exit, err), &out); rerr != nil {
			return out, rerr
		}
	}
	out.Head = l.readHead(ctx)
	return out, nil
}

// rejection builds the site's GIT_PUSH_REJECTED — the three-case wording
// switch. The worktree site probes `rev-parse HEAD` first (its error
// ignored, as before) so the message can name where main is.
func (l *Landing) rejection(ctx context.Context, site PushSite, branch string, exit int, err error) *shiperr.ShipError {
	rc, gitErr := fmt.Sprintf("%d", exit), errText(err)
	switch site {
	case SiteWorktree:
		head, _ := l.Capture(ctx, "rev-parse", "HEAD")
		head = strings.TrimSpace(head)
		return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
			fmt.Sprintf("ship: git push failed (rc=%d); main is at %s: %v", exit, head, err),
			shiperr.GitRCKey, rc, "git_err", gitErr, shiperr.BranchKey, branch, "head", head)
	case SitePushOnly:
		return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
			fmt.Sprintf("ship --push-only: git push failed (rc=%d): %v", exit, err),
			shiperr.GitRCKey, rc, "git_err", gitErr, shiperr.BranchKey, branch)
	}
	return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
		fmt.Sprintf("ship: git push failed (rc=%d): %v", exit, err),
		shiperr.GitRCKey, rc, "git_err", gitErr, shiperr.BranchKey, branch)
}

// repairPush is the inline push-race repair (ADR-0039 §8 mode #4), run at
// the push site so the healed path still flows through the post-push
// verification and the binding. Returns nil when the push landed (retried,
// or already present on origin); otherwise the error to surface — the
// original transient rejection (declined, with Debug stamped), or a
// precondition reclassification (repair_outcome=needs-reaudit) when origin
// diverged and the only legitimate route is a re-audit on the new base.
// Never rebases, never force-pushes. The host's guards come in on the
// request; out reports what ran.
func (l *Landing) repairPush(ctx context.Context, req PushRequest, rejected *shiperr.ShipError, out *PushResult) error {
	if req.DryRun || req.RepairAttempted {
		return rejected
	}
	out.RepairAttempted = true
	emitLog(req.Log, "[ship] REPAIR: push rejected — fetching origin and probing for a fast-forward retry")
	originRef, head, probe := l.probeOrigin(ctx, req.Branch)
	if probe != "" {
		return l.declined(req, rejected, out, probe)
	}
	if originRef == head {
		out.RepairOutcome = RepairAlreadyPushed
		emitLog(req.Log, "[ship] REPAIR: origin already at HEAD — push race resolved itself")
		return nil
	}
	if l.IsAncestor(ctx, originRef, "HEAD") {
		s := l.streams()
		exit, pushErr := l.git(ctx, []string{"push", "origin", req.Branch}, s.Stdout, s.Stderr)
		if pushErr == nil && exit == 0 {
			out.RepairOutcome = RepairPushRetried
			emitLog(req.Log, "[ship] REPAIR: push retry after fetch succeeded (origin was an ancestor — fast-forward)")
			return nil
		}
		return l.declined(req, rejected, out, "push_retry")
	}
	// Origin diverged: a push would need a rebase/merge, which mutates the
	// audited tree. Reclassify so the recovery chain re-audits on the new
	// base — the local commit is preserved for a cheap re-land.
	out.RepairOutcome = RepairNeedsReaudit
	return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition,
		fmt.Sprintf("ship: push rejected and origin/%s diverged — audited tree must be re-audited on the new base (no auto-rebase; local commit preserved). Reconcile at a batch boundary with `evolve sync-main`, then complete the stranded push with `evolve ship --push-only`.", req.Branch),
		shiperr.BranchKey, req.Branch, "origin_ref", originRef, "head", head,
		"repair_attempted", string(shiperr.CodeGitPushRejected), shiperr.RepairOutcomeKey, string(RepairNeedsReaudit))
}

// probeOrigin is the repair's fetch and its two ref reads; probe names the
// first call that failed ("" when all three succeeded).
func (l *Landing) probeOrigin(ctx context.Context, branch string) (originRef, head, probe string) {
	if exit, err := l.git(ctx, []string{"fetch", "origin", branch}, io.Discard, io.Discard); err != nil || exit != 0 {
		return "", "", "fetch"
	}
	originRef, err := l.Capture(ctx, "rev-parse", "origin/"+branch)
	if err != nil {
		return "", "", "origin_ref"
	}
	head, err = l.Capture(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", "", "head"
	}
	return strings.TrimSpace(originRef), strings.TrimSpace(head), ""
}

// declined records a declined repair on the result and on the original
// error's Debug map, reports which probe declined, and returns that SAME
// error — the callers recover it through AsShipError.
func (l *Landing) declined(req PushRequest, rejected *shiperr.ShipError, out *PushResult, probe string) *shiperr.ShipError {
	out.RepairOutcome = RepairDeclined
	rejected.Debug["repair_attempted"] = string(shiperr.CodeGitPushRejected)
	rejected.Debug[shiperr.RepairOutcomeKey] = string(RepairDeclined)
	l.warn("Landing.Push", CodePushRepairDeclined, "push rejected; inline fetch + ff-retry declined at "+probe,
		map[string]string{shiperr.StepKey: stepPush, shiperr.BranchKey: req.Branch, "probe": probe})
	return rejected
}

// readHead is the post-push `rev-parse HEAD` (a blank-identifier capture at
// all three sites before the move): an error or an empty SHA is the WARN,
// the result's head stays empty and the ship proceeds exactly as before.
func (l *Landing) readHead(ctx context.Context) string {
	head, err := l.Capture(ctx, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	switch {
	case err != nil:
		l.warn("Landing.Push", CodeHeadReadFailed, "rev-parse HEAD after the push failed: "+err.Error(),
			map[string]string{shiperr.StepKey: stepPush, "ref": "HEAD", "err": err.Error()})
		return ""
	case head == "":
		l.warn("Landing.Push", CodeHeadReadFailed, "rev-parse HEAD after the push returned empty",
			map[string]string{shiperr.StepKey: stepPush, "ref": "HEAD"})
	}
	return head
}

// IsAncestor reports whether anc is an ancestor of desc (git merge-base):
// true only on exit 0 without a spawn error.
func (l *Landing) IsAncestor(ctx context.Context, anc, desc string) bool {
	exit, err := l.git(ctx, []string{"merge-base", "--is-ancestor", anc, desc}, io.Discard, io.Discard)
	return err == nil && exit == 0
}

// Capture runs `git <args>` and returns its stdout. A spawn error or an exit
// above 1 is a transient GIT_IO (rc=1 from git diff is "differences exist",
// not an error). It carries no step: the caller's step is unknown here.
func (l *Landing) Capture(ctx context.Context, args ...string) (string, error) {
	var buf strings.Builder
	exitCode, err := l.git(ctx, args, &buf, io.Discard)
	if err != nil {
		return "", shiperr.NewShipError(shiperr.CodeGitIO, shiperr.ShipClassTransient, shiperr.StageAtomicShip,
			fmt.Sprintf("ship: git %v: %v", args, err), "git_args", fmt.Sprintf("%v", args), "git_err", err.Error())
	}
	if exitCode > 1 {
		return "", shiperr.NewShipError(shiperr.CodeGitIO, shiperr.ShipClassTransient, shiperr.StageAtomicShip,
			fmt.Sprintf("ship: git %v exited %d", args, exitCode),
			"git_args", fmt.Sprintf("%v", args), shiperr.GitRCKey, fmt.Sprintf("%d", exitCode))
	}
	return buf.String(), nil
}
