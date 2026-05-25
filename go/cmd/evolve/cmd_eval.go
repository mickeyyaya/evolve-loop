package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifyeval"
)

// runEval implements `evolve eval <subcommand>`. Subcommands:
//   - quality-check <eval.md> — Level-0 tautology detection
//   - verify <eval.md> <workspace> — independent eval re-execution (Phase 2A port 3)
//
// Exit codes from quality-check mirror the bash contract:
//
//	0 PASS, 1 WARN, 2 HALT, 10 bad args, 1 internal error.
func runEval(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve eval: missing subcommand (quality-check|verify)")
		return 10
	}
	switch args[0] {
	case "quality-check":
		return runEvalQualityCheck(args[1:], stdout, stderr)
	case "verify":
		return runEvalVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve eval: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runEvalQualityCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval quality-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(stderr, "evolve eval quality-check: missing <eval.md> path")
		return 10
	}
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: rest[0]})
	if err != nil {
		fmt.Fprintf(stderr, "evolve eval quality-check: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[eval quality-check] %s\n", res.Path)
	for _, c := range res.Commands {
		fmt.Fprintf(stdout, "  L%d %s   %s\n", c.Level, c.Reason, c.Line)
	}
	switch res.Overall {
	case evalqualitycheck.LevelPass:
		fmt.Fprintln(stdout, "[eval quality-check] verdict: PASS")
		return 0
	case evalqualitycheck.LevelWarn:
		fmt.Fprintln(stdout, "[eval quality-check] verdict: WARN")
		return 1
	default:
		fmt.Fprintln(stdout, "[eval quality-check] verdict: HALT (Level-0 tautology)")
		return 2
	}
}

// runEvalVerify implements `evolve eval verify <eval.md> <workspace>`.
// Exit codes:
//   - 0 verdict PASS, 1 verdict FAIL, 10 bad args, 1 internal error.
func runEvalVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	rest := fs.Args()
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "evolve eval verify: missing <eval.md> <workspace>")
		return 10
	}
	res, err := verifyeval.Verify(verifyeval.Options{Path: rest[0], Workspace: rest[1]})
	if err != nil {
		fmt.Fprintf(stderr, "evolve eval verify: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[eval verify] %s (workspace=%s)\n", res.Path, rest[1])
	for _, c := range res.Commands {
		mark := "PASS"
		if !c.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(stdout, "  [%s] %s\n", mark, c.Command)
		if c.Reason != "" {
			fmt.Fprintf(stdout, "        reason: %s\n", c.Reason)
		}
	}
	fmt.Fprintf(stdout, "[eval verify] verdict: %s\n", res.Verdict)
	if res.Verdict == "PASS" {
		return 0
	}
	return 1
}
