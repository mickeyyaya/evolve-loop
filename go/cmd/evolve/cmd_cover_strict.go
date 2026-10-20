package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
)

type coverStrictArgs struct {
	root, profile, floors, packages string
	asJSON                          bool
}

func runCoverStrict(args []string, stdout, stderr io.Writer) int {
	in, ok := parseCoverStrict(args, stderr)
	if !ok {
		return exitUsage
	}
	moduleDir, err := coverModuleDir(in.root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cover strict: %v\n", err)
		return exitIO
	}
	if in.profile == "" {
		in.profile = filepath.Join(moduleDir, "coverage.txt")
	}
	if in.floors == "" {
		in.floors = filepath.Join(moduleDir, ".cover-strict")
	}
	packages, err := readPackageSet(in.packages)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cover strict: %v\n", err)
		return exitIO
	}
	results, err := coverstrict.Check(moduleDir, in.profile, in.floors)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cover strict: %v\n", err)
		return exitRefused
	}
	results = coverstrict.Within(results, packages)
	if err := printCoverStrict(stdout, results, in.asJSON); err != nil {
		fmt.Fprintf(stderr, "evolve cover strict: write the verdict: %v\n", err)
		return exitIO
	}
	if !coverstrict.Clean(results) {
		return exitRefused
	}
	return 0
}

func parseCoverStrict(args []string, stderr io.Writer) (coverStrictArgs, bool) {
	var in coverStrictArgs
	flags := flag.NewFlagSet("cover strict", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&in.root, "root", ".", "repository root or Go module root")
	flags.StringVar(&in.profile, "profile", "", "coverage profile (default: <module>/coverage.txt)")
	flags.StringVar(&in.floors, "floors", "", "package floors file (default: <module>/.cover-strict)")
	flags.StringVar(&in.packages, "packages", "", packagesFlagUsage)
	flags.BoolVar(&in.asJSON, "json", false, "print the verdict as JSON")
	positional, err := parseInterspersed(flags, args)
	switch {
	case err != nil:
		fmt.Fprintln(stderr, coverUsage)
		return in, false
	case len(positional) > 0:
		fmt.Fprintf(stderr, "evolve cover strict: unexpected arguments %q\n%s\n", positional, coverUsage)
		return in, false
	}
	return in, true
}

func printCoverStrict(w io.Writer, results []coverstrict.Result, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Clean   bool                 `json:"clean"`
			Results []coverstrict.Result `json:"results"`
		}{coverstrict.Clean(results), results})
	}
	for _, r := range results {
		if _, err := fmt.Fprintln(w, r.Line()); err != nil {
			return err
		}
	}
	return nil
}
