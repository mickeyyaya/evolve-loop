package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
)

const (
	testAuditForm  = "test audit [--rebaseline <package>] [--check] [--root DIR] [--json]"
	testAuditUsage = "usage: evolve " + testAuditForm
	testUsage      = "usage: evolve test <verb>\n\nverbs:\n" +
		"  audit      audit tests against per-package baseline shards\n" +
		"             ( " + testAuditForm + " )\n"
)

type testAuditArgs struct {
	rebaseline string
	root       string
	check      bool
	asJSON     bool
	help       bool
}

func runTest(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, testUsage)
		return cliExitOK
	}
	if args[0] == "audit" {
		return runTestAudit(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve test: unknown verb %q\n%s", args[0], testUsage)
	return exitUsage
}

func parseTestAuditArgs(args []string, stderr io.Writer) (testAuditArgs, bool) {
	var in testAuditArgs
	flags := flag.NewFlagSet("test audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&in.rebaseline, "rebaseline", "", "package directory whose baseline shard is updated")
	flags.StringVar(&in.root, "root", ".", "Go module root")
	flags.BoolVar(&in.check, "check", false, "verify all shards against the module tree")
	flags.BoolVar(&in.asJSON, "json", false, "print the result as JSON")
	flags.BoolVar(&in.help, "help", false, "show usage")
	positional, err := parseInterspersed(flags, args)
	if err != nil {
		fmt.Fprintln(stderr, testAuditUsage)
		return in, false
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "evolve test audit: unexpected arguments %q\n%s\n", positional, testAuditUsage)
		return in, false
	}
	if in.rebaseline != "" && in.check {
		fmt.Fprintf(stderr, "evolve test audit: use --rebaseline or --check, not both\n%s\n", testAuditUsage)
		return in, false
	}
	return in, true
}

func runTestAudit(args []string, stdout, stderr io.Writer) int {
	return runTestAuditWith(args, testaudit.DefaultConfig, stdout, stderr)
}

func runTestAuditWith(args []string, loadConfig func() (testaudit.Config, error), stdout, stderr io.Writer) int {
	in, ok := parseTestAuditArgs(args, stderr)
	if !ok {
		return exitUsage
	}
	if in.help {
		fmt.Fprintln(stdout, testAuditUsage)
		return cliExitOK
	}
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "evolve test audit: %v\n", err)
		return cliExitIO
	}
	if in.rebaseline != "" {
		return runTestAuditRebaseline(in, cfg, stdout, stderr)
	}
	if in.check {
		return runTestAuditCheck(in, cfg, stdout, stderr)
	}
	return runTestAuditList(in, cfg, stdout, stderr)
}

func runTestAuditRebaseline(in testAuditArgs, cfg testaudit.Config, stdout, stderr io.Writer) int {
	if err := testaudit.Rebaseline(in.root, in.rebaseline, cfg); err != nil {
		fmt.Fprintf(stderr, "evolve test audit: %v\n", err)
		return cliExitFailed
	}
	if in.asJSON {
		return printVerbJSON("test audit", map[string]any{"package": in.rebaseline, "ok": true}, stdout, stderr)
	}
	fmt.Fprintf(stdout, "rebaselined %s\n", in.rebaseline)
	return cliExitOK
}

func runTestAuditCheck(in testAuditArgs, cfg testaudit.Config, stdout, stderr io.Writer) int {
	if err := testaudit.CheckTree(in.root, cfg); err != nil {
		fmt.Fprintf(stderr, "evolve test audit: %v\n", err)
		return cliExitRefused
	}
	if in.asJSON {
		return printVerbJSON("test audit", map[string]any{"ok": true}, stdout, stderr)
	}
	fmt.Fprintln(stdout, "test audit: clean")
	return cliExitOK
}

func runTestAuditList(in testAuditArgs, cfg testaudit.Config, stdout, stderr io.Writer) int {
	found, err := testaudit.AnalyzeTree(in.root, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "evolve test audit: %v\n", err)
		return cliExitIO
	}
	if in.asJSON {
		return printVerbJSON("test audit", map[string]any{"rows": found}, stdout, stderr)
	}
	for _, f := range found {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", f.Package, f.Test, f.Rule)
	}
	return cliExitOK
}
