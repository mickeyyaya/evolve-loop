package opscmd

import (
	"flag"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
)

func RunDoctor(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve doctor: missing subcommand (try: probe <tool> | boot <driver> | live <driver> | plane [root] | versions [--latest] [--json])")
		return exitUsage
	}
	switch args[0] {
	case "probe":
		return runDoctorProbe(args[1:], stdout, stderr)
	case "boot":
		return runDoctorBoot(args[1:], stdout, stderr)
	case "live":
		return runDoctorLive(args[1:], stdout, stderr)
	case "plane":
		return runDoctorPlane(args[1:], stdout, stderr)
	case "versions":
		return runDoctorVersions(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve doctor: unknown subcommand %q\n", args[0])
		return exitUsage
	}
}

var (
	doctorProbe    = doctor.Probe
	doctorEmitJSON = doctor.EmitJSON
)

func runDoctorProbe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	var quiet bool
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	fs.BoolVar(&quiet, "quiet", false, "suppress stdout on success/failure")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "evolve doctor probe: usage: evolve doctor probe <tool> [--json] [--quiet]")
		return exitUsage
	}
	tool := fs.Arg(0)
	r, err := doctorProbe(tool)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor probe: %v\n", err)
		return exitUsage
	}
	if asJSON {
		buf, err := doctorEmitJSON(r)
		if err != nil {
			fmt.Fprintf(stderr, "evolve doctor probe: %v\n", err)
			return exitUsage
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
		return exitFail
	}
	return exitOK
}
