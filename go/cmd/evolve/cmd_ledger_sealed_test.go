package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLedgerVerify_ASealedLedgerVerifiesAndNamesTheSegmentItResumedFrom(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	l := ledger.New(evolveDir)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		if err := l.Append(ctx, core.LedgerEntry{Cycle: i, Role: "builder", Kind: "phase_complete", Message: fmt.Sprintf("entry-%d", i)}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	if err := l.Seal(ctx, 4); err != nil {
		t.Fatalf("Seal: %v", err)
	}

	var stdout, stderr bytes.Buffer
	rc := runLedger([]string{"verify", "--evolve-dir", evolveDir}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("evolve ledger verify on a sealed, untampered ledger: rc=%d, stderr=%q", rc, stderr.String())
	}
	if out := stderr.String(); !strings.Contains(out, "newest sealed segment's last line entry_seq=7") || strings.Contains(out, "operator-adjudicated") {
		t.Errorf("the verdict must name the sealed boundary it resumed from, not an operator epoch anchor: %q", out)
	}
}

func TestLedgerAnchor_AfterASealRecordsTheAnchorAndExitsZero(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		seq  string
	}{
		{"anchor on a live tail line", "10"},
		{"anchor on a line sealed into a segment", "3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evolveDir := t.TempDir()
			l := ledger.New(evolveDir)
			ctx := context.Background()
			for i := 0; i < 12; i++ {
				if err := l.Append(ctx, core.LedgerEntry{Cycle: i, Role: "builder", Kind: "phase_complete", Message: fmt.Sprintf("entry-%d", i)}); err != nil {
					t.Fatalf("Append %d: %v", i, err)
				}
			}
			if err := l.Seal(ctx, 4); err != nil {
				t.Fatalf("Seal: %v", err)
			}

			var stdout, stderr bytes.Buffer
			rc := runLedger([]string{"anchor", tc.seq, "--evolve-dir", evolveDir, "--note", "anchor after seal"}, nil, &stdout, &stderr)

			if rc != 0 {
				t.Fatalf("evolve ledger anchor %s after a seal: rc=%d, stderr=%q", tc.seq, rc, stderr.String())
			}
			if _, err := os.Stat(filepath.Join(evolveDir, "ledger-anchor.json")); err != nil {
				t.Errorf("anchor not recorded: %v", err)
			}
		})
	}
}
