package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type event struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

type pkgStat struct {
	Pkg             string
	Wall            float64
	AllTestsSum     float64
	SerialOnlySum   float64
	ParallelLongest float64
	NumTests        int
	SlowestTest     string
	SlowestSecs     float64
	Status          string
}

func (p pkgStat) Floor() float64 { return p.SerialOnlySum + p.ParallelLongest }

func (p pkgStat) Dominant() string {
	if p.ParallelLongest > p.SerialOnlySum {
		return "parallel"
	}
	return "serial"
}

type testStat struct {
	Pkg     string
	Test    string
	Elapsed float64
}

type Report struct {
	Packages   []pkgStat
	Tests      []testStat
	Incomplete []string
}

func Parse(r io.Reader) (*Report, error) {
	agg := &aggregator{pkgs: map[string]*pkgStat{}, paused: map[string]bool{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		agg.observe(e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan test2json stream: %w", err)
	}
	return agg.report(), nil
}

type aggregator struct {
	pkgs   map[string]*pkgStat
	tests  []testStat
	paused map[string]bool
}

func (agg *aggregator) get(name string) *pkgStat {
	p := agg.pkgs[name]
	if p == nil {
		p = &pkgStat{Pkg: name}
		agg.pkgs[name] = p
	}
	return p
}

func (agg *aggregator) observe(e event) {
	switch e.Action {
	case "run", "start":
		if e.Package != "" {
			agg.get(e.Package)
		}
	case "pause":
		if e.Test != "" && !strings.Contains(e.Test, "/") {
			agg.paused[e.Package+"\x00"+e.Test] = true
		}
	case "pass", "fail", "skip":
		if e.Test == "" {
			p := agg.get(e.Package)
			p.Wall = e.Elapsed
			p.Status = e.Action
			return
		}
		if strings.Contains(e.Test, "/") {
			return
		}
		p := agg.get(e.Package)
		p.NumTests++
		p.AllTestsSum += e.Elapsed
		key := e.Package + "\x00" + e.Test
		wasPaused := agg.paused[key]
		delete(agg.paused, key)
		if wasPaused {
			p.ParallelLongest = max(p.ParallelLongest, e.Elapsed)
		} else {
			p.SerialOnlySum += e.Elapsed
		}
		if e.Elapsed > p.SlowestSecs {
			p.SlowestSecs = e.Elapsed
			p.SlowestTest = e.Test
		}
		agg.tests = append(agg.tests, testStat{Pkg: e.Package, Test: e.Test, Elapsed: e.Elapsed})
	}
}

func (agg *aggregator) report() *Report {
	rep := &Report{}
	for _, p := range agg.pkgs {
		rep.Packages = append(rep.Packages, *p)
		if p.Status == "" {
			rep.Incomplete = append(rep.Incomplete, p.Pkg)
		}
	}
	sort.Strings(rep.Incomplete)
	sort.SliceStable(rep.Packages, func(i, j int) bool { return rep.Packages[i].Wall > rep.Packages[j].Wall })
	rep.Tests = agg.tests
	sort.SliceStable(rep.Tests, func(i, j int) bool { return rep.Tests[i].Elapsed > rep.Tests[j].Elapsed })
	return rep
}

type MarkdownOptions struct {
	Title        string
	Top          int
	ThresholdPkg float64
	ThresholdTst float64
	Budget       float64
}

func (rep *Report) Markdown(o MarkdownOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", o.Title)
	rep.writeOverBudget(&b, o)
	rep.writeOverview(&b, o)
	rep.writeSlowPackages(&b, o)
	rep.writeSlowestTests(&b, o)
	rep.writeOverThresholdCount(&b, o)
	return b.String()
}

func (rep *Report) writeOverview(b *strings.Builder, o MarkdownOptions) {
	var aggWall, aggSerial float64
	totalTests := 0
	for _, p := range rep.Packages {
		aggWall += p.Wall
		aggSerial += p.AllTestsSum
		totalTests += p.NumTests
	}

	fmt.Fprintf(b, "- Packages: **%d**\n", len(rep.Packages))
	fmt.Fprintf(b, "- Top-level tests: **%d**\n", totalTests)
	fmt.Fprintf(b, "- Aggregate package wall time: **%.1fs** (sum of parallel-aware per-package times)\n", aggWall)
	fmt.Fprintf(b, "- Fully-serial upper bound (Σ test elapsed): **%.1fs**\n\n", aggSerial)

	if len(rep.Incomplete) > 0 {
		fmt.Fprintf(b, "> ⚠ **%d package(s) had no terminal summary** (truncated stream / panic / timeout) — their wall time is missing and they are NOT counted above: %s\n\n",
			len(rep.Incomplete), strings.Join(rep.Incomplete, ", "))
	}
}

func (rep *Report) OverBudget(limit float64) []pkgStat {
	var over []pkgStat
	if limit <= 0 {
		return over
	}
	for _, p := range rep.Packages {
		if p.Wall > limit {
			over = append(over, p)
		}
	}
	return over
}

func (rep *Report) writeOverBudget(b *strings.Builder, o MarkdownOptions) {
	if o.Budget <= 0 {
		return
	}
	fmt.Fprintf(b, "## Packages over budget (> %.1fs wall)\n\n", o.Budget)
	over := rep.OverBudget(o.Budget)
	if len(over) == 0 {
		b.WriteString("_(none)_ All packages are within the budget.\n\n")
		return
	}
	b.WriteString("| Package | Wall (s) | Over by (s) | Status |\n|---|--:|--:|---|\n")
	for _, p := range over {
		fmt.Fprintf(b, "| %s | %.2f | %.2f | %s |\n", shortPkg(p.Pkg), p.Wall, p.Wall-o.Budget, p.Status)
	}
	b.WriteString("\n")
}

func (rep *Report) writeSlowPackages(b *strings.Builder, o MarkdownOptions) {
	fmt.Fprintf(b, "## Slow packages (> %.1fs wall) — optimization targets\n\n", o.ThresholdPkg)
	flagged := 0
	b.WriteString("| Package | Wall (s) | Tests | Σall tests (s) | Serial Σ (s) | Parallel max (s) | Floor (s) | Dominant | Slowest test | Slowest (s) |\n")
	b.WriteString("|---|--:|--:|--:|--:|--:|--:|---|---|--:|\n")
	for _, p := range rep.Packages {
		if p.Wall <= o.ThresholdPkg {
			continue
		}
		flagged++
		fmt.Fprintf(b, "| %s | %.2f | %d | %.2f | %.2f | %.2f | %.2f | %s | %s | %.2f |\n",
			shortPkg(p.Pkg), p.Wall, p.NumTests, p.AllTestsSum, p.SerialOnlySum, p.ParallelLongest, p.Floor(), p.Dominant(), p.SlowestTest, p.SlowestSecs)
	}
	if flagged == 0 {
		b.WriteString("| _(none)_ | | | | | | | | | |\n")
	}
	b.WriteString("\n")
}

func (rep *Report) writeSlowestTests(b *strings.Builder, o MarkdownOptions) {
	fmt.Fprintf(b, "## Slowest %d tests\n\n", o.Top)
	b.WriteString("| Test | Package | Elapsed (s) |\n|---|---|--:|\n")
	for i, t := range rep.Tests {
		if i >= o.Top {
			break
		}
		fmt.Fprintf(b, "| %s | %s | %.2f |\n", t.Test, shortPkg(t.Pkg), t.Elapsed)
	}
	b.WriteString("\n")
}

func (rep *Report) writeOverThresholdCount(b *strings.Builder, o MarkdownOptions) {
	over := 0
	for _, t := range rep.Tests {
		if t.Elapsed > o.ThresholdTst {
			over++
		}
	}
	fmt.Fprintf(b, "_%d tests exceed the %.1fs per-test threshold._\n", over, o.ThresholdTst)
}

func shortPkg(pkg string) string {
	const prefix = "github.com/mickeyyaya/evolve-loop/go/"
	return strings.TrimPrefix(pkg, prefix)
}
