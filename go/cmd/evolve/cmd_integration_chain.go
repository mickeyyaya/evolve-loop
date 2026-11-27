package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	waveRecordKey     = "integration.wave_record"
	waveRecordVersion = "wave-record/1"
	chainFieldDigest  = "wave_record_digest"
)

type ledgerChain struct {
	root    string
	signals *signalcenter.Center
}

func poolScope(pool int) stageevidence.ScopeKey {
	return stageevidence.ScopeKey{Kind: stageevidence.ScopePool, ID: pool}
}

func waveRecordVersionOf(string) (string, error) { return waveRecordVersion, nil }

func (c ledgerChain) Seal(ctx context.Context, pool int, digest string) error {
	ld := ledger.New(paths.EvolveDirOf(c.root), ledger.WithSignals(c.signals))
	site := newStageEvidenceSite(c.root, ld, c.signals)
	var fault error
	rec := site.openScope(poolScope(pool), waveRecordVersionOf, func(err error) { fault = errors.Join(fault, err) })
	rec.Record(waveRecordKey, map[string]string{chainFieldDigest: digest})
	rec.Close()
	if fault != nil {
		return fmt.Errorf("integration: record the digest of pool %d: %w", pool, fault)
	}
	if _, err := ld.SealStageEvidence(ctx, poolScope(pool), stageevidence.SealInput{}); err != nil {
		return fmt.Errorf("integration: seal the record of pool %d: %w", pool, err)
	}
	return nil
}

func (c ledgerChain) SealedAll(ctx context.Context) (map[int]string, error) {
	chain, err := stageLedgerReader(c.root).ReadChain(ctx, waveRecordKey)
	if errors.Is(err, stageevidence.ErrEmptyChain) {
		return map[int]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return lastDigests(chain.Records), nil
}

func lastDigests(records []stageevidence.Record) map[int]string {
	digests := map[int]string{}
	seqs := map[int]int{}
	for _, r := range records {
		scope := r.ScopeKey()
		last, seen := seqs[scope.ID]
		if r.Key != waveRecordKey || r.Kind != stageevidence.KindDecision || scope.Kind != stageevidence.ScopePool || (seen && r.Seq < last) {
			continue
		}
		seqs[scope.ID], digests[scope.ID] = r.Seq, r.Fields[chainFieldDigest]
	}
	return digests
}

func readinessOf(ctx context.Context, root string, chain integration.ChainPort) integration.Readiness {
	store := integration.FileWaveStore{Root: root}
	count := integration.RolloutCount{Root: root, Chain: chain, Gather: func(pool int) integrationStatusEvidence {
		return gatherEvidence(root, store, pool)
	}}
	return count.Count(ctx, store.Pools())
}
