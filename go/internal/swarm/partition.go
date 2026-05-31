package swarm

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Validate is the pure, mode-aware partition validator — the correctness crux of
// the writer swarm. It runs after the planner and before any worker launches.
// It never touches the filesystem or git, so it is exhaustively table-testable.
//
//   - WRITER mode (strict): every target file must be owned by at most one
//     worker. ANY overlap → Collapse to N=1 (a writer swarm runs only on a
//     provably-disjoint plan; per ADR-0032, "when not cleanly independent, do
//     NOT swarm writers"). The depends_on DAG must also be acyclic with no
//     dangling refs; the resulting topological order is the merge-train order.
//   - READER mode (lenient): overlap is allowed (read overlap wastes tokens,
//     never corrupts), so the disjointness check is skipped. Only N≥2 and DAG
//     validity (usually a no-op — readers rarely declare deps) are checked.
//
// A plan that IsFallback() (planner declared non-partitionable, or <2 workers)
// short-circuits to Collapse without inspecting file ownership.
func Validate(plan SwarmPlan) ValidationResult {
	if plan.IsFallback() {
		return ValidationResult{
			Collapse: true,
			Reason:   fallbackReason(plan),
		}
	}

	// Duplicate worker_id is a planner bug that would silently dedupe in the
	// ownership/indegree maps (a 2-worker plan could validate OK with a 1-entry
	// merge order). Reject before any mode-specific check.
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

// validateWriter enforces strict disjoint file ownership + a valid merge DAG.
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

// validateReader checks only DAG validity (overlap is allowed for readers).
func validateReader(plan SwarmPlan) ValidationResult {
	if _, err := TopoOrder(plan.Workers); err != nil {
		// Readers rarely declare deps; a bad one is still a planner bug worth
		// collapsing on rather than silently ignoring.
		return ValidationResult{
			Collapse: true,
			Reason:   fmt.Sprintf("invalid reader DAG: %v → falling back to N=1", err),
		}
	}
	return ValidationResult{OK: true}
}

// detectConflicts returns every file claimed by more than one worker, after
// normalizing paths (clean + slash). Deterministic: conflicts sorted by file,
// and the offending worker IDs within each conflict sorted ascending.
func detectConflicts(workers []WorkerSpec) []Conflict {
	owners := make(map[string]map[string]bool) // normalized file → set(workerID)
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

// duplicateWorkerID returns the first worker_id that appears more than once, or
// "" if all IDs are unique.
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

// normalizePath makes two spellings of the same repo file compare equal so an
// overlap can't slip through via "./a.go" vs "a.go". Repo-relative, forward
// slashes, no trailing slash, and CASE-FOLDED — because the dev hosts (macOS
// HFS+/APFS default, Windows) are case-insensitive, so "Foo/A.go" and
// "foo/a.go" are the same file and must collide in the disjointness check.
// (Absolute/escape rejection is enforced later by the post-build git-status
// guard, which runs against the real worktree.)
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
