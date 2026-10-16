package swarm

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func Validate(plan SwarmPlan) ValidationResult {
	if plan.IsFallback() {
		return ValidationResult{
			Collapse: true,
			Reason:   fallbackReason(plan),
		}
	}

	if dup := duplicateWorkerID(plan.Workers); dup != "" {
		return ValidationResult{
			Collapse: true,
			Reason:   fmt.Sprintf("duplicate worker_id %q → falling back to N=1", dup),
		}
	}

	switch plan.Mode {
	case ModeReader:
		return validateReader(plan)
	case ModeWriter:
		return validateWriter(plan)
	default:
		return ValidationResult{
			Collapse: true,
			Reason:   fmt.Sprintf("unknown swarm mode %q (expected writer|reader)", plan.Mode),
		}
	}
}

func validateWriter(plan SwarmPlan) ValidationResult {
	conflicts := detectConflicts(plan.Workers)
	if len(conflicts) > 0 {
		return ValidationResult{
			Collapse:  true,
			Reason:    fmt.Sprintf("writer partition not disjoint: %d file(s) claimed by multiple workers → falling back to N=1", len(conflicts)),
			Conflicts: conflicts,
		}
	}
	order, err := TopoOrder(plan.Workers)
	if err != nil {
		return ValidationResult{
			Collapse: true,
			Reason:   fmt.Sprintf("invalid merge DAG: %v → falling back to N=1", err),
		}
	}
	return ValidationResult{OK: true, MergeOrder: order}
}

func validateReader(plan SwarmPlan) ValidationResult {
	if _, err := TopoOrder(plan.Workers); err != nil {
		return ValidationResult{
			Collapse: true,
			Reason:   fmt.Sprintf("invalid reader DAG: %v → falling back to N=1", err),
		}
	}
	return ValidationResult{OK: true}
}

func detectConflicts(workers []WorkerSpec) []Conflict {
	owners := make(map[string]map[string]bool)
	for _, w := range workers {
		for _, f := range w.TargetFiles {
			norm := normalizePath(f)
			if norm == "" {
				continue
			}
			if owners[norm] == nil {
				owners[norm] = make(map[string]bool)
			}
			owners[norm][w.WorkerID] = true
		}
	}

	var conflicts []Conflict
	for file, set := range owners {
		if len(set) < 2 {
			continue
		}
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		conflicts = append(conflicts, Conflict{File: file, Workers: ids})
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].File < conflicts[j].File })
	return conflicts
}

func duplicateWorkerID(workers []WorkerSpec) string {
	seen := make(map[string]bool, len(workers))
	for _, w := range workers {
		if seen[w.WorkerID] {
			return w.WorkerID
		}
		seen[w.WorkerID] = true
	}
	return ""
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(filepath.Clean(p))
	return strings.ToLower(strings.TrimSuffix(p, "/"))
}

func fallbackReason(plan SwarmPlan) string {
	if !plan.Partitionable {
		if plan.Rationale != "" {
			return "planner declared non-partitionable: " + plan.Rationale
		}
		return "planner declared non-partitionable"
	}
	return fmt.Sprintf("only %d worker(s) — nothing to parallelize", len(plan.Workers))
}
