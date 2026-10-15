package landing

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

type PushSite uint8

const (
	SiteDirect PushSite = iota
	SiteWorktree
	SitePushOnly
)

type PushRequest struct {
	Branch          string
	Commit          string
	Site            PushSite
	DryRun          bool
	RepairAttempted bool
	Log             func(string)
}

type PushResult struct {
	Head            string
	RepairAttempted bool
	RepairOutcome   RepairOutcome
}

type RepairOutcome string

const (
	RepairDeclined      RepairOutcome = "declined"
	RepairAlreadyPushed RepairOutcome = "already-pushed"
	RepairPushRetried   RepairOutcome = "push-retried"
	RepairNeedsReaudit  RepairOutcome = "needs-reaudit"
)

var transportBackoff = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

type pushAttempt struct {
	exit   int
	err    error
	stderr string
}

func (a pushAttempt) failed() bool { return a.err != nil || a.exit != 0 }

func (a pushAttempt) detail() string {
	switch {
	case a.err != nil:
		return ": " + a.err.Error()
	case strings.TrimSpace(a.stderr) != "":
		return ": " + stderrDetail(a.stderr)
	}
	return ""
}

func (l *Landing) Push(ctx context.Context, req PushRequest) (PushResult, error) {
	var out PushResult
	if req.Site == SiteWorktree && req.Commit == "" {
		return out, fail(stepPush, shiperr.CodeArgs, shiperr.ShipClassConfig, "ship: a worktree push names no commit, so ship pushes nothing",
			shiperr.BranchKey, req.Branch)
	}
	if attempt := l.send(ctx, req); attempt.failed() {
		if rerr := l.answerRejection(ctx, req, attempt, &out); rerr != nil {
			return out, rerr
		}
	}
	out.Head = l.landedHead(ctx, req)
	return out, nil
}

func (l *Landing) send(ctx context.Context, req PushRequest) pushAttempt {
	s := l.streams()
	var captured strings.Builder
	stderr := io.Writer(&captured)
	if s.Stderr != nil {
		stderr = io.MultiWriter(s.Stderr, &captured)
	}
	exit, err := l.git(ctx, pushArgs(req), s.Stdout, stderr)
	return pushAttempt{exit: exit, err: err, stderr: captured.String()}
}

func pushArgs(req PushRequest) []string {
	if req.Site == SiteWorktree {
		return []string{"push", "origin", req.Commit + ":refs/heads/" + req.Branch}
	}
	return []string{"push", "origin", req.Branch}
}

func tipRef(req PushRequest) string {
	if req.Site == SiteWorktree {
		return req.Commit
	}
	return "HEAD"
}

func (l *Landing) landedHead(ctx context.Context, req PushRequest) string {
	if req.Site == SiteWorktree {
		return req.Commit
	}
	return l.readHead(ctx)
}

func (l *Landing) rejection(req PushRequest, a pushAttempt) *shiperr.ShipError {
	rc, gitErr := fmt.Sprintf("%d", a.exit), errText(a.err)
	switch req.Site {
	case SiteWorktree:
		return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
			fmt.Sprintf("ship: git push of %s to origin/%s failed (rc=%d)%s; ship did not move %s", req.Commit, req.Branch, a.exit, a.detail(), req.Branch),
			shiperr.GitRCKey, rc, "git_err", gitErr, shiperr.BranchKey, req.Branch, "commit", req.Commit)
	case SitePushOnly:
		return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
			fmt.Sprintf("ship --push-only: git push failed (rc=%d)%s", a.exit, a.detail()),
			shiperr.GitRCKey, rc, "git_err", gitErr, shiperr.BranchKey, req.Branch)
	}
	return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
		fmt.Sprintf("ship: git push failed (rc=%d)%s", a.exit, a.detail()),
		shiperr.GitRCKey, rc, "git_err", gitErr, shiperr.BranchKey, req.Branch)
}

func (l *Landing) refused(req PushRequest, a pushAttempt) *shiperr.ShipError {
	return fail(stepPush, shiperr.CodeGitPushPolicyRefused, shiperr.ShipClassPrecondition,
		fmt.Sprintf("ship: origin refused the push to %s by policy; ship does not retry a policy refusal%s", req.Branch, a.detail()),
		shiperr.GitRCKey, fmt.Sprintf("%d", a.exit), "git_err", errText(a.err), shiperr.BranchKey, req.Branch)
}

func (l *Landing) answerRejection(ctx context.Context, req PushRequest, a pushAttempt, out *PushResult) error {
	kind := classifyPush(a.stderr)
	if kind == pushPolicy {
		return l.refused(req, a)
	}
	rejected := l.rejection(req, a)
	if req.DryRun || req.RepairAttempted {
		return rejected
	}
	out.RepairAttempted = true
	if kind == pushTransport {
		return l.retryTransport(ctx, req, rejected, out)
	}
	return l.repairPush(ctx, req, rejected, out)
}

func (l *Landing) retryTransport(ctx context.Context, req PushRequest, rejected *shiperr.ShipError, out *PushResult) error {
	emitLog(req.Log, "[ship] REPAIR: the push failed with a transport or server error; ship retries the identical push after a backoff")
	for _, wait := range transportBackoff {
		l.sleep(wait)
		a := l.send(ctx, req)
		if !a.failed() {
			out.RepairOutcome = RepairPushRetried
			emitLog(req.Log, fmt.Sprintf("[ship] REPAIR: push retry after a %s backoff succeeded", wait))
			return nil
		}
		switch classifyPush(a.stderr) {
		case pushPolicy:
			out.RepairOutcome = RepairDeclined
			return l.refused(req, a)
		case pushRace:
			return l.declined(req, rejected, out, "transport_retry")
		}
	}
	return l.declined(req, rejected, out, "transport_retry")
}

func (l *Landing) repairPush(ctx context.Context, req PushRequest, rejected *shiperr.ShipError, out *PushResult) error {
	emitLog(req.Log, "[ship] REPAIR: push rejected — fetching origin and probing for a fast-forward retry")
	originRef, head, probe := l.probeOrigin(ctx, req)
	if probe != "" {
		return l.declined(req, rejected, out, probe)
	}
	if originRef == head {
		out.RepairOutcome = RepairAlreadyPushed
		emitLog(req.Log, "[ship] REPAIR: origin already at HEAD — push race resolved itself")
		return nil
	}
	if l.IsAncestor(ctx, originRef, tipRef(req)) {
		if l.send(ctx, req).failed() {
			return l.declined(req, rejected, out, "push_retry")
		}
		out.RepairOutcome = RepairPushRetried
		emitLog(req.Log, "[ship] REPAIR: push retry after fetch succeeded (origin was an ancestor — fast-forward)")
		return nil
	}
	out.RepairOutcome = RepairNeedsReaudit
	return divergedOrigin(req, originRef, head)
}

func divergedOrigin(req PushRequest, originRef, head string) *shiperr.ShipError {
	msg := fmt.Sprintf("ship: push rejected and origin/%s diverged — audited tree must be re-audited on the new base (no auto-rebase; local commit preserved). Reconcile at a batch boundary with `evolve sync-main`, then complete the stranded push with `evolve ship --push-only`.", req.Branch)
	if req.Site == SiteWorktree {
		msg = fmt.Sprintf("ship: origin rejected the push of %s because origin/%s diverged; Audit must run again on the new base (ship does not rebase or force-push)", req.Commit, req.Branch)
	}
	return fail(stepPush, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition, msg,
		shiperr.BranchKey, req.Branch, "origin_ref", originRef, "head", head,
		"repair_attempted", string(shiperr.CodeGitPushRejected), shiperr.RepairOutcomeKey, string(RepairNeedsReaudit))
}

func (l *Landing) probeOrigin(ctx context.Context, req PushRequest) (originRef, head, probe string) {
	if exit, err := l.git(ctx, []string{"fetch", "origin", req.Branch}, io.Discard, io.Discard); err != nil || exit != 0 {
		return "", "", "fetch"
	}
	originRef, err := l.Capture(ctx, "rev-parse", "origin/"+req.Branch)
	if err != nil {
		return "", "", "origin_ref"
	}
	if req.Site == SiteWorktree {
		return strings.TrimSpace(originRef), req.Commit, ""
	}
	if head, err = l.Capture(ctx, "rev-parse", "HEAD"); err != nil {
		return "", "", "head"
	}
	return strings.TrimSpace(originRef), strings.TrimSpace(head), ""
}

func (l *Landing) declined(req PushRequest, rejected *shiperr.ShipError, out *PushResult, probe string) *shiperr.ShipError {
	out.RepairOutcome = RepairDeclined
	rejected.Debug["repair_attempted"] = string(shiperr.CodeGitPushRejected)
	rejected.Debug[shiperr.RepairOutcomeKey] = string(RepairDeclined)
	l.warn("Landing.Push", CodePushRepairDeclined, "the remote rejected the push; the inline repair declined at "+probe,
		map[string]string{shiperr.StepKey: stepPush, shiperr.BranchKey: req.Branch, "probe": probe})
	return rejected
}

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

func (l *Landing) IsAncestor(ctx context.Context, anc, desc string) bool {
	exit, err := l.git(ctx, []string{"merge-base", "--is-ancestor", anc, desc}, io.Discard, io.Discard)
	return err == nil && exit == 0
}

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
