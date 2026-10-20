package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const servedModelUnreported = "unreported"

type cycleModelRow struct {
	Phase       string `json:"phase"`
	Attempt     int    `json:"attempt"`
	CLI         string `json:"cli"`
	Tier        string `json:"tier"`
	Selector    string `json:"selector"`
	ServedModel string `json:"served_model"`
}

func runCycleModels(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("evolve cycle models", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		cycle       string
		jsonOut     bool
	)
	flags.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	flags.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	flags.StringVar(&cycle, "cycle", "", "cycle number whose llm-calls.ndjson to read (default: the latest cycle with call records)")
	flags.BoolVar(&jsonOut, "json", false, "emit the rows as JSON")
	if err := flags.Parse(args); err != nil {
		return 10
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, nil)
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	label, rows, err := loadCycleModelRows(filepath.Join(evolveDir, "runs"), cycle)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle models: %v\n", err)
		return 1
	}
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			fmt.Fprintf(stderr, "evolve cycle models: encode: %v\n", err)
			return 1
		}
		return 0
	}
	renderCycleModels(stdout, label, rows)
	return 0
}

func loadCycleModelRows(runsDir, cycle string) (string, []cycleModelRow, error) {
	workspace, label := resolveModelsWorkspace(runsDir, cycle)
	if workspace == "" {
		return "", nil, fmt.Errorf("no %s under %s", llmcalls.Filename, runsDir)
	}
	result, err := llmcalls.ReadWorkspace(workspace)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", nil, fmt.Errorf("read %s: %w", llmcalls.Path(workspace), err)
	}
	if len(result.Records) == 0 {
		return "", nil, fmt.Errorf("%s holds no call records (%s, %d unparseable lines skipped)",
			label, llmcalls.Path(workspace), result.Skipped)
	}
	return label, cycleModelRows(result.Records), nil
}

func resolveModelsWorkspace(runsDir, cycle string) (workspace, label string) {
	if cycle != "" {
		label = "cycle-" + cycle
		return filepath.Join(runsDir, label), label
	}
	matches, _ := filepath.Glob(filepath.Join(runsDir, "cycle-*", llmcalls.Filename))
	best, bestN := "", -1
	for _, m := range matches {
		dir := filepath.Base(filepath.Dir(m))
		if n := cycleNumber(dir); n > bestN {
			bestN, best = n, dir
		}
	}
	if best == "" {
		return "", ""
	}
	return filepath.Join(runsDir, best), best
}

func cycleModelRows(records []llmcalls.Record) []cycleModelRow {
	rows := make([]cycleModelRow, 0, len(records))
	for _, rec := range records {
		selector, _ := rec.ModelIdentity()
		served := rec.ServedModel
		if served == "" {
			served = servedModelUnreported
		}
		rows = append(rows, cycleModelRow{
			Phase: rec.Phase, Attempt: rec.Attempt, CLI: rec.CLI,
			Tier: rec.Model, Selector: selector, ServedModel: served,
		})
	}
	return rows
}

func renderCycleModels(w io.Writer, label string, rows []cycleModelRow) {
	fmt.Fprintf(w, "Cycle models — %s\n\n", label)
	fmt.Fprintf(w, "%-22s %-3s %-16s %-10s %-28s %s\n", "PHASE", "ATT", "CLI", "TIER", "SELECTOR", "SERVED")
	for _, r := range rows {
		fmt.Fprintf(w, "%-22s %-3d %-16s %-10s %-28s %s\n",
			truncate(r.Phase, 22), r.Attempt, r.CLI, r.Tier, r.Selector, r.ServedModel)
	}
}
