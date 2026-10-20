package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func ledgerSealMode(gcMode string, pol gc.Policy) string {
	if gcMode == "enforce" && pol.LedgerSealEnforced() {
		return "enforce"
	}
	return "shadow"
}

func sealLedgerGC(cfg loopConfig, mode string, keep int, stderr io.Writer) {
	lines, err := countLines(filepath.Join(cfg.EvolveDir, "ledger.jsonl"))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "[gc] WARN: ledger seal skipped: %v\n", err)
		}
		return
	}
	if lines <= 2*keep {
		return
	}
	if mode != "enforce" {
		fmt.Fprintf(stderr, "[gc] shadow: the ledger holds %d lines; enforce seals all but the last %d\n", lines, keep)
		return
	}
	l := ledger.New(cfg.EvolveDir)
	if err := l.Seal(context.Background(), keep); err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: ledger seal failed: %v\n", err)
		return
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: ledger sealed but deep verify FAILED: %v\n", err)
		return
	}
	fmt.Fprintf(stderr, "[gc] enforce: sealed the ledger (keep=%d) and deep-verified it\n", keep)
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 64<<10)
	lines := 0
	for {
		n, err := f.Read(buf)
		lines += bytes.Count(buf[:n], []byte("\n"))
		if err == io.EOF {
			return lines, nil
		}
		if err != nil {
			return lines, err
		}
	}
}
