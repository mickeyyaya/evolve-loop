package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// event is the subset of `go test -json` event fields we consume. See
// `go doc test2json` for the full schema.
type event struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"` // seconds; present on pass/fail/skip
}

// pkgStat is the per-package aggregate.
type pkgStat struct {
	Pkg         string
	Wall        float64 // package-level Elapsed (real wall time, parallel-aware)
	SerialSum   float64 // Σ top-level test Elapsed (fully-serial upper bound)
	NumTests    int
	SlowestTest string
	SlowestSecs float64
	Status      string // pass | fail | skip | (empty)
}

// testStat is one top-level test's timing.
type testStat struct {
	Pkg     string
	Test    string
	Elapsed float64
}

// Report is the parsed, aggregated view of a `go test -json` stream.
type Report struct {
	Packages []pkgStat
	Tests    []testStat // top-level tests only (no "Parent/Sub")
	// Incomplete names packages that started but emitted no terminal
	// pass/fail/skip summary — Wall is unknown (a truncated stream, a panic,
	// or a build/timeout kill). Surfaced as a report note so a partial run
	// isn't silently mistaken for a complete one.
	Incomplete []string
}

// Parse reads a `go test -json` event stream and aggregates per-package and
// per-test timing. Subtests (Test contains "/") are excluded from the test
// list and the serial-sum so parent timings are not double-counted.
func Parse(r io.Reader) (*Report, error) {
	pkgs := map[string]*pkgStat{}
	get := func(name string) *pkgStat {
		p := pkgs[name]
		if p == nil {
			p = &pkgStat{Pkg: name}
			pkgs[name] = p
		}
		return p
	}

	var tests []testStat
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 16*1024*1024) // grow lazily; some Output lines are large
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // tolerate non-event lines (build output, etc.)
		}
		switch e.Action {
		case "run", "start":
			// Register the package so a run that never reaches a terminal
			// summary (panic / timeout / truncated stream) is detectable as
			// Incomplete rather than vanishing from the report entirely.
			if e.Package != "" {
				get(e.Package)
			}
		case "pass", "fail", "skip":
			if e.Test == "" { // package-level summary event
				p := get(e.Package)
				p.Wall = e.Elapsed
				p.Status = e.Action
				continue
			}
			if strings.Contains(e.Test, "/") {
				continue // subtest — folded into its parent's elapsed
			}
			p := get(e.Package)
			p.NumTests++
			p.SerialSum += e.Elapsed
			if e.Elapsed > p.SlowestSecs {
				p.SlowestSecs = e.Elapsed
				p.SlowestTest = e.Test
			}
			tests = append(tests, testStat{Pkg: e.Package, Test: e.Test, Elapsed: e.Elapsed})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan test2json stream: %w", err)
	}

	rep := &Report{}
	for _, p := range pkgs {
		rep.Packages = append(rep.Packages, *p)
		if p.Status == "" { // started but no terminal pass/fail/skip summary
			rep.Incomplete = append(rep.Incomplete, p.Pkg)
		}
	}
	sort.Strings(rep.Incomplete)
	// Stable sort so packages/tests with equal timing keep a reproducible order.
	sort.SliceStable(rep.Packages, func(i, j int) bool { return rep.Packages[i].Wall > rep.Packages[j].Wall })
	rep.Tests = tests
	sort.SliceStable(rep.Tests, func(i, j int) bool { return rep.Tests[i].Elapsed > rep.Tests[j].Elapsed })
	return rep, nil
}

// MarkdownOptions tune the rendered report.
type MarkdownOptions struct {
	Title        string
	Top          int     // rows in the "slowest" tables
	ThresholdPkg float64 // flag packages whose wall time exceeds this (s)
	ThresholdTst float64 // flag tests whose elapsed exceeds this (s)
}

// Markdown renders the report as a Markdown document.
func (rep *Report) Markdown(o MarkdownOptions) string {
	var b strings.Builder
	var aggWall, aggSerial float64
	totalTests := 0
	for _, p := range rep.Packages {
		aggWall += p.Wall
		aggSerial += p.SerialSum
		totalTests += p.NumTests
	}

	fmt.Fprintf(&b, "# %s\n\n", o.Title)
	fmt.Fprintf(&b, "- Packages: **%d**\n", len(rep.Packages))
	fmt.Fprintf(&b, "- Top-level tests: **%d**\n", totalTests)
	fmt.Fprintf(&b, "- Aggregate package wall time: **%.1fs** (sum of parallel-aware per-package times)\n", aggWall)
	fmt.Fprintf(&b, "- Fully-serial upper bound (Σ test elapsed): **%.1fs**\n\n", aggSerial)

	if len(rep.Incomplete) > 0 {
		fmt.Fprintf(&b, "> ⚠ **%d package(s) had no terminal summary** (truncated stream / panic / timeout) — their wall time is missing and they are NOT counted above: %s\n\n",
			len(rep.Incomplete), strings.Join(rep.Incomplete, ", "))
	}

	// Optimization targets: packages over the wall threshold.
	fmt.Fprintf(&b, "## Slow packages (> %.1fs wall) — optimization targets\n\n", o.ThresholdPkg)
	flagged := 0
	b.WriteString("| Package | Wall (s) | Tests | Σserial (s) | Slowest test | Slowest (s) |\n")
	b.WriteString("|---|--:|--:|--:|---|--:|\n")
	for _, p := range rep.Packages {
		if p.Wall <= o.ThresholdPkg {
			continue
		}
		flagged++
		fmt.Fprintf(&b, "| %s | %.2f | %d | %.2f | %s | %.2f |\n",
			shortPkg(p.Pkg), p.Wall, p.NumTests, p.SerialSum, p.SlowestTest, p.SlowestSecs)
	}
	if flagged == 0 {
		b.WriteString("| _(none)_ | | | | | |\n")
	}
	b.WriteString("\n")

	// Slowest individual tests.
	fmt.Fprintf(&b, "## Slowest %d tests\n\n", o.Top)
	b.WriteString("| Test | Package | Elapsed (s) |\n|---|---|--:|\n")
	for i, t := range rep.Tests {
		if i >= o.Top {
			break
		}
		fmt.Fprintf(&b, "| %s | %s | %.2f |\n", t.Test, shortPkg(t.Pkg), t.Elapsed)
	}
	b.WriteString("\n")

	// Count of tests over the per-test threshold (regression watch).
	over := 0
	for _, t := range rep.Tests {
		if t.Elapsed > o.ThresholdTst {
			over++
		}
	}
	fmt.Fprintf(&b, "_%d tests exceed the %.1fs per-test threshold._\n", over, o.ThresholdTst)
	return b.String()
}

// shortPkg trims the module prefix for readability.
func shortPkg(pkg string) string {
	const prefix = "github.com/mickeyyaya/evolve-loop/go/"
	return strings.TrimPrefix(pkg, prefix)
}
