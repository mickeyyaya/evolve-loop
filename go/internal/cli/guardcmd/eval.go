package guardcmd

import (
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifyeval"
)

// runEval implements `evolve eval <subcommand>`. Subcommands:
//   - quality-check [-predicates <path>] <eval.md> — Level-0 tautology
//     detection (single file), plus an advisory authoring-time flaky-shape
//     lint over the NEW cycle's ACS predicate sources when -predicates names
//     a predicates_test.go file or go/acs/cycle<N> dir
//   - diversity-check <evalsDir> — suite-level adversarial-diversity check
//   - verify <eval.md> <workspace> — independent eval re-execution (Phase 2A port 3)
//
// Exit codes from quality-check / diversity-check mirror the bash contract:
//
//	0 PASS, 1 WARN, 2 HALT, 10 bad args, 1 internal error.
func RunEval(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve eval: missing subcommand (quality-check|diversity-check|verify)")
		return 10
	}
	switch args[0] {
	case "quality-check":
		return runEvalQualityCheck(args[1:], stdout, stderr)
	case "diversity-check":
		return runEvalDiversityCheck(args[1:], stdout, stderr)
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
	predicates := fs.String("predicates", "", "NEW cycle's Go ACS predicate source (predicates_test.go file or go/acs/cycle<N> dir); adds the advisory flaky-shape lint (WARN-level, never HALT)")
	rest, ok := parseInterspersed(fs, args)
	if !ok {
		return 10
	}
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
	overall := res.Overall
	// Keyed on flag PRESENCE, not on a non-empty value: `-predicates "$DIR"` with
	// DIR unset is the realistic shell slip, and skipping the lint on it printed a
	// clean PASS with no receipt at all — the same silent-clean class as the
	// dropped flag this loop already exists to prevent. An empty value now reaches
	// the lint, which reports it as the error it is.
	if flagWasSet(fs, "predicates") {
		// Stage=advisory. The join below is MONOTONIC (max of two severities),
		// which is the structural pin: an advisory finding can raise PASS→WARN
		// but can never lower a Level-0 tautology HALT to WARN, no matter how the
		// surrounding code is later refactored. An `overall == LevelPass` guard
		// would have been one edit away from that bug.
		overall = maxEvalLevel(overall, flakyLintAdvisory(*predicates, stdout, stderr))
	}
	switch overall {
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

// parseInterspersed parses args into fs, accepting flags BEFORE or AFTER the
// positionals, and returns the positionals (ok=false on a flag error, already
// reported to fs.Output()).
//
// stdlib flag stops at the first positional, so the documented
// `quality-check <eval.md> -predicates <dir>` order silently DROPPED the flag —
// probed live 2026-07-30: the lint ran on nothing and reported a clean PASS.
// cmdutil.ReorderArgs is bool-flag-only by contract and would scatter
// -predicates' VALUE into the positional list, so this collects positionals one
// at a time and re-parses the remainder (the opscmd console-lease shape). Each
// iteration strictly shrinks `remaining`, so the loop terminates.
//
// Contract note: an unknown flag placed after a positional was previously
// absorbed as an ignored extra positional and now errors (rc 10) — stricter, and
// the only in-repo caller passes a single positional.
func parseInterspersed(fs *flag.FlagSet, args []string) (positional []string, ok bool) {
	remaining := args
	for {
		if err := fs.Parse(remaining); err != nil {
			return nil, false
		}
		if fs.NArg() == 0 {
			return positional, true
		}
		positional = append(positional, fs.Arg(0))
		remaining = fs.Args()[1:]
	}
}

// flagWasSet reports whether name was given on the command line at all,
// regardless of the value parsed. The interspersed-parse loop above calls
// fs.Parse repeatedly, and fs.Visit reflects every set flag across those calls.
func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// maxEvalLevel is the monotonic severity join: PASS(0) < WARN(1) < HALT(2), so
// the result is never LESS severe than either input. Every advisory contribution
// to the quality-check verdict goes through it, which is what makes "advisory can
// never lower a HALT" a property of the code rather than of one conjunct.
func maxEvalLevel(a, b evalqualitycheck.Level) evalqualitycheck.Level {
	if b > a {
		return b
	}
	return a
}

// flakyLintAdvisory runs the authoring-time flaky-shape lint over path (the
// NEW cycle's predicate sources), prints one advisory line per finding annotated
// with its Luo FSE'14 flakiness class, and returns the severity it contributes
// (LevelWarn when anything fired, else LevelPass) for the monotonic join.
//
// The receipt line is UNCONDITIONAL: "linted 0 file(s)" must be visibly
// different from "linted 12 file(s), 0 findings", because a path that taught the
// lint nothing previously printed exactly nothing and read as a clean tree — the
// same silent-clean class as the dropped -predicates flag this seam already hit.
//
// Advisory lines use a `flaky[...]` prefix, deliberately NOT the `L<n>` prefix
// the tautology classifier uses: an Auditor reading `L1` next to a flaky-shape
// note would be told the eval is a weak tautology, which is a different (and
// false) claim.
//
// A lint error is a LOUD skip on stderr contributing LevelPass — advisory tooling
// must never block the eval verdict, but a typo'd path must not pass silently.
func flakyLintAdvisory(path string, stdout, stderr io.Writer) evalqualitycheck.Level {
	report, err := evalqualitycheck.LintFlakyPredicates(path)
	if err != nil {
		fmt.Fprintf(stderr, "evolve eval quality-check: flaky-lint: %v (advisory lint skipped)\n", err)
		return evalqualitycheck.LevelPass
	}
	for _, f := range report.Findings {
		fmt.Fprintf(stdout, "  flaky[%s] %s:%s — %s\n", f.Class, f.File, f.Func, f.Reason)
	}
	fmt.Fprintf(stdout, "[eval quality-check] flaky-lint: linted %d file(s) under %s — %d advisory finding(s) (stage=advisory: raises PASS→WARN, never HALT)\n",
		report.Linted(), path, len(report.Findings))
	if len(report.Findings) > 0 {
		return evalqualitycheck.LevelWarn
	}
	return evalqualitycheck.LevelPass
}

// runEvalDiversityCheck implements `evolve eval diversity-check <evalsDir> [slug]`.
// Scores a directory of evals for adversarial diversity (negative + edge cases).
// Exit codes: 0 PASS, 1 WARN, 2 HALT, 10 bad args, 1 internal error.
func runEvalDiversityCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval diversity-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(stderr, "evolve eval diversity-check: missing <evalsDir> path")
		return 10
	}
	opts := evalqualitycheck.DiversityOptions{EvalDir: rest[0]}
	if len(rest) >= 2 {
		opts.Slug = rest[1]
	}
	res, err := evalqualitycheck.CheckDiversity(opts)
	if err != nil {
		fmt.Fprintf(stderr, "evolve eval diversity-check: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[eval diversity-check] %s — %d evals, %d with negative cases, %d with edge cases, %d positive-only\n",
		res.EvalDir, res.EvalCount, res.NegativeCaseCount, res.EdgeCaseCount, res.PositiveOnlyCount)
	for _, r := range res.Reasons {
		fmt.Fprintf(stdout, "  %s\n", r)
	}
	switch res.Level {
	case evalqualitycheck.DiversityPass:
		fmt.Fprintln(stdout, "[eval diversity-check] verdict: PASS")
		return 0
	case evalqualitycheck.DiversityWarn:
		fmt.Fprintln(stdout, "[eval diversity-check] verdict: WARN")
		return 1
	default:
		fmt.Fprintln(stdout, "[eval diversity-check] verdict: HALT (cohesive suite, zero negative cases)")
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
