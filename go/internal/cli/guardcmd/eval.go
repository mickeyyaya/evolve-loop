package guardcmd

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifyeval"
)

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
	predicates := fs.String("predicates", "", "NEW cycle's Go ACS predicate source (predicates_test.go file or go/acs/cycle<N> dir); adds the advisory flaky-shape and unsatisfiable-predicate lints (WARN-level, never HALT)")
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
	if flagWasSet(fs, "predicates") {
		for _, lint := range predicateLints() {
			overall = maxEvalLevel(overall, lint.advise(*predicates, stdout, stderr))
		}
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

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func maxEvalLevel(a, b evalqualitycheck.Level) evalqualitycheck.Level {
	if b > a {
		return b
	}
	return a
}

type predicateLint struct {
	name string
	run  func(path string) (linted int, findings []string, err error)
}

func predicateLints() []predicateLint {
	return []predicateLint{
		{name: "flaky-lint", run: flakyLintFindings},
		{name: "unsatisfiable-lint", run: unsatisfiableLintFindings},
	}
}

func (l predicateLint) advise(path string, stdout, stderr io.Writer) evalqualitycheck.Level {
	linted, findings, err := l.run(path)
	if err != nil {
		fmt.Fprintf(stderr, "evolve eval quality-check: %s: %v (advisory lint skipped)\n", l.name, err)
		return evalqualitycheck.LevelPass
	}
	for _, f := range findings {
		fmt.Fprintf(stdout, "  %s\n", f)
	}
	fmt.Fprintf(stdout, "[eval quality-check] %s: linted %d file(s) under %s — %d advisory finding(s) (stage=advisory: raises PASS→WARN, never HALT)\n",
		l.name, linted, path, len(findings))
	if len(findings) > 0 {
		return evalqualitycheck.LevelWarn
	}
	return evalqualitycheck.LevelPass
}

func flakyLintFindings(path string) (int, []string, error) {
	report, err := evalqualitycheck.LintFlakyPredicates(path)
	findings := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, fmt.Sprintf("flaky[%s] %s:%s — %s", f.Class, f.File, f.Func, f.Reason))
	}
	return report.Linted(), findings, err
}

func unsatisfiableLintFindings(path string) (int, []string, error) {
	report, err := evalqualitycheck.LintUnsatisfiablePredicates(path)
	findings := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, fmt.Sprintf("unsatisfiable[%s] %s:%s — %s", f.Kind, f.File, f.Func, f.Reason))
	}
	return report.Linted(), findings, err
}

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
		command := strings.ReplaceAll(c.Command, "\n", "\n         ")
		fmt.Fprintf(stdout, "  [%s] %s\n", mark, command)
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
