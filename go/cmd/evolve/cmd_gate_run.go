package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/gaterun"
)

const gateRunUsage = "usage: evolve gate run --entry <name>"

func init() {
	commands = append(commands, subcommand{Name: "gate", Summary: "Run one registered gate entry as the child of a hermetic run (gate run --entry <name>)", Run: runGate})
}

func runGate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprintf(stderr, "evolve gate: want the verb run (%s)\n", gateRunUsage)
		return cliExitUsage
	}
	return runGateRunWith(gateregistry.Entries(), args[1:], stdin, stdout, stderr)
}

func runGateRunWith(entries []gatecontract.Entry, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve gate run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var name string
	fs.StringVar(&name, "entry", "", "the registered gate entry to run")
	if err := fs.Parse(args); err != nil {
		return cliExitUsage
	}
	if name == "" || fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve gate run: %s\n", gateRunUsage)
		return cliExitUsage
	}
	at := slices.IndexFunc(entries, func(e gatecontract.Entry) bool { return e.Name == name })
	if at < 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		fmt.Fprintf(stderr, "evolve gate run: no registered entry %q (known: %s)\n", name, strings.Join(names, ", "))
		return cliExitUsage
	}
	if err := gaterun.Serve(context.Background(), entries[at], stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "evolve gate run: %v\n", err)
		return cliExitFailed
	}
	return cliExitOK
}
