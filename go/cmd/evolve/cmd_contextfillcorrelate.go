// cmd_contextfillcorrelate.go wires internal/contextfillcorrelate into the
// top-level dispatch table as `evolve context-fill correlate`. Without this
// file the join would be a library with no production caller — the exact dead
// seam the caller-proof floor exists to catch.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfillcorrelate"
)

// runContextFill implements `evolve context-fill <correlate>`.
func runContextFill(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve context-fill: missing subcommand (correlate)")
		return 10
	}
	switch args[0] {
	case "correlate":
		return runContextFillCorrelate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve context-fill: unknown subcommand %q\n", args[0])
		return 10
	}
}

// writeReportAtomic writes the markdown report to path using the repo's
// temp-then-rename convention (internal/core/blocker_breaker.go et al), so a
// reader never observes a half-written report and a crash never truncates a
// good one.
//
// The Lstat guard is the security half: the previous bare os.WriteFile opened
// the destination O_TRUNC and followed symlinks, so an attacker (or a stale
// artifact link) who pre-planted `report.md -> ~/.ssh/authorized_keys` had that
// target truncated and overwritten with report text. Refusing a non-regular
// destination outright is cheaper than trying to make following one safe; the
// rename then replaces the path itself rather than writing through it.
func writeReportAtomic(path, content string) error {
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("refusing to write %s: not a regular file (mode %s)", path, fi.Mode())
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup; the rename error is the one that matters
		return err
	}
	return nil
}

// runContextFillCorrelate reads the real corpus under --project-root and emits
// the fill-vs-verdict correlation: --json for the machine projection, --out for
// the markdown artifact, markdown on stdout when neither is given. A root with
// no dossier corpus exits non-zero rather than printing an empty report —
// absent evidence must never read as a measured zero.
func runContextFillCorrelate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve context-fill correlate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot, out string
	var asJSON bool
	fs.StringVar(&projectRoot, "project-root", ".", "repository root holding knowledge-base/cycles and .evolve/runs")
	fs.StringVar(&out, "out", "", "write the markdown report to this path")
	fs.BoolVar(&asJSON, "json", false, "emit the Report as JSON on stdout")
	if err := fs.Parse(args); err != nil {
		return 10
	}

	rows, err := contextfillcorrelate.Load(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "evolve context-fill correlate: %v\n", err)
		return 1
	}
	rep := contextfillcorrelate.Correlate(rows)

	if asJSON {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve context-fill correlate: encode report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
	}

	md := contextfillcorrelate.Markdown(rep)
	if out != "" {
		if err := writeReportAtomic(out, md); err != nil {
			fmt.Fprintf(stderr, "evolve context-fill correlate: write %s: %v\n", out, err)
			return 1
		}
		fmt.Fprintf(stderr, "context-fill correlate: wrote %s (%d cycles joined, %d no data)\n",
			out, rep.CyclesJoined, len(rep.NoData))
		return 0
	}
	if !asJSON {
		fmt.Fprint(stdout, md)
	}
	return 0
}
