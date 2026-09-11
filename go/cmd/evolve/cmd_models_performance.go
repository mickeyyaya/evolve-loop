package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	evolog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

type modelPerformanceReport struct {
	Attempts         int                    `json:"attempts"`
	MalformedSkipped int                    `json:"malformed_skipped"`
	ReadWarnings     int                    `json:"read_warnings"`
	Groups           []llmcalls.Performance `json:"groups"`
}

func runModelsPerformance(args []string, stdout, stderr io.Writer) int {
	opts, ok := parseModelsFlags("performance", args, stderr)
	if !ok {
		return 10
	}
	records, skipped, warnings := collectModelAttempts(opts.EvolveDir)
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "evolve models performance: WARN: ledger read failed detail=%s\n", evolog.DiagnosticField(warning))
	}
	rows := llmcalls.Aggregate(records)
	if opts.AsJSON {
		report := modelPerformanceReport{
			Attempts: len(records), MalformedSkipped: skipped,
			ReadWarnings: len(warnings), Groups: rows,
		}
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve models performance: encode report failed detail=%s\n", evolog.DiagnosticField(err.Error()))
			return 1
		}
		_, _ = stdout.Write(append(encoded, '\n'))
		return 0
	}
	printModelPerformance(stdout, rows, len(records), skipped)
	return 0
}

func collectModelAttempts(evolveDir string) ([]llmcalls.Record, int, []string) {
	runsDir := filepath.Join(evolveDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, []string{err.Error()}
	}
	sort.Slice(entries, func(i, j int) bool {
		return cycleDirectoryNumber(entries[i].Name()) < cycleDirectoryNumber(entries[j].Name())
	})
	var records []llmcalls.Record
	var skipped int
	var warnings []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "cycle-") {
			continue
		}
		workspace := filepath.Join(runsDir, entry.Name())
		result, readErr := llmcalls.ReadWorkspace(workspace)
		if readErr != nil && !os.IsNotExist(readErr) {
			warnings = append(warnings, readErr.Error())
		}
		records = append(records, result.Records...)
		skipped += result.Skipped
	}
	return records, skipped, warnings
}

func cycleDirectoryNumber(name string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(name, "cycle-"))
	if err != nil {
		return int(^uint(0) >> 1)
	}
	return n
}

func printModelPerformance(w io.Writer, rows []llmcalls.Performance, attempts, skipped int) {
	fmt.Fprintf(w, "Model attempt performance (attempts=%d, malformed_skipped=%d)\n", attempts, skipped)
	if len(rows) == 0 {
		fmt.Fprintln(w, "No orchestration-owned model attempts recorded.")
		return
	}
	fmt.Fprintln(w, "CLI | dispatched model | requested | dispatch | measurement | timing scope | outcome | attempts | latency p50/p95 (coverage) | input/output | usage | output tok/s*")
	for _, row := range rows {
		rate := "-"
		if row.OutputTokensPerSecond != nil {
			rate = fmt.Sprintf("%.2f", *row.OutputTokensPerSecond)
		}
		fmt.Fprintf(w, "%s | %s | %s | %s | %s | %s | %s | %d | %s | %d/%d | %d measured, %d partial, %d unavailable, %d errors | %s\n",
			safePerformanceCell(row.CLI), safePerformanceCell(row.Model), safePerformanceCell(row.RequestedModel),
			safePerformanceCell(row.DispatchSource), safePerformanceCell(row.MeasurementSource),
			safePerformanceCell(row.TimingScope), safePerformanceCell(row.Outcome), row.Attempts,
			performanceLatency(row),
			row.Tokens.Input, row.Tokens.Output, row.UsageMeasured, row.UsagePartial,
			row.UsageUnavailable, row.ResolverErrors, rate)
	}
	fmt.Fprintln(w, "* amortized measured output tokens / full bridge-dispatch time; excludes partial and unavailable usage")
}

func performanceLatency(row llmcalls.Performance) string {
	if row.LatencySamples == 0 {
		return fmt.Sprintf("-/- (0 samples, %d unavailable)", row.LatencyUnavailable)
	}
	return fmt.Sprintf("%s/%s (%d samples, %d unavailable)",
		phasetiming.HumanMS(row.P50LatencyMS), phasetiming.HumanMS(row.P95LatencyMS),
		row.LatencySamples, row.LatencyUnavailable)
}

func safePerformanceCell(value string) string {
	const maxRunes = 80
	clean := make([]rune, 0, min(len(value), maxRunes))
	pendingSpace := false
	truncated := false
	for _, r := range value {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) || unicode.IsSpace(r) {
			pendingSpace = len(clean) > 0
			continue
		}
		needed := 1
		if pendingSpace {
			needed++
		}
		if len(clean)+needed > maxRunes {
			truncated = true
			break
		}
		if pendingSpace {
			clean = append(clean, ' ')
			pendingSpace = false
		}
		clean = append(clean, r)
	}
	if truncated && len(clean) > 0 {
		clean[len(clean)-1] = '…'
	}
	return string(clean)
}
