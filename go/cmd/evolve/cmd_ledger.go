package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/ledger"
)

// runLedger implements `evolve ledger <subcommand>`. Subcommands:
//
//	verify [--evolve-dir DIR] [--deep]   walk the chain; exit 2 on break
//	                                     (--deep reconstructs sealed
//	                                     segments + live tail, L3.3)
//	seal   [--evolve-dir DIR] [--keep N] move all but the newest N lines
//	                                     into ledger-segments/*.jsonl.gz
//	tail   [--evolve-dir DIR] [--n N]    print the last N entries as JSONL
//	anchor <seq> [--evolve-dir DIR] [--note S]  record a non-destructive
//	                                     epoch-anchor at entry_seq=<seq>
//	                                     (ledger-1740; OPERATOR sign-off)
func runLedger(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve ledger: missing subcommand (try: verify | seal | tail | anchor)")
		return 10
	}
	switch args[0] {
	case "verify":
		return runLedgerVerify(args[1:], stderr)
	case "seal":
		return runLedgerSeal(args[1:], stderr)
	case "tail":
		return runLedgerTail(args[1:], stdout, stderr)
	case "anchor":
		return runLedgerAnchor(args[1:], stderr)
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

// runLedgerAnchor records a non-destructive epoch-anchor (ADR-0048; the
// ledger-1740 disposition). An OPERATOR action: it declares the pre-anchor
// prefix trusted-as-preserved and validates the chain strictly forward from the
// anchored line. Always re-verifies after, so an anchor that does not green the
// chain is surfaced immediately (rc 2), not at the next audit.
func runLedgerAnchor(args []string, stderr io.Writer) int {
	// <entry_seq> is the first positional, BEFORE flags: Go's flag.Parse stops
	// at the first non-flag token, so a flag-first layout (`--note x 42`) would
	// be the only one that works otherwise — backwards from the usage. Take the
	// positional explicitly, then parse the remaining flags.
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve ledger anchor: usage: evolve ledger anchor <entry_seq> [--note S] [--evolve-dir DIR]")
		return 10
	}
	seq, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "evolve ledger anchor: <entry_seq> must be an integer: %v\n", err)
		return 10
	}
	fs := flag.NewFlagSet("evolve ledger anchor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDir, note string
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.StringVar(&note, "note", "", "operator note recorded with the anchor (why this epoch is trusted)")
	if err := fs.Parse(args[1:]); err != nil {
		return 10
	}
	l := ledger.New(evolveDir)
	if err := l.Anchor(context.Background(), seq, note); err != nil {
		fmt.Fprintf(stderr, "[ledger] anchor failed: %v\n", err)
		return 1
	}
	if err := l.Verify(context.Background()); err != nil {
		fmt.Fprintf(stderr, "[ledger] anchor recorded at entry_seq=%d but the chain still does NOT verify forward: %v\n", seq, err)
		return 2
	}
	fmt.Fprintf(stderr, "[ledger] OK: epoch-anchored at entry_seq=%d (%s/ledger-anchor.json). "+
		"Pre-anchor history is PRESERVED and no longer chain-validated — trusted by this operator action; the chain verifies strictly forward from here.\n", seq, evolveDir)
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
