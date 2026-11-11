package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
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
	report, err := codeindex.RunBench(ctx, codeindex.BenchRun{
		Backends:    measured,
		Symbols:     codeindex.SampleSymbols(pool, *symbols, codeBenchProbes()),
		Rounds:      *rounds,
		Now:         time.Now,
		LoadAvg:     func() float64 { return hostLoadAverage(ctx, host, os.ReadFile) },
		Root:        loc.indexDir,
		GrepSymbols: codeBenchGrepSymbols,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evolve code bench: %v\n", err)
		return codeExitFailed
	}
	fmt.Fprint(stdout, report.Table())
	return 0
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
	if raw, err := readFile("/proc/loadavg"); err == nil {
		return parseLoadAverage(string(raw))
	}
	var out bytes.Buffer
	if _, err := host.run(ctx, "sysctl", "", []string{"-n", "vm.loadavg"}, host.env, nil, &out, io.Discard); err != nil {
		return 0
	}
	return parseLoadAverage(out.String())
}

func parseLoadAverage(raw string) float64 {
	fields := strings.Fields(strings.Trim(strings.TrimSpace(raw), "{}"))
	if len(fields) == 0 {
		return 0
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return load
}
