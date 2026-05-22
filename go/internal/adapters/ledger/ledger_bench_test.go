package ledger

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Phase 4 task #19: ledger append throughput. The ledger is the
// hot-path on every phase boundary — orchestrator, role-gate, and
// phase-gate all record entries. Bash baseline is roughly 10-20 ms per
// append (jq + tee + flock + sha256sum). Go target ≤ 0.6× per parent
// plan §6 item 8.
//
// Run: go test -bench=. -benchmem -run=^$ ./internal/adapters/ledger/

func benchEntry(seq int) core.LedgerEntry {
	return core.LedgerEntry{
		TS:              "2026-05-23T04:30:00Z",
		Cycle:           seq,
		Role:            "auditor",
		Kind:            "phase-complete",
		Model:           "claude-opus-4-7",
		ExitCode:        0,
		DurationS:       "12.345",
		ArtifactPath:    "/path/to/artifact",
		ArtifactSHA256:  "abcdef1234567890",
		ChallengeToken:  "ct-abc123",
		GitHEAD:         "deadbeef",
		TreeStateSHA:    "treesha123",
		WorkerCount:     1,
		Workers:         []string{"worker-0"},
	}
}

// BenchmarkAppendSerial measures single-writer append throughput.
// Each iteration writes one entry through the full hash-chain pipeline
// (readTip + marshal + sha256 + atomic append).
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

// BenchmarkVerify measures whole-chain verify time. Realistic cycles
// produce 30-80 entries; benchmarking with 100 entries.
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
