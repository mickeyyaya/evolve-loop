// Command testlatency turns a go test -json event stream on stdin into a Markdown latency report on stdout.
// See docs/architecture/packages/cmd-testlatency.md.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

var exitProcess = os.Exit

func main() {
	exitProcess(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("testlatency", flag.ContinueOnError)
	fs.SetOutput(stderr)
	title := fs.String("title", "Test latency report", "report title")
	top := fs.Int("top", 25, "number of rows in the slowest-tests table")
	thrPkg := fs.Float64("threshold-pkg", 5.0, "flag packages whose wall time exceeds this (seconds)")
	thrTest := fs.Float64("threshold-test", 1.0, "flag tests whose elapsed exceeds this (seconds)")
	budget := fs.Duration("budget", 0, "lead the report with the packages whose wall time exceeds this (for example 180s)")
	failOver := fs.Bool("fail-over-budget", false, "exit 1 when a package is over --budget")
	render := fs.Bool("render", false, "write the Output field of each event to stdout (the plain go test text) and exit 0")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *render {
		renderOutput(stdin, stdout, stderr)
		return 0
	}
	if *failOver && *budget <= 0 {
		fmt.Fprintln(stderr, "testlatency: --fail-over-budget needs --budget")
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
		Budget:       budget.Seconds(),
	}))
	if *failOver && len(rep.OverBudget(budget.Seconds())) > 0 {
		return 1
	}
	return 0
}
