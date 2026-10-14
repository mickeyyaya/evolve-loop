package core

import (
	"context"
	"errors"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
)

func (o *Orchestrator) latestAuditedTree(ctx context.Context, runID string) (string, error) {
	entry, err := o.latestAuditEntry(ctx, runID)
	return entry.WorktreeTreeSHA, err
}

func (o *Orchestrator) latestAuditEntry(ctx context.Context, runID string) (auditledger.Entry, error) {
	it, err := o.ledger.Iter(ctx)
	if err != nil {
		return auditledger.Entry{}, err
	}
	defer func() { _ = it.Close() }()
	var rows []auditledger.Entry
	for {
		e, ok, err := it.Next()
		if err != nil {
			return auditledger.Entry{}, err
		}
		if !ok {
			break
		}
		row := auditledger.Entry{Role: e.Role, Kind: e.Kind, RunID: e.RunID, GitHEAD: e.GitHEAD, WorktreeTreeSHA: e.WorktreeTreeSHA, WorktreeBaseSHA: e.WorktreeBaseSHA, ArtifactSHA256: e.ArtifactSHA256}
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
