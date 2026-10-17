package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/treewalk"
)

const treewalkUsage = `Usage:
  evolve treewalk snapshot --root DIR --out FILE
  evolve treewalk check --root DIR --baseline FILE`

func runTreewalk(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve treewalk: missing subcommand (snapshot|check)\n"+treewalkUsage)
		return 10
	}
	switch args[0] {
	case "snapshot":
		return runTreewalkSnapshot(args[1:], stdout, stderr)
	case "check":
		return runTreewalkCheck(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, treewalkUsage)
		return 0
	}
	fmt.Fprintf(stderr, "evolve treewalk: unknown subcommand %q\n%s\n", args[0], treewalkUsage)
	return 10
}

func runTreewalkSnapshot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve treewalk snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "directory tree to walk")
	out := fs.String("out", "", "file that receives the snapshot")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *out == "" {
		fmt.Fprintln(stderr, treewalkUsage)
		return 10
	}
	snap, err := treewalk.Take(*root)
	if err == nil {
		err = snap.Write(*out)
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve treewalk snapshot: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "treewalk: %d paths under %s\n", len(snap.Paths), *root)
	return 0
}

func runTreewalkCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve treewalk check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "directory tree to walk")
	baseline := fs.String("baseline", "", "snapshot taken before the tests ran")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *baseline == "" {
		fmt.Fprintln(stderr, treewalkUsage)
		return 10
	}
	before, err := treewalk.Load(*baseline)
	if err != nil {
		fmt.Fprintf(stderr, "evolve treewalk check: %v\n", err)
		return 1
	}
	after, err := treewalk.Take(*root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve treewalk check: %v\n", err)
		return 1
	}
	leaked := treewalk.Diff(before, after)
	for _, p := range leaked {
		fmt.Fprintf(stdout, "LEAK %s\n", p)
	}
	if len(leaked) > 0 {
		fmt.Fprintf(stderr, "evolve treewalk check: %d path(s) appeared under %s while the tests ran\n", len(leaked), *root)
		return 1
	}
	fmt.Fprintf(stdout, "treewalk: clean, no path appeared under %s\n", *root)
	return 0
}
