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
	entry, err := o.latestAuditEntry(ctx, runID)
	return entry.WorktreeTreeSHA, err
}

// latestAuditEntry is the auditor row ship binds, with the tree and the artifact it names; a run without one
// is an empty entry, not an error.
func (o *Orchestrator) latestAuditEntry(ctx context.Context, runID string) (auditledger.Entry, error) {
	it, err := o.ledger.Iter(ctx)
	if err != nil {
		return auditledger.Entry{}, err
	}
	defer func() { _ = it.Close() }() // read-only iteration; a close error is not actionable
	var rows []auditledger.Entry
	for {
		e, ok, err := it.Next()
		if err != nil {
			return auditledger.Entry{}, err
		}
		if !ok {
			break
		}
		row := auditledger.Entry{Role: e.Role, Kind: e.Kind, RunID: e.RunID, GitHEAD: e.GitHEAD, WorktreeTreeSHA: e.WorktreeTreeSHA, ArtifactSHA256: e.ArtifactSHA256}
		if auditledger.IsAuditorRow(row) {
			rows = append(rows, row)
		}
	}
	slices.Reverse(rows)
	entry, err := auditledger.BindRun(rows, runID)
	if errors.Is(err, auditledger.ErrNoAuditorForRun) {
		return auditledger.Entry{}, nil
	}
	return entry, err
}
