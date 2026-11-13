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
	margin, ok := parseTimeoutMargin(o.timeoutMargin, stderr)
	if !ok {
		return 2
	}
	gate, ok := loadGate(o.ratchetPath, o.testTimeout, o.rusagePath, stderr)
	if !ok {
		return 2
	}
	rep, err := Parse(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, rep.Markdown(MarkdownOptions{
		Title:         o.title,
		Top:           o.top,
		ThresholdPkg:  o.thrPkg,
		ThresholdTst:  o.thrTest,
		Budget:        limit.Seconds(),
		TimeoutMargin: margin.Seconds(),
	}))
	overBudget := enforce && len(rep.OverBudget(limit.Seconds())) > 0
	gateFailed := gate.failed(stdout, stderr, rep)
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
	title         string
	top           int
	thrPkg        float64
	thrTest       float64
	budget        time.Duration
	failOver      bool
	pkgBudget     bool
	ratchetPath   string
	testTimeout   string
	rusagePath    string
	render        bool
	timeoutMargin string
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
	fs.StringVar(&o.ratchetPath, "ratchet", "", "gate packages against this ratchet file: exit 1 for a package above its CPU baseline after the speed index, or above -test-timeout")
	fs.StringVar(&o.rusagePath, "rusage", "", "test-rusage.ndjson that -ratchet reads the package CPU from")
	fs.StringVar(&o.testTimeout, "test-timeout", "20m", "the go test timeout that caps every package limit")
	fs.BoolVar(&o.render, "render", false, "write the Output field of each event to stdout (the plain go test text) and exit 0")
	fs.StringVar(&o.timeoutMargin, "timeout-margin", "", "add a section with the margin of the slowest package against this go test timeout (for example 30m); LOW under 20%")
	if err := fs.Parse(args); err != nil {
		return options{}, false
	}
	return o, true
}

func parseTimeoutMargin(value string, stderr io.Writer) (time.Duration, bool) {
	if value == "" {
		return 0, true
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		fmt.Fprintf(stderr, "testlatency: -timeout-margin %q: want a positive duration such as 30m\n", value)
		return 0, false
	}
	return d, true
}
