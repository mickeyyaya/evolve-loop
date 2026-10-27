// Command testlatency turns a go test -json event stream on stdin into a Markdown latency report on stdout.
// See docs/architecture/packages/cmd-testlatency.md.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
)

var exitProcess = os.Exit

func main() {
	exitProcess(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "tighten" {
		return runTighten(args[1:], stdin, stderr)
	}
	fs := newFlagSet("testlatency", stderr)
	title := fs.String("title", "Test latency report", "report title")
	top := fs.Int("top", 25, "number of rows in the slowest-tests table")
	thrPkg := fs.Float64("threshold-pkg", 5.0, "flag packages whose wall time exceeds this (seconds)")
	thrTest := fs.Float64("threshold-test", 1.0, "flag tests whose elapsed exceeds this (seconds)")
	budget := fs.Duration("budget", 0, "lead the report with the packages whose wall time exceeds this (for example 180s)")
	failOver := fs.Bool("fail-over-budget", false, "exit 1 when a package is over --budget")
	pkgBudget := fs.Bool("package-budget", false, "apply addedtests.PackageWallBudget and exit 1 when a package exceeds it")
	ratchetPath := fs.String("ratchet", "", "gate packages against this ratchet file: exit 1 above min(2 x class ceiling, -test-timeout)")
	testTimeout := fs.String("test-timeout", "20m", "the go test timeout that caps every package limit")
	render := fs.Bool("render", false, "write the Output field of each event to stdout (the plain go test text) and exit 0")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *render {
		renderOutput(stdin, stdout, stderr)
		return 0
	}
	limit, enforce, err := resolveBudget(addedtests.PackageWallBudget, *pkgBudget, *failOver, *budget)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 2
	}
	ratchet, timeout, ok := loadGate(*ratchetPath, *testTimeout, stderr)
	if !ok {
		return 2
	}
	rep, err := Parse(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, rep.Markdown(MarkdownOptions{
		Title:        *title,
		Top:          *top,
		ThresholdPkg: *thrPkg,
		ThresholdTst: *thrTest,
		Budget:       limit.Seconds(),
	}))
	overBudget := enforce && len(rep.OverBudget(limit.Seconds())) > 0
	gateFailed := ratchet.gate(stdout, stderr, rep, timeout)
	if overBudget || gateFailed {
		return 1
	}
	return 0
}

func resolveBudget(wallBudget string, pkgBudget, failOver bool, budget time.Duration) (time.Duration, bool, error) {
	if pkgBudget {
		d, err := time.ParseDuration(wallBudget)
		if err != nil {
			return 0, false, fmt.Errorf("addedtests.PackageWallBudget: %w", err)
		}
		budget, failOver = d, true
	}
	if failOver && budget <= 0 {
		return 0, false, fmt.Errorf("--fail-over-budget needs --budget")
	}
	return budget, failOver, nil
}
