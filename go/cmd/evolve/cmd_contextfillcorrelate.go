package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfillcorrelate"
)

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

func writeReportAtomic(path, content string) error {
	return writeReportAtomicWith(os.Rename, path, content)
}

func writeReportAtomicWith(rename func(string, string) error, path, content string) error {
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("refusing to write %s: not a regular file (mode %s)", path, fi.Mode())
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

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
