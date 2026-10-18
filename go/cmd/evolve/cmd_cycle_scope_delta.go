package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	scopeDeltaClean   = 0
	scopeDeltaOutside = 1
	scopeDeltaIOError = 2
	scopeDeltaUsage   = 10
)

type scopeDeltaRow struct {
	Path        string                 `json:"path"`
	Class       scopedelta.Class       `json:"class"`
	Disposition scopedelta.Disposition `json:"disposition"`
}

type scopeDeltaReport struct {
	Cycle   int                `json:"cycle"`
	Summary scopedelta.Summary `json:"summary"`
	Entries []scopeDeltaRow    `json:"entries"`
}

func runCycleScopeDelta(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle scope-delta", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		cycle       int
		jsonOut     bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.IntVar(&cycle, "cycle", 0, "the cycle whose changed paths to classify (or pass it positionally)")
	fs.BoolVar(&jsonOut, "json", false, "emit {cycle, summary, entries} as JSON")
	if err := fs.Parse(args); err != nil {
		return scopeDeltaUsage
	}
	cycle, err := scopeDeltaCycle(fs, cycle)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle scope-delta: %v\n", err)
		return scopeDeltaUsage
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle scope-delta: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	report, err := buildScopeDeltaReport(evolveDir, cycle, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle scope-delta: %v\n", err)
		return scopeDeltaIOError
	}
	if err := writeScopeDeltaReport(stdout, report, jsonOut); err != nil {
		fmt.Fprintf(stderr, "evolve cycle scope-delta: write: %v\n", err)
		return scopeDeltaIOError
	}
	if report.Summary.Carved > 0 || report.Summary.Refused > 0 {
		return scopeDeltaOutside
	}
	return scopeDeltaClean
}

func scopeDeltaCycle(fs *flag.FlagSet, cycle int) (int, error) {
	flagSet := false
	fs.Visit(func(f *flag.Flag) { flagSet = flagSet || f.Name == "cycle" })
	switch {
	case fs.NArg() > 1:
		return 0, fmt.Errorf("unexpected arguments %q", fs.Args()[1:])
	case fs.NArg() == 1 && flagSet:
		return 0, fmt.Errorf("cycle given both as --cycle and positionally")
	case fs.NArg() == 1:
		n, err := strconv.Atoi(fs.Arg(0))
		if err != nil {
			return 0, fmt.Errorf("cycle %q is not a number", fs.Arg(0))
		}
		cycle = n
	case !flagSet:
		return 0, fmt.Errorf("missing cycle (usage: evolve cycle scope-delta --cycle N [--json])")
	}
	if cycle < 1 {
		return 0, fmt.Errorf("cycle must be a positive integer, got %d", cycle)
	}
	return cycle, nil
}

func buildScopeDeltaReport(evolveDir string, cycle int, stderr io.Writer) (scopeDeltaReport, error) {
	state, err := readLaneState(filepath.Join(evolveDir, "runs"), cycle)
	if err != nil {
		return scopeDeltaReport{}, err
	}
	declared, err := core.LaneDeclaredFiles(evolveDir, cycle)
	if err != nil {
		return scopeDeltaReport{}, err
	}
	changed, err := laneChangedPaths(state)
	if err != nil {
		return scopeDeltaReport{}, err
	}
	entries := core.ClassifyLaneChanges(evolveDir, cycle, declared, changed, guards.IsProtectedSurface, stderr)
	rows := make([]scopeDeltaRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, scopeDeltaRow{Path: e.Path, Class: e.Class, Disposition: e.Disposition})
	}
	return scopeDeltaReport{Cycle: cycle, Summary: scopedelta.Summarize(entries), Entries: rows}, nil
}

func readLaneState(runsDir string, cycle int) (cyclestate.CycleState, error) {
	path := filepath.Join(inboxbatch.CycleDir(runsDir, strconv.Itoa(cycle)), "cycle-state.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return cyclestate.CycleState{}, fmt.Errorf("read cycle %d state: %w", cycle, err)
	}
	var state cyclestate.CycleState
	if err := json.Unmarshal(body, &state); err != nil {
		return cyclestate.CycleState{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return state, nil
}

func laneChangedPaths(state cyclestate.CycleState) ([]string, error) {
	if state.ActiveWorktree == "" || state.WorktreeBaseSHA == "" {
		return nil, errors.New("cycle state names no active_worktree and worktree_base_sha to diff")
	}
	out, err := sysexec.Command(context.Background(), "git", "-C", state.ActiveWorktree, "diff", "--name-only", state.WorktreeBaseSHA, "--").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s in %s: %w", state.WorktreeBaseSHA, state.ActiveWorktree, err)
	}
	return strings.Fields(string(out)), nil
}

func writeScopeDeltaReport(w io.Writer, report scopeDeltaReport, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	s := report.Summary
	if _, err := fmt.Fprintf(w, "cycle %d scope delta: %d changed, kept=%d carved=%d refused=%d\n", report.Cycle, len(report.Entries), s.Kept, s.Carved, s.Refused); err != nil {
		return err
	}
	for _, r := range report.Entries {
		if _, err := fmt.Fprintf(w, "  %-13s %-7s %s\n", r.Class, r.Disposition, r.Path); err != nil {
			return err
		}
	}
	return nil
}
