package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeResumeCarrierPended signalcenter.Code = "ORCHESTRATOR_RESUME_CARRIER_PENDED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeResumeCarrierPended, "a resume found this cycle's Evolve-Carrier commit at the lane's HEAD (a crash between the fleet rebase and the pend) and pended it on its fork point before any phase ran, so ship binds the pending change and no re-audit sees an empty diff")
}

func (o *Orchestrator) healCarrierAtHead(ctx context.Context, projectRoot string, cycle int, cs CycleState) error {
	if cs.ActiveWorktree == "" || cs.RunID == "" || inPlaceWorktree(cs.ActiveWorktree, projectRoot) {
		return nil
	}
	label := fmt.Sprintf("cycle-%d/%s", cycle, cs.RunID)
	carried, err := carrierAtHead(ctx, cs.ActiveWorktree, label, gitCapture)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d resume cannot read the lane HEAD for a carrier (%v); a carrier left there stays committed\n", cycle, err)
		return nil
	}
	if !carried {
		return nil
	}
	if rebasing, err := rebaseInProgress(ctx, cs.ActiveWorktree, gitCapture); err != nil || rebasing {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d resume leaves the carrier %s at HEAD: a rebase is in progress or unreadable (%v)\n", cycle, label, err)
		return nil
	}
	onto, err := pendOnForkPoint(ctx, cs.ActiveWorktree, gitCapture)
	if err != nil {
		return fmt.Errorf("pend the carrier %s: %w", label, err)
	}
	reason := fmt.Sprintf("pended the carrier %s at HEAD on its fork point %s", label, onto)
	fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d resume %s\n", cycle, reason)
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.healCarrierAtHead",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeResumeCarrierPended, Reason: reason,
	})
	return nil
}

func carrierAtHead(ctx context.Context, worktree, label string, git gitFn) (bool, error) {
	trailers, err := gitStdout(ctx, git, worktree, "log", "-1", "--format=%(trailers:key=Evolve-Carrier,valueonly)")
	if err != nil {
		return false, err
	}
	return slices.Contains(strings.Fields(trailers), label), nil
}

func pendOnForkPoint(ctx context.Context, worktree string, git gitFn) (string, error) {
	onto, err := forkPoint(ctx, git, worktree)
	if err != nil {
		return "", err
	}
	if _, err := gitStdout(ctx, git, worktree, "reset", "--soft", onto); err != nil {
		return "", err
	}
	return onto, nil
}

func rebaseInProgress(ctx context.Context, worktree string, git gitFn) (bool, error) {
	for _, state := range []string{"rebase-merge", "rebase-apply"} {
		p, err := gitStdout(ctx, git, worktree, "rev-parse", "--git-path", state)
		if err != nil {
			return false, err
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(worktree, p)
		}
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}
