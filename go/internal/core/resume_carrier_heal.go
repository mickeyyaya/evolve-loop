package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeResumeCarrierPended signalcenter.Code = "ORCHESTRATOR_RESUME_CARRIER_PENDED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeResumeCarrierPended, "a resume found this cycle's Evolve-Carrier commit at the lane's HEAD (a crash between the fleet rebase and the pend) and pended it on its fork point before any phase ran, so the resumed cycle sees the change pending and never re-audits an empty diff")
}

func (o *Orchestrator) healCarrierAtHead(ctx context.Context, projectRoot string, cycle int, cs CycleState, git gitFn) error {
	if cs.ActiveWorktree == "" || cs.RunID == "" || inPlaceWorktree(cs.ActiveWorktree, projectRoot) {
		return nil
	}
	checkout, err := isGitCheckout(cs.ActiveWorktree)
	if err != nil || !checkout {
		return err
	}
	label := fmt.Sprintf("cycle-%d/%s", cycle, cs.RunID)
	carried, err := carrierAtHead(ctx, cs.ActiveWorktree, label, git)
	if err != nil {
		return fmt.Errorf("read the HEAD of lane %s: %w", cs.ActiveWorktree, err)
	}
	if !carried {
		return nil
	}
	rebasing, err := rebaseInProgress(ctx, cs.ActiveWorktree, git)
	if err != nil {
		return fmt.Errorf("check lane %s for a rebase in progress: %w", cs.ActiveWorktree, err)
	}
	if rebasing {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d resume leaves the carrier %s at HEAD: a rebase is in progress\n", cycle, label)
		return nil
	}
	onto, err := pendOnForkPoint(ctx, cs.ActiveWorktree, git)
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

func isGitCheckout(worktree string) (bool, error) {
	_, err := os.Stat(filepath.Join(worktree, ".git"))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the git checkout of lane %s: %w", worktree, err)
	}
	return true, nil
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
