package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

const (
	codeBenchVerbName    = "bench"
	codeBenchTimeout     = 15 * time.Minute
	codeBenchGrepSymbols = 10
)

func codeBenchVerb(args []string, stdout, stderr io.Writer, host codeIndexHost) int {
	fs := flag.NewFlagSet("evolve code bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	backends := fs.String("backends", "", "comma-separated backends to measure (default: every known backend)")
	symbols := fs.Int("symbols", 40, "number of symbols to sample from the truth")
	rounds := fs.Int("rounds", 3, "rounds of each query, for the latency percentiles")
	root := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, codeUsage)
		return codeExitUsage
	}
	loc, err := loadCodeIndexLocation(codeProjectRoot(*root))
	if err != nil {
		fmt.Fprintf(stderr, "evolve code bench: %v\n", err)
		return codeExitFailed
	}
	factories := codeBackendFactories(loc, host.run, host.env)
	names := codeBenchNames(*backends, factories)
	measured, err := codeindex.Resolve(names, factories)
	if err != nil {
		fmt.Fprintf(stderr, "evolve code bench: %v\n", err)
		return codeExitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, codeBenchTimeout)
	defer cancel()
	pool, err := gotypes.Truth(gotypes.IndexPath(loc.indexDir))
	if err != nil {
		rc, _, message := codeErrorOutcome(codeindex.Request{Query: codeindex.QueryCallers}, err)
		fmt.Fprintln(stderr, message)
		return rc
	}
	report, err := runCodeBench(ctx, host, loc, measured, pool, *symbols, *rounds)
	if err != nil {
		fmt.Fprintf(stderr, "evolve code bench: %v\n", err)
		return codeExitFailed
	}
	if err := report.WriteTable(stdout); err != nil {
		fmt.Fprintf(stderr, "evolve code bench: %v\n", err)
		return codeExitFailed
	}
	return 0
}

type independentBenchOps struct {
	truth        func() ([]codeindex.BenchSymbol, error)
	mkdtemp      func(dir, pattern string) (string, error)
	writeFixture func(repoDir string) error
	mkdirAll     func(path string, perm os.FileMode) error
	buildIndex   func(ctx context.Context, moduleDir, out string) error
}

func defaultIndependentBenchOps() independentBenchOps {
	return independentBenchOps{
		truth:        codeindex.IndependentTruth,
		mkdtemp:      os.MkdirTemp,
		writeFixture: codeindex.WriteIndependentFixture,
		mkdirAll:     os.MkdirAll,
		buildIndex:   gotypes.Build,
	}
}

func runCodeBench(ctx context.Context, host codeIndexHost, loc codeIndexLocation, measured []codeindex.Backend, pool []codeindex.BenchSymbol, symbols, rounds int) (codeindex.BenchReport, error) {
	loadAvg := func() float64 { return hostLoadAverage(ctx, host, os.ReadFile) }
	derived, err := codeindex.RunBench(ctx, codeindex.BenchRun{
		TruthSet:    "go-types",
		Backends:    measured,
		Symbols:     codeindex.SampleSymbols(pool, symbols, codeBenchProbes()),
		Rounds:      rounds,
		Now:         time.Now,
		LoadAvg:     loadAvg,
		Root:        loc.indexDir,
		GrepSymbols: codeBenchGrepSymbols,
	})
	if err != nil {
		return codeindex.BenchReport{}, err
	}
	independent, err := runIndependentBench(ctx, host, loc, measured, rounds, loadAvg, defaultIndependentBenchOps())
	if err != nil {
		return codeindex.BenchReport{}, err
	}
	derived.Rows = slices.Concat(derived.Rows, independent.Rows)
	return derived, nil
}

func runIndependentBench(ctx context.Context, host codeIndexHost, loc codeIndexLocation, measured []codeindex.Backend, rounds int, loadAvg func() float64, ops independentBenchOps) (codeindex.BenchReport, error) {
	truth, err := ops.truth()
	if err != nil {
		return codeindex.BenchReport{}, err
	}
	tmpDir, err := ops.mkdtemp("", "evolve-code-bench-*")
	if err != nil {
		return codeindex.BenchReport{}, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	repoDir := filepath.Join(tmpDir, "repo")
	indexDir := filepath.Join(tmpDir, "idx")
	if err := ops.writeFixture(repoDir); err != nil {
		return codeindex.BenchReport{}, err
	}
	if err := ops.mkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		return codeindex.BenchReport{}, err
	}
	if err := ops.buildIndex(ctx, filepath.Join(repoDir, "go"), gotypes.IndexPath(indexDir)); err != nil {
		return codeindex.BenchReport{}, err
	}
	names := make([]string, len(measured))
	for i, b := range measured {
		names[i] = b.Name()
	}
	fixtureLoc := loc
	fixtureLoc.indexDir = indexDir
	backends, err := codeindex.Resolve(names, codeBackendFactories(fixtureLoc, host.run, host.env))
	if err != nil {
		return codeindex.BenchReport{}, err
	}
	return codeindex.RunBench(ctx, codeindex.BenchRun{
		TruthSet: "independent",
		Backends: backends,
		Symbols:  truth,
		Rounds:   rounds,
		Now:      time.Now,
		LoadAvg:  loadAvg,
	})
}

func codeBenchNames(flagValue string, factories map[string]codeindex.Factory) []string {
	if strings.TrimSpace(flagValue) == "" {
		return codeindex.KnownBackends(factories)
	}
	return strings.Split(flagValue, ",")
}

func codeBenchProbes() []string {
	var probes []string
	for _, probe := range codeindextest.StandardProbes() {
		if probe.Request.Query == codeindex.QueryCallers {
			probes = append(probes, probe.Request.Arg)
		}
	}
	return probes
}

func hostLoadAverage(ctx context.Context, host codeIndexHost, readFile func(string) ([]byte, error)) float64 {
	return cyclespend.ReadLoadAverage(readFile, func() (string, error) {
		var out bytes.Buffer
		_, err := host.run(ctx, "sysctl", "", []string{"-n", "vm.loadavg"}, host.env, nil, &out, io.Discard)
		return out.String(), err
	})
}

func parseLoadAverage(raw string) float64 {
	return cyclespend.ParseLoadAverage(raw)
}
