package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	testScopedUsage  = "usage: evolve test-scoped [--base <ref>] [--root DIR] [--json]"
	testScopedSchema = "evolve.test-scoped/1"
)

type testScopedArgs struct {
	base   string
	root   string
	asJSON bool
}

type testScopedJSON struct {
	Schema     string   `json:"schema"`
	BaseRef    string   `json:"base_ref,omitempty"`
	MergeBase  string   `json:"merge_base,omitempty"`
	Selected   []string `json:"selected"`
	DurationMS int64    `json:"duration_ms"`
	Pass       bool     `json:"pass"`
}

func runTestScoped(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	in, ok := parseTestScoped(args, stderr)
	if !ok {
		return exitUsage
	}
	res, err := changedpkgs.RunShadowTests(context.Background(), in.root, in.base, sysexec.DefaultRunner)
	if err != nil {
		fmt.Fprintf(stderr, "evolve test-scoped: %v\n", err)
		return exitIO
	}
	if err := printTestScoped(stdout, res, in.asJSON); err != nil {
		fmt.Fprintf(stderr, "evolve test-scoped: %v\n", err)
		return exitIO
	}
	if !res.Pass {
		return exitRefused
	}
	return 0
}

func parseTestScoped(args []string, stderr io.Writer) (testScopedArgs, bool) {
	var in testScopedArgs
	flags := flag.NewFlagSet("test-scoped", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&in.base, "base", "main", "git ref to compute merge-base against")
	flags.StringVar(&in.root, "root", ".", "repository root")
	flags.BoolVar(&in.asJSON, "json", false, "print the result as JSON")
	positional, err := parseInterspersed(flags, args)
	if err != nil {
		fmt.Fprintln(stderr, testScopedUsage)
		return in, false
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "evolve test-scoped: unexpected arguments %q\n%s\n", positional, testScopedUsage)
		return in, false
	}
	return in, true
}

func printTestScoped(stdout io.Writer, res changedpkgs.ShadowTestResult, asJSON bool) error {
	if asJSON {
		env := testScopedJSON{
			Schema:     testScopedSchema,
			BaseRef:    res.BaseRef,
			MergeBase:  res.MergeBase,
			Selected:   res.Selected,
			DurationMS: res.DurationMS,
			Pass:       res.Pass,
		}
		return json.NewEncoder(stdout).Encode(env)
	}
	if _, err := fmt.Fprintf(stdout, "test-scoped: selected=%d duration_ms=%d pass=%t\n", len(res.Selected), res.DurationMS, res.Pass); err != nil {
		return err
	}
	if len(res.Selected) > 0 {
		if _, err := fmt.Fprintf(stdout, "packages: %s\n", strings.Join(res.Selected, " ")); err != nil {
			return err
		}
	}
	if !res.Pass && res.Output != "" {
		if _, err := fmt.Fprint(stdout, res.Output); err != nil {
			return err
		}
	}
	return nil
}
