package main

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/logread"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var logVerbs = []string{"digest", "events", "failures", "index", "show"}

const logsUsage = `Usage: evolve logs <verb> --cycle N [flags]

  evolve logs digest   --cycle N                              D0: one line for each phase outcome (40 lines)
  evolve logs events   --cycle N [--phase P] [--kind K] [--code C] [--level L] [--from N]   D1 (200 lines)
  evolve logs failures --cycle N [--phase P] [--kind K] [--code C] [--from N]               D1, WARN and above (200 lines)
  evolve logs index    --cycle N [--from N]                   the files of the cycle (200 lines)
  evolve logs show     --cycle N --phase P [--layer d2|d3] [--from LINE]   D2 report (200 lines) or D3 raw excerpt (150 lines)

  --as PHASE   the phase that reads; in shadow mode a read outside its phase.json log_access records LOG_ACCESS_SHADOW and goes on
  --project-root P

Each verb stops at its line budget and ends with a truncation line that names a narrower query. No verb returns a whole file.
Exit 0 read, 1 no log or a read error, 10 usage.
`

type logsFlags struct {
	cycle                                          int
	from                                           int
	phase, kind, code, level, layer, as, rootInput string
}

func parseLogsFlags(verb string, args []string, stderr io.Writer) (logsFlags, bool) {
	var f logsFlags
	fs := flag.NewFlagSet("logs "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.IntVar(&f.cycle, "cycle", 0, "cycle number")
	fs.IntVar(&f.from, "from", 0, "first record or line (1-based)")
	fs.StringVar(&f.phase, "phase", "", "phase name")
	fs.StringVar(&f.kind, "kind", "", "event kind")
	fs.StringVar(&f.code, "code", "", "event code")
	fs.StringVar(&f.level, "level", "", "least severity: info, warn or incident")
	fs.StringVar(&f.layer, "layer", "d2", "show: d2 or d3")
	fs.StringVar(&f.as, "as", "", "the reading phase")
	fs.StringVar(&f.rootInput, "project-root", "", "project root (default EVOLVE_PROJECT_ROOT or cwd)")
	if _, err := cmdutil.ParseInterspersed(fs, args); err != nil {
		return f, false
	}
	return f, true
}

func runLogs(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, logsUsage)
		return 10
	}
	verb := args[0]
	if verb == "help" || verb == "-h" || verb == "--help" {
		fmt.Fprint(stdout, logsUsage)
		return 0
	}
	if !slices.Contains(logVerbs, verb) {
		fmt.Fprintf(stderr, "evolve logs: unknown verb %q\n\n%s", verb, logsUsage)
		return 10
	}
	f, ok := parseLogsFlags(verb, args[1:], stderr)
	if !ok {
		return 10
	}
	sev := signalcenter.Severity(strings.ToUpper(f.level))
	shown, layerOK := logread.ParseLayer(f.layer)
	switch {
	case f.cycle <= 0:
		fmt.Fprintf(stderr, "evolve logs %s: --cycle N is required\n", verb)
	case verb == "show" && f.phase == "":
		fmt.Fprintln(stderr, "evolve logs show: --phase P is required")
	case f.level != "" && !sev.Valid():
		fmt.Fprintf(stderr, "evolve logs %s: --level %q must be info, warn or incident\n", verb, f.level)
	case verb == "show" && (!layerOK || shown == logread.D0 || shown == logread.D1):
		fmt.Fprintf(stderr, "evolve logs show: --layer %q must be d2 or d3\n", f.layer)
	default:
		return readLogs(verb, f, shown, sev, stdout, stderr)
	}
	return 10
}

func readLogs(verb string, f logsFlags, shown logread.Layer, sev signalcenter.Severity, stdout, stderr io.Writer) int {
	root := paths.AbsoluteRoot("--project-root", firstNonEmpty(f.rootInput, cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")), nil)
	signals, closeSignals := newRoleSignalCenter(roleAgent, root, paths.EvolveDirOf(root), stderr)
	defer closeSignals()
	reader := logread.Reader{RunDir: core.RunWorkspacePath(root, f.cycle), Cycle: f.cycle, Signals: signals}
	if f.as != "" {
		reader.RecordShadow(f.as, declaredLogAccess(root, f.as), verbLayer(verb, shown), verb)
	}
	q := logread.Query{Phase: f.phase, Kind: f.kind, Code: f.code, Level: sev, From: f.from}
	var lines []string
	var err error
	switch verb {
	case "digest":
		lines, err = reader.Digest()
	case "events":
		lines, err = reader.Events(q)
	case "failures":
		lines, err = reader.Failures(q)
	case "index":
		lines, err = reader.Index(f.from)
	case "show":
		lines, err = reader.Show(f.phase, shown, f.from)
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve logs %s: %v\n", verb, err)
		return 1
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

func verbLayer(verb string, shown logread.Layer) logread.Layer {
	switch verb {
	case "digest":
		return logread.D0
	case "show":
		return shown
	}
	return logread.D1
}

func declaredLogAccess(root, phase string) *phasespec.LogAccess {
	catalog, _, _, err := phasespec.MergedCatalog(root)
	if err != nil {
		return nil
	}
	spec, ok := catalog.Get(phase)
	if !ok {
		return nil
	}
	return spec.LogAccess
}
