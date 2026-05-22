package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
)

// runDoctor implements `evolve doctor <subcommand>`. Currently the
// only subcommand is `probe <tool>` (the port of scripts/utility/
// probe-tool.sh). Returns the process exit code.
func runDoctor(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve doctor: missing subcommand (try: probe <tool>)")
		return 10
	}
	switch args[0] {
	case "probe":
		return runDoctorProbe(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve doctor: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runDoctorProbe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	var quiet bool
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	fs.BoolVar(&quiet, "quiet", false, "suppress stdout on success/failure")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 10
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "evolve doctor probe: usage: evolve doctor probe <tool> [--json] [--quiet]")
		return 10
	}
	tool := fs.Arg(0)
	r, err := doctor.Probe(tool)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor probe: %v\n", err)
		return 10
	}
	if asJSON {
		buf, err := doctor.EmitJSON(r)
		if err != nil {
			fmt.Fprintf(stderr, "evolve doctor probe: %v\n", err)
			return 10
		}
		fmt.Fprintf(stdout, "%s\n", buf)
	} else if !quiet {
		if r.Found {
			fmt.Fprintf(stdout, "%s\n", r.Path)
			fmt.Fprintf(stderr, "[doctor] OK: %s found at %s (via %s)\n", r.Tool, r.Path, r.Method)
		} else {
			fmt.Fprintf(stderr, "[doctor] NOT FOUND: %s\n", r.Tool)
			fmt.Fprintln(stderr, "[doctor] checked locations:")
			for _, c := range r.Checked {
				fmt.Fprintf(stderr, "[doctor]   %s\n", c)
			}
		}
	}
	if !r.Found {
		return 1
	}
	return 0
}
