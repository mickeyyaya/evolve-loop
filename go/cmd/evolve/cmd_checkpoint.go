package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

const checkpointUsage = `Usage:
  evolve checkpoint save [--worktree DIR | --all] [--label TEXT] [--push] [--json] [--project-root P]
  evolve checkpoint list [--worktree DIR] [--json] [--project-root P]
  evolve checkpoint restore <ref> --into DIR [--json] [--project-root P]
  evolve checkpoint prune [--landed] [--json] [--project-root P]`

var checkpointClock = time.Now

func runCheckpoint(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && cmdutil.HasHelp(args[:1]) {
		fmt.Fprintln(stdout, checkpointUsage)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve checkpoint: missing subcommand (save|list|restore|prune)\n"+checkpointUsage)
		return 10
	}
	switch args[0] {
	case "save":
		return runCheckpointSave(args[1:], stdout, stderr)
	case "list":
		return runCheckpointList(args[1:], stdout, stderr)
	case "restore":
		return runCheckpointRestore(args[1:], stdout, stderr)
	case "prune":
		return runCheckpointPrune(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve checkpoint: unknown subcommand %q\n%s\n", args[0], checkpointUsage)
	return 10
}

type checkpointFlags struct {
	fs          *flag.FlagSet
	projectRoot *string
	asJSON      *bool
}

func newCheckpointFlags(name string, stderr io.Writer) checkpointFlags {
	fs := flag.NewFlagSet("evolve checkpoint "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return checkpointFlags{
		fs:          fs,
		projectRoot: fs.String("project-root", ".", "a worktree of the repository whose checkpoints to work on; its .evolve/policy.json sets the retention"),
		asJSON:      fs.Bool("json", false, "print JSON instead of lines"),
	}
}

func (f checkpointFlags) parse(args []string) bool {
	return f.fs.Parse(args) == nil && f.fs.NArg() == 0
}

func (f checkpointFlags) hub(stderr io.Writer) (wtcheckpoint.Hub, bool) {
	hub, err := wtcheckpoint.ResolveHub(*f.projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", f.fs.Name(), err)
		return wtcheckpoint.Hub{}, false
	}
	return hub, true
}

func (f checkpointFlags) keepOrNoPrune(stderr io.Writer) int {
	keep, err := f.keep()
	if err != nil {
		fmt.Fprintf(stderr, "%s: WARN: %v; saving, but not pruning any checkpoint until the policy loads\n", f.fs.Name(), err)
		return 0
	}
	return keep
}

func (f checkpointFlags) keep() (int, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(*f.projectRoot)))
	if err != nil {
		return 0, err
	}
	return pol.CheckpointConfig().KeepPerWorktree, nil
}

func (f checkpointFlags) emitJSON(stdout, stderr io.Writer, v any) bool {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "%s: encode json: %v\n", f.fs.Name(), err)
		return false
	}
	return true
}
