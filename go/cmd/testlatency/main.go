// Command testlatency turns a `go test -json` event stream into a Markdown
// latency report: per-package wall time, the longest-path test in each
// package, the globally slowest tests, and counts over configurable
// thresholds. It reads the stream on stdin and writes Markdown to stdout, so
// it composes with the Makefile:
//
//	go test -json ./... | go run ./cmd/testlatency -title "Fast suite" > report.md
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	title := flag.String("title", "Test latency report", "report title")
	top := flag.Int("top", 25, "number of rows in the slowest-tests table")
	thrPkg := flag.Float64("threshold-pkg", 5.0, "flag packages whose wall time exceeds this (seconds)")
	thrTest := flag.Float64("threshold-test", 1.0, "flag tests whose elapsed exceeds this (seconds)")
	flag.Parse()

	rep, err := Parse(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testlatency: %v\n", err)
		os.Exit(1)
	}
	md := rep.Markdown(MarkdownOptions{
		Title:        *title,
		Top:          *top,
		ThresholdPkg: *thrPkg,
		ThresholdTst: *thrTest,
	})
	fmt.Print(md)
}
