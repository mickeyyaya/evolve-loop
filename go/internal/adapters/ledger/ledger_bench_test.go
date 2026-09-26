package ledger

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func benchEntry(seq int) core.LedgerEntry {
	return core.LedgerEntry{
		TS:             "2026-05-23T04:30:00Z",
		Cycle:          seq,
		Role:           "auditor",
		Kind:           "phase-complete",
		Model:          "claude-opus-4-7",
		ExitCode:       0,
		DurationS:      "12.345",
		ArtifactPath:   "/path/to/artifact",
		ArtifactSHA256: "abcdef1234567890",
		ChallengeToken: "ct-abc123",
		GitHEAD:        "deadbeef",
		TreeStateSHA:   "treesha123",
		WorkerCount:    1,
		Workers:        []string{"worker-0"},
	}
}

func BenchmarkAppendSerial(b *testing.B) {
	dir := b.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	l := New(evolveDir)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := l.Append(ctx, benchEntry(i)); err != nil {
			b.Fatal(err)
		}
	}
}

// 100 entries: a realistic cycle writes 30-80.
func BenchmarkVerify(b *testing.B) {
	dir := b.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	l := New(evolveDir)
	ctx := context.Background()

	const n = 100
	for i := 0; i < n; i++ {
		if err := l.Append(ctx, benchEntry(i)); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := l.Verify(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
