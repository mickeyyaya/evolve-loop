package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
)

func ClassifyLaneChanges(evolveDir string, cycle int, declared, changed []string, protected func(string) bool, warn io.Writer) []scopedelta.Entry {
	scope := scopedelta.Scope{
		Cycle: cycle, Declared: declared,
		Protected:  protectedAmong(changed, protected),
		LaneOthers: SiblingLaneFiles(evolveDir, cycle, warn),
	}
	entries := make([]scopedelta.Entry, 0, len(changed))
	for _, p := range changed {
		entries = append(entries, scopedelta.Classify(p, scopedelta.Declaration{}, scope, scopedelta.DefaultClosureRules()))
	}
	return entries
}

func protectedAmong(paths []string, member func(string) bool) []string {
	if member == nil {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(paths), func(p string) bool { return !member(p) })
}

func LaneDeclaredFiles(evolveDir string, cycle int) ([]string, error) {
	scan, err := inboxbatch.ScanDir(inboxbatch.ProcessingCycleDir(filepath.Join(evolveDir, "inbox"), cycle))
	if err != nil {
		return nil, fmt.Errorf("lane %d claims: %w", cycle, err)
	}
	if scan.HasUnreadable() {
		return nil, fmt.Errorf("lane %d claims are partial: %v", cycle, scan.Warnings)
	}
	items := scan.Items
	pin := committedset.LanePin(laneWorkspace(evolveDir, cycle))
	var files []string
	for _, it := range items {
		if pin == nil || slices.Contains(pin, it.ID) {
			files = append(files, it.Files...)
		}
	}
	return files, nil
}

func SiblingLaneFiles(evolveDir string, cycle int, warn io.Writer) []string {
	self, err := readLanePin(evolveDir, cycle)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(warn, "[scope-delta] WARN lane %d pin unreadable: %v; no sibling lane is known\n", cycle, err)
	}
	if self.GoalHash == "" {
		return nil
	}
	var files []string
	for _, dir := range inboxbatch.CycleDirs(filepath.Join(evolveDir, "runs")) {
		other, _ := inboxbatch.ParseProcessingCycle(filepath.Base(dir))
		if other == cycle {
			continue
		}
		pin, err := readLanePin(evolveDir, other)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(warn, "[scope-delta] WARN skip sibling cycle %d: %v\n", other, err)
			}
			continue
		}
		if pin.GoalHash != self.GoalHash {
			continue
		}
		declared, err := LaneDeclaredFiles(evolveDir, other)
		if err != nil {
			fmt.Fprintf(warn, "[scope-delta] WARN skip sibling cycle %d: %v\n", other, err)
			continue
		}
		files = append(files, declared...)
	}
	return files
}

func laneWorkspace(evolveDir string, cycle int) string {
	return inboxbatch.CycleDir(filepath.Join(evolveDir, "runs"), strconv.Itoa(cycle))
}

func readLanePin(evolveDir string, cycle int) (LaneScope, error) {
	var pin LaneScope
	body, err := os.ReadFile(filepath.Join(laneWorkspace(evolveDir, cycle), LaneScopeFile))
	if err != nil {
		return pin, err
	}
	if err := json.Unmarshal(body, &pin); err != nil {
		return pin, fmt.Errorf("parse %s: %w", LaneScopeFile, err)
	}
	return pin, nil
}
