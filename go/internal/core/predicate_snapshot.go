package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const predicateSnapshotFile = "predicates-at-build.json"

type predicateBase struct {
	sources map[string]string
	fault   string
}

func sealPredicateBase(workspace, worktree string, cycle int) (map[string]string, error) {
	sources := cyclePredicateSources(worktree, cycle)
	if workspace == "" {
		return sources, nil
	}
	if err := writePredicateSnapshot(workspace, sources); err != nil {
		return nil, fmt.Errorf("seal predicate baseline: %w", err)
	}
	return sources, nil
}

func writePredicateSnapshot(workspace string, sources map[string]string) error {
	data, _ := json.Marshal(sources)
	path := filepath.Join(workspace, predicateSnapshotFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readPredicateSnapshot(workspace string) (map[string]string, string) {
	path := filepath.Join(workspace, predicateSnapshotFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}, "no predicate base at " + path + " for this Build entry"
	}
	sources := map[string]string{}
	if err := json.Unmarshal(data, &sources); err != nil {
		return map[string]string{}, "the predicate base at " + path + " does not parse: " + err.Error()
	}
	return sources, ""
}

func keptPredicateBase(cs CycleState, _ int) (predicateBase, error) {
	sources, fault := readPredicateSnapshot(cs.WorkspacePath)
	return predicateBase{sources: sources, fault: fault}, nil
}

func sealedPredicateBase(cs CycleState, cycle int) (predicateBase, error) {
	sources, err := sealPredicateBase(cs.WorkspacePath, cs.ActiveWorktree, cycle)
	return predicateBase{sources: sources}, err
}
