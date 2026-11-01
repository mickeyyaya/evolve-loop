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
	o, ok := parseOptions(args, stderr)
	if !ok {
		return 2
	}
	if o.render {
		renderOutput(stdin, stdout, stderr)
		return 0
	}
	limit, enforce, err := resolveBudget(addedtests.PackageWallBudget, o.pkgBudget, o.failOver, o.budget)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 2
	}
	ratchet, timeout, ok := loadGate(o.ratchetPath, o.testTimeout, stderr)
	if !ok {
		return 2
	}
	rep, err := Parse(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, rep.Markdown(MarkdownOptions{
		Title:        o.title,
		Top:          o.top,
		ThresholdPkg: o.thrPkg,
		ThresholdTst: o.thrTest,
		Budget:       limit.Seconds(),
	}))
	overBudget := enforce && len(rep.OverBudget(limit.Seconds())) > 0
	gateFailed := ratchet.gate(stdout, stderr, rep, timeout) && !o.reportOnly
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

type options struct {
	title       string
	top         int
	thrPkg      float64
	thrTest     float64
	budget      time.Duration
	failOver    bool
	pkgBudget   bool
	ratchetPath string
	testTimeout string
	reportOnly  bool
	render      bool
}

func parseOptions(args []string, stderr io.Writer) (options, bool) {
	var o options
	fs := newFlagSet("testlatency", stderr)
	fs.StringVar(&o.title, "title", "Test latency report", "report title")
	fs.IntVar(&o.top, "top", 25, "number of rows in the slowest-tests table")
	fs.Float64Var(&o.thrPkg, "threshold-pkg", 5.0, "flag packages whose wall time exceeds this (seconds)")
	fs.Float64Var(&o.thrTest, "threshold-test", 1.0, "flag tests whose elapsed exceeds this (seconds)")
	fs.DurationVar(&o.budget, "budget", 0, "lead the report with the packages whose wall time exceeds this (for example 180s)")
	fs.BoolVar(&o.failOver, "fail-over-budget", false, "exit 1 when a package is over --budget")
	fs.BoolVar(&o.pkgBudget, "package-budget", false, "apply addedtests.PackageWallBudget and exit 1 when a package exceeds it")
	fs.StringVar(&o.ratchetPath, "ratchet", "", "gate packages against this ratchet file: exit 1 above min(2 x class ceiling, -test-timeout)")
	fs.StringVar(&o.testTimeout, "test-timeout", "20m", "the go test timeout that caps every package limit")
	fs.BoolVar(&o.reportOnly, "ratchet-report-only", false, "print each package above its ratchet limit but exit 0 for a breach; an unreadable ratchet or stream still fails")
	fs.BoolVar(&o.render, "render", false, "write the Output field of each event to stdout (the plain go test text) and exit 0")
	if err := fs.Parse(args); err != nil {
		return options{}, false
	}
	return o, true
}
