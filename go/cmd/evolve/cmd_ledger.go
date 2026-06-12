package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
)

// runLedger implements `evolve ledger <subcommand>`. Subcommands:
//
//	verify [--evolve-dir DIR] [--deep]   walk the chain; exit 2 on break
//	                                     (--deep reconstructs sealed
//	                                     segments + live tail, L3.3)
//	seal   [--evolve-dir DIR] [--keep N] move all but the newest N lines
//	                                     into ledger-segments/*.jsonl.gz
//	tail   [--evolve-dir DIR] [--n N]    print the last N entries as JSONL
func runLedger(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve ledger: missing subcommand (try: verify | seal | tail)")
		return 10
	}
	switch args[0] {
	case "verify":
		return runLedgerVerify(args[1:], stderr)
	case "seal":
		return runLedgerSeal(args[1:], stderr)
	case "tail":
		return runLedgerTail(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve ledger: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runLedgerVerify(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve ledger verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDir string
	var deep bool
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.BoolVar(&deep, "deep", false, "reconstruct sealed segments + live tail and verify end-to-end")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	l := ledger.New(evolveDir)
	verify, scope := l.Verify, "chain intact"
	if deep {
		verify, scope = l.VerifyDeep, "chain intact incl. sealed segments"
	}
	if err := verify(context.Background()); err != nil {
		fmt.Fprintf(stderr, "[ledger] BROKEN: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "[ledger] OK: %s (%s/ledger.jsonl)\n", scope, evolveDir)
	return 0
}

func runLedgerSeal(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve ledger seal", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDir string
	var keep int
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.IntVar(&keep, "keep", 500, "live-tail lines to keep unsealed (min 1)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	l := ledger.New(evolveDir)
	if err := l.Seal(context.Background(), keep); err != nil {
		fmt.Fprintf(stderr, "[ledger] seal failed: %v\n", err)
		return 1
	}
	// Always leave with a deep-verified chain — a seal that cannot verify
	// must surface immediately, not at the next audit.
	if err := l.VerifyDeep(context.Background()); err != nil {
		fmt.Fprintf(stderr, "[ledger] seal completed but deep verify FAILED: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "[ledger] OK: sealed (keep=%d) and deep-verified (%s)\n", keep, evolveDir)
	return 0
}

func runLedgerTail(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve ledger tail", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDir string
	var n int
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.IntVar(&n, "n", 10, "number of entries to print (0 = all)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	l := ledger.New(evolveDir)
	it, err := l.Iter(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "[ledger] tail: %v\n", err)
		return 1
	}
	defer func() { _ = it.Close() }()
	var collected []any
	for {
		entry, ok, err := it.Next()
		if err != nil {
			fmt.Fprintf(stderr, "[ledger] tail: %v\n", err)
			return 1
		}
		if !ok {
			break
		}
		collected = append(collected, entry)
	}
	start := 0
	if n > 0 && len(collected) > n {
		start = len(collected) - n
	}
	for _, e := range collected[start:] {
		buf, err := json.Marshal(e)
		if err != nil {
			fmt.Fprintf(stderr, "[ledger] tail: marshal: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", buf)
	}
	return 0
}
