package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
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

// runEvalVerify is implemented in cmd_verify_eval.go (Phase 2A port 3).
// Defined here so cmd_eval.go can reference it before that file lands.
func runEvalVerify(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "evolve eval verify: not yet implemented (Phase 2A port 3)")
	return 99
}
