package ledger

import (
	"context"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	benchLedgerEntries = 5000
	benchSealedScopes  = 40
)

func BenchmarkStageChainReaderOverALargeLedger(b *testing.B) {
	dir := b.TempDir()
	l := New(dir)
	ctx := context.Background()
	for i := range benchLedgerEntries {
		if err := l.Append(ctx, core.LedgerEntry{TS: "2026-11-22T00:00:00Z", Cycle: i, Role: "build", Kind: "phase", Message: strconv.Itoa(i)}); err != nil {
			b.Fatal(err)
		}
	}
	for id := 1; id <= benchSealedScopes; id++ {
		rec := stageevidence.NewRecorder(stageevidence.RecorderConfig{
			EvolveDir: dir, Scope: stageevidence.CycleScope(id), Session: "s", Build: "rev", Lock: func(string) (func(), error) { return func() {}, nil },
			Version: func(string) (string, error) { return evVersion, nil },
			OnFault: func(err error) { b.Fatal(err) },
		})
		rec.Record(evKeyA, map[string]string{"would_stop": "false"})
		rec.Close()
		if _, err := l.SealStageEvidence(ctx, stageevidence.CycleScope(id), stageevidence.SealInput{Exit: "completed"}); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for range b.N {
		if _, err := l.StageChain().ReadChain(ctx, evKeyA); err != nil {
			b.Fatal(err)
		}
	}
}
