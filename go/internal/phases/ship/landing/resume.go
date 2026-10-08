package landing

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

type Witness struct {
	Cycle                 int
	RunID                 string
	Journaled             bool
	AuditPassed           bool
	AuditArtifactSHA256   string
	AuditedTree           string
	LaneTip               string
	ExplanationViewSHA256 string
}

type Verdict struct {
	Declined string
	OnOrigin bool
}

func (l *Landing) Resume(ctx context.Context, in Intent, w Witness) Verdict {
	if why := unproven(in, w); why != "" {
		return Verdict{Declined: why}
	}
	return l.Admit(ctx, in)
}

func unproven(in Intent, w Witness) string {
	switch {
	case in.AuditArtifactSHA256 == "" || in.AuditedTree == "" || in.LaneTree == "":
		return "the intent binds no audit"
	case !w.Journaled:
		return fmt.Sprintf("the ship journal does not hold the prepared commit %s", in.CommitSHA)
	case w.Cycle != in.Cycle || w.RunID != in.RunID:
		return fmt.Sprintf("the intent is cycle %d run %q, not cycle %d run %q", in.Cycle, in.RunID, w.Cycle, w.RunID)
	case !w.AuditPassed:
		return "the newest audit of the run is not a PASS"
	case w.AuditArtifactSHA256 != in.AuditArtifactSHA256 || w.AuditedTree != in.AuditedTree:
		return fmt.Sprintf("a newer audit (%s of tree %s) supersedes the landing's audit (%s of tree %s)", w.AuditArtifactSHA256, w.AuditedTree, in.AuditArtifactSHA256, in.AuditedTree)
	case w.LaneTip != in.CommitSHA:
		return fmt.Sprintf("the lane tip moved to %s off the prepared commit %s", w.LaneTip, in.CommitSHA)
	case w.ExplanationViewSHA256 != in.ExplanationViewSHA256:
		return "the sealed Build explanation changed after ship prepared the landing"
	}
	return ""
}

func (l *Landing) Admit(ctx context.Context, in Intent) Verdict {
	if tree, err := l.Capture(ctx, "rev-parse", in.CommitSHA+"^{tree}"); err != nil || strings.TrimSpace(tree) != in.CommitTree {
		return Verdict{Declined: fmt.Sprintf("the prepared commit %s does not hold the tree %s", in.CommitSHA, in.CommitTree)}
	}
	l.refreshOrigin(ctx, in.Branch)
	origin, err := l.Capture(ctx, "rev-parse", "origin/"+in.Branch)
	if err != nil {
		return Verdict{Declined: fmt.Sprintf("origin/%s is unreadable: %v", in.Branch, err)}
	}
	origin = strings.TrimSpace(origin)
	switch {
	case l.IsAncestor(ctx, in.CommitSHA, origin):
		return Verdict{OnOrigin: true}
	case !l.IsAncestor(ctx, origin, in.CommitSHA):
		return Verdict{Declined: fmt.Sprintf("origin/%s (%s) diverged from the prepared commit %s", in.Branch, origin, in.CommitSHA)}
	case !l.IsAncestor(ctx, in.Branch, in.CommitSHA):
		return Verdict{Declined: fmt.Sprintf("%s is not an ancestor of the prepared commit %s", in.Branch, in.CommitSHA)}
	}
	return Verdict{}
}

func (l *Landing) refreshOrigin(ctx context.Context, branch string) {
	if exit, err := l.git(ctx, []string{"fetch", "origin", branch}, io.Discard, io.Discard); err != nil || exit != 0 {
		l.warn("Landing.Resume", CodeOriginFetchFailed, fmt.Sprintf("fetch origin %s before a resume failed (exit=%d, err=%v); the resume decides on the last known origin ref", branch, exit, err),
			map[string]string{shiperr.StepKey: stepResume, shiperr.BranchKey: branch, shiperr.GitRCKey: fmt.Sprintf("%d", exit), "git_err": errText(err)})
	}
}
