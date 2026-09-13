// cmd_signals.go — `evolve signals codes generate|check`: projects the Signal
// Center code registry (every module's RegisterCode, all linked into this
// binary) into the GENERATED region of docs/architecture/signal-codes.md,
// exactly like `evolve flags generate|check` projects the flag registry.
// `check` exits 2 on drift; a cmd/evolve test runs it, so CI gates an
// undocumented or re-documented code (ADR-0101 S2, design §5.4).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

const (
	signalCodesBegin = "<!-- GENERATED:signal-codes BEGIN — do not edit by hand; run `evolve signals codes generate` -->"
	signalCodesEnd   = "<!-- GENERATED:signal-codes END -->"
)

func runSignals(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "codes" {
		fmt.Fprintln(stderr, "usage: evolve signals codes <generate|check>")
		return 10
	}
	docPath := filepath.Join(sourceRoot(), "docs", "architecture", "signal-codes.md")
	switch args[1] {
	case "generate":
		return signalCodesRun(docPath, true, stdout, stderr)
	case "check":
		return signalCodesRun(docPath, false, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q (want generate|check)\n", args[1])
		return 10
	}
}

// signalCodesRun renders the registry into the doc's marked region and either
// writes it (generate) or compares against disk (check; exit 2 on drift).
func signalCodesRun(docPath string, write bool, stdout, stderr io.Writer) int {
	doc, err := os.ReadFile(docPath)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", docPath, err)
		return 1
	}
	block := signalCodesBegin + "\n\n" + signalcenter.RenderCodes() + signalCodesEnd
	next, err := skillcheck.SpliceMarkedRegion(string(doc), block, signalCodesBegin, signalCodesEnd, "")
	if err != nil {
		fmt.Fprintf(stderr, "splice %s: %v\n", docPath, err)
		return 1
	}
	count := registeredCodeCount()
	if write {
		if next == string(doc) {
			fmt.Fprintf(stdout, "signals: codes up to date (%d codes)\n", count)
			return 0
		}
		if werr := os.WriteFile(docPath, []byte(next), 0o644); werr != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", docPath, werr)
			return 1
		}
		fmt.Fprintf(stdout, "signals: regenerated %d codes in %s\n", count, docPath)
		return 0
	}
	if next != string(doc) {
		fmt.Fprintf(stderr, "signals: %s is stale vs the code registry — run `evolve signals codes generate`\n", docPath)
		return 2
	}
	fmt.Fprintf(stdout, "signals: %d codes in sync\n", count)
	return 0
}

func registeredCodeCount() int {
	n := 0
	for _, docs := range signalcenter.RegisteredCodes() {
		n += len(docs)
	}
	return n
}
