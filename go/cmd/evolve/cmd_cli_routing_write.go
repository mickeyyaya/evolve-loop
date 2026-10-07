package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

const routingShipHint = "ship with: evolve ship --class manual (at a wave boundary)"

type routingWriteFlags struct {
	root   string
	clis   string
	model  string
	dryRun bool
}

type routingEdit func(existing []byte, pol policy.Policy) (map[string]any, error)

type routingWrite struct {
	verb   string
	flags  routingWriteFlags
	stdout io.Writer
	stderr io.Writer
}

func runCLIRoutingWrite(verb string, args []string, stdout, stderr io.Writer) int {
	f, rest, ok := parseRoutingWriteFlags(verb, args, stderr)
	if !ok {
		return exitRoutingUsage
	}
	edit, err := routingEditFor(verb, f, rest)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cli-routing %s: %v (%s)\n", verb, err, cliRoutingUsage)
		return exitRoutingUsage
	}
	if err := refuseRoutingWrite(f.root, os.Getenv, time.Now()); err != nil {
		fmt.Fprintf(stderr, "evolve cli-routing %s: %v\n", verb, err)
		return exitRoutingFinding
	}
	return writeRoutingEdit(routingWrite{verb: "cli-routing " + verb, flags: f, stdout: stdout, stderr: stderr}, edit)
}

func parseRoutingWriteFlags(verb string, args []string, stderr io.Writer) (routingWriteFlags, []string, bool) {
	var f routingWriteFlags
	fs := flag.NewFlagSet("cli-routing "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.root, "project-root", "", "project root (default: EVOLVE_PROJECT_ROOT or the cwd)")
	fs.StringVar(&f.clis, "clis", "", "init: the CLIs this operator has, in walk order")
	fs.StringVar(&f.model, "model", "", "set agents.<agent>: the tier the agent dispatches at")
	fs.BoolVar(&f.dryRun, "dry-run", false, "print the result and write nothing")
	rest, err := cmdutil.ParseInterspersed(fs, args)
	if err != nil {
		return f, nil, false
	}
	root, err := routingProjectRoot(f.root, os.Getwd)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cli-routing %s: %v\n", verb, err)
		return f, nil, false
	}
	f.root = root
	return f, rest, true
}

func routingEditFor(verb string, f routingWriteFlags, rest []string) (routingEdit, error) {
	switch {
	case verb == "init" && len(rest) == 0 && f.clis != "":
		return initRoutingEdit(splitRoutingList(f.clis)), nil
	case verb == "set" && len(rest) == 2:
		return setRoutingEdit(routingSet{key: rest[0], values: splitRoutingList(rest[1]), model: f.model}), nil
	case verb == "unset" && len(rest) == 1:
		return unsetRoutingEdit(rest[0]), nil
	case verb == "migrate" && len(rest) == 0:
		return migrateRoutingEdit, nil
	}
	return nil, errors.New("wrong arguments")
}

func splitRoutingList(value string) []string {
	var out []string
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			out = append(out, entry)
		}
	}
	return out
}

func refuseRoutingWrite(root string, getenv func(string) string, now time.Time) error {
	if subagent.ReadDispatchDepth(getenv) > 0 {
		return errors.New("refused inside a phase: the routing table changes at a wave boundary, never from a phase agent")
	}
	live := runlease.LiveRuns(filepath.Join(root, ".evolve", "runs"), now)
	if len(live) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(live))
	for _, run := range live {
		dirs = append(dirs, filepath.Base(run.Dir))
	}
	return fmt.Errorf("refused while a cycle runs (%s holds a live lease): change the table at a wave boundary", strings.Join(dirs, ", "))
}

func writeRoutingEdit(w routingWrite, edit routingEdit) int {
	path := filepath.Join(w.flags.root, ".evolve", "policy.json")
	candidate, err := routingCandidate(path, edit)
	if err == nil {
		err = validateRoutingCandidate(w.flags.root, candidate, w.stderr)
	}
	if err != nil {
		fmt.Fprintf(w.stderr, "evolve %s: %v\n", w.verb, err)
		return exitRoutingFinding
	}
	if w.flags.dryRun {
		fmt.Fprintf(w.stdout, "%s --dry-run: %s would become\n%s", w.verb, path, candidate)
		return 0
	}
	if err := atomicwrite.Bytes(path, candidate); err != nil {
		fmt.Fprintf(w.stderr, "evolve %s: %v\n", w.verb, err)
		return exitRoutingFinding
	}
	fmt.Fprintf(w.stdout, "%s: wrote %s\n%s\n", w.verb, path, routingShipHint)
	return 0
}

func routingCandidate(path string, edit routingEdit) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	pol, err := policy.Parse(orEmptyObject(existing))
	if err != nil {
		return nil, fmt.Errorf("the current policy.json does not parse: %w", err)
	}
	patch, err := edit(existing, pol)
	if err != nil {
		return nil, err
	}
	return policy.PatchBlocks(existing, patch)
}

func orEmptyObject(raw []byte) []byte {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return []byte("{}")
	}
	return raw
}

func validateRoutingCandidate(root string, candidate []byte, stderr io.Writer) error {
	pol, err := policy.Parse(candidate)
	if err != nil {
		return fmt.Errorf("the result would not parse: %w", err)
	}
	s := routerSetupOf(pol, routerSite{root: root, catalog: routingCatalog(root)}, cliroute.Host{LookPath: everyBinaryPresent})
	_, findings, err := cliroute.Build(s)
	reportRoutingFindings(stderr, findings)
	if err != nil {
		return fmt.Errorf("refused, the result would not compile: %w", err)
	}
	return nil
}

func initRoutingEdit(clis []string) routingEdit {
	return func(existing []byte, pol policy.Policy) (map[string]any, error) {
		if pol.CLIRouting != nil {
			return nil, errors.New("a cli_routing block exists: edit it with evolve cli-routing set")
		}
		return migrateLegacyKeys(existing, pol, policy.CLIRouting{CLIs: clis, Default: clis})
	}
}

func migrateRoutingEdit(existing []byte, pol policy.Policy) (map[string]any, error) {
	if pol.CLIRouting == nil {
		return nil, errors.New("there is no cli_routing block to migrate into: run evolve cli-routing init --clis first")
	}
	return migrateLegacyKeys(existing, pol, *pol.CLIRouting)
}

func setRoutingEdit(set routingSet) routingEdit {
	return func(_ []byte, pol policy.Policy) (map[string]any, error) {
		if pol.CLIRouting == nil {
			return nil, errors.New("there is no cli_routing block: run evolve cli-routing init --clis first")
		}
		block, err := setRoutingKey(cloneRouting(*pol.CLIRouting), set)
		return map[string]any{"cli_routing": block}, err
	}
}

func unsetRoutingEdit(key string) routingEdit {
	return func(_ []byte, pol policy.Policy) (map[string]any, error) {
		if pol.CLIRouting == nil {
			return nil, errors.New("there is no cli_routing block")
		}
		block, err := unsetRoutingKey(cloneRouting(*pol.CLIRouting), key)
		return map[string]any{"cli_routing": block}, err
	}
}
