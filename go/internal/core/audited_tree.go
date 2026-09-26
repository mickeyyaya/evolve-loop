package core

import (
	"context"
	"errors"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
)

// latestAuditedTree returns the worktree tree bound by the auditor row ship binds (auditledger.BindRun
// over this run's rows, newest first), or "" when that row carries no tree or the run has none.
func (o *Orchestrator) latestAuditedTree(ctx context.Context, runID string) (string, error) {
	it, err := o.ledger.Iter(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = it.Close() }() // read-only iteration; a close error is not actionable
	var rows []auditledger.Entry
	for {
		e, ok, err := it.Next()
		if err != nil {
			return "", err
		}
		if !ok {
			break
		}
		row := auditledger.Entry{Role: e.Role, Kind: e.Kind, RunID: e.RunID, GitHEAD: e.GitHEAD, WorktreeTreeSHA: e.WorktreeTreeSHA}
		if auditledger.IsAuditorRow(row) {
			rows = append(rows, row)
		}
	}
	slices.Reverse(rows)
	entry, err := auditledger.BindRun(rows, runID)
	if errors.Is(err, auditledger.ErrNoAuditorForRun) {
		return "", nil
	}
	return entry.WorktreeTreeSHA, err
}
