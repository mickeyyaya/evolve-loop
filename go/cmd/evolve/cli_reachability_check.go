package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
)

func runReachability(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve reachability: missing subcommand (check-pin)")
		return 10
	}
	switch args[0] {
	case "check-pin":
		return runReachabilityCheckPin(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve reachability: unknown subcommand %q\n", args[0])
		return 10
	}
}

// runReachabilityCheckPin reports whether pinning a call to the referenced
// symbol inside the pinning package would close an import cycle.
func runReachabilityCheckPin(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve reachability check-pin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, pinningPkg, referencedPkg, symbol, pkgsRaw string
	fs.StringVar(&root, "root", "", "module root directory (containing go.mod)")
	fs.StringVar(&pinningPkg, "pinning-package", "", "package the structural test's pin would be written inside")
	fs.StringVar(&referencedPkg, "referenced-package", "", "package the pinned call site references")
	fs.StringVar(&symbol, "symbol", "", "symbol referenced at the call site")
	fs.StringVar(&pkgsRaw, "pkgs", "", "comma-separated package patterns (e.g. ./internal/fleet,./internal/core) scoping the import graph")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if root == "" || pinningPkg == "" || referencedPkg == "" || symbol == "" || pkgsRaw == "" {
		fmt.Fprintln(stderr, "evolve reachability check-pin: --root, --pinning-package, --referenced-package, --symbol, --pkgs are all required")
		return 10
	}

	pkgs := strings.Split(pkgsRaw, ",")
	graph, err := reachabilityprobe.BuildImportGraph(root, pkgs...)
	if err != nil {
		fmt.Fprintf(stderr, "evolve reachability check-pin: %v\n", err)
		return 10
	}

	violation := reachabilityprobe.CheckCallSite(graph, reachabilityprobe.CallSite{
		PinningPackage:    pinningPkg,
		ReferencedPackage: referencedPkg,
		Symbol:            symbol,
	})
	if violation != nil {
		fmt.Fprintln(stderr, violation.Error())
		return 1
	}

	fmt.Fprintf(stdout, "reachability check-pin: no import-cycle violation pinning %s.%s( inside %s\n", referencedPkg, symbol, pinningPkg)
	return 0
}
