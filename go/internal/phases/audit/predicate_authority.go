package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

func preservePredicateCandidate(path string) error {
	f, err := os.CreateTemp(filepath.Dir(path), "acs-verdict.candidate.*.json")
	if err != nil {
		return fmt.Errorf("preserve predicate candidate: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(path, name); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("retire predicate candidate: %w", err)
	}
	return nil
}

func predicateTreeFor(req core.PhaseRequest) (string, error) {
	if req.Cycle <= 0 || req.RunID == "" || req.AuditRound <= 0 || req.Worktree == "" {
		return "", fmt.Errorf("host predicate audit requires cycle, run, round, and worktree identity")
	}
	snap, err := treefence.Take(context.Background(), req.Worktree)
	if err != nil {
		return "", fmt.Errorf("capture predicate tree: %w", err)
	}
	tracked, err := treefence.TakeTracked(context.Background(), req.Worktree)
	if err != nil {
		return "", fmt.Errorf("capture predicate ship tree: %w", err)
	}
	if snap.Tree != tracked.Tree {
		return "", fmt.Errorf("predicate execution tree includes undeclared inputs absent from the ship tree; explicitly stage intended Build files or remove the inputs, then re-run Audit")
	}
	return snap.Tree, nil
}

func beginPredicateEvidence(req core.PhaseRequest) (func() error, error) {
	beforeTree, err := predicateTreeFor(req)
	if err != nil {
		return nil, err
	}
	return func() error { return sealPredicateEvidence(req, beforeTree) }, nil
}

func sealPredicateEvidence(req core.PhaseRequest, beforeTree string) error {
	afterTree, err := predicateTreeFor(req)
	if err != nil {
		return err
	}
	if beforeTree != afterTree {
		return fmt.Errorf("worktree changed during host verification; re-run Audit on the restored Build tree")
	}
	raw, err := os.ReadFile(filepath.Join(req.Workspace, acssuite.VerdictFilename))
	if err != nil {
		return err
	}
	return acssuite.SealEvidence(filepath.Join(req.Workspace, "audit-report.md"), raw, acssuite.EvidenceIdentity{
		Cycle: req.Cycle, RunID: req.RunID, Round: req.AuditRound, TreeSHA: afterTree,
	})
}
