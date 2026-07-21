// `evolve carryover apply-decisions` applies a reviewed keep/drop/cluster
// decisions file (authored by the cycle-997 carryover-consolidation-sweep pass)
// to state.json:carryoverTodos through the SANCTIONED locked read-modify-write
// path (flock.WithPathLock on the `<statePath>.lock` sidecar) — the same
// single-writer contract cmd_loop.go's auto-prune block and reset.go already
// honour. It is the missing link the inbox item names: the TTL prune machinery
// (failurelog.PruneExpiredCarryoverTodos) can only remove entries whose
// expiresAt is already past; it cannot act on a semantic keep/drop/cluster
// judgment. This command does.
//
// Semantics:
//   - `drop`    ids are removed from carryoverTodos (stale failure echoes /
//     landed duplicate shadows).
//   - `cluster` ids are ALSO removed — they have been re-filed as amortised
//     sweep-group inbox items (Task 3), so leaving them in carryoverTodos would
//     double-count them.
//   - `keep`    ids stay resident (genuinely-live small items).
//
// Guards:
//   - Every decision row MUST carry a non-empty reason. A single empty-reason
//     row aborts the whole apply BEFORE any lock is taken or byte is written —
//     the anti-hand-edit / anti-unjustified-drop contract (state.json is left
//     exactly as-is).
//   - The write is atomic (temp + rename) and serialized by the sidecar lock,
//     so concurrent callers never corrupt the array.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

// carryoverApplyCeiling is the convergence target the inbox item names (135 → ~25).
// Reported (not enforced by abort) so an apply that legitimately keeps a few
// extra live items still succeeds; the ceiling is a signal, not a gate.
const carryoverApplyCeiling = 25

// carryoverDecisionRow mirrors the on-disk schema the decisions artifact emits.
// ClusterGroup is required only when Decision=="cluster".
type carryoverDecisionRow struct {
	ID           string `json:"id"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	ClusterGroup string `json:"cluster_group"`
}

type carryoverDecisionsDoc struct {
	SourceCount int                    `json:"source_count"`
	Decisions   []carryoverDecisionRow `json:"decisions"`
}

// carryoverApplyResult reports what the apply did (rendered to the operator and
// asserted by tests).
type carryoverApplyResult struct {
	Before    int
	After     int
	Dropped   int
	Clustered int
}

// runCarryover implements `evolve carryover <apply-decisions>`.
func runCarryover(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve carryover: missing subcommand (apply-decisions)")
		return 10
	}
	switch args[0] {
	case "apply-decisions":
		return runCarryoverApplyDecisions(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve carryover: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runCarryoverApplyDecisions(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve carryover apply-decisions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var statePath, decisionsPath string
	var apply bool
	fs.StringVar(&statePath, "state", ".evolve/state.json", "path to state.json")
	fs.StringVar(&decisionsPath, "decisions", ".evolve/carryover-decisions-2026-07-21.json", "path to the reviewed decisions JSON")
	fs.BoolVar(&apply, "apply", false, "actually mutate state.json; without it the command reports the plan (dry-run)")
	if err := fs.Parse(args); err != nil {
		return 10
	}

	doc, err := loadCarryoverDecisions(decisionsPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve carryover: %v\n", err)
		return 10
	}
	// Validate BEFORE touching state.json so a bad decisions file never
	// half-mutates the live array.
	if err := validateCarryoverDecisions(doc); err != nil {
		fmt.Fprintf(stderr, "evolve carryover: %v\n", err)
		return 10
	}

	if !apply {
		drops, clusters := 0, 0
		for _, d := range doc.Decisions {
			switch d.Decision {
			case "drop":
				drops++
			case "cluster":
				clusters++
			}
		}
		fmt.Fprintf(stdout, "carryover apply-decisions (dry-run): would remove %d drop + %d cluster = %d ids; re-run with --apply\n",
			drops, clusters, drops+clusters)
		return 0
	}

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		fmt.Fprintf(stderr, "evolve carryover: %v\n", err)
		return 10
	}
	fmt.Fprintf(stdout, "carryover apply-decisions: carryoverTodos %d→%d (dropped %d, clustered-out %d; ceiling %d)\n",
		res.Before, res.After, res.Dropped, res.Clustered, carryoverApplyCeiling)
	if res.After > carryoverApplyCeiling {
		fmt.Fprintf(stderr, "evolve carryover: WARN: %d entries remain, above the ~%d ceiling\n", res.After, carryoverApplyCeiling)
	}
	return 0
}

func loadCarryoverDecisions(path string) (carryoverDecisionsDoc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return carryoverDecisionsDoc{}, fmt.Errorf("read decisions %s: %w", path, err)
	}
	var doc carryoverDecisionsDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return carryoverDecisionsDoc{}, fmt.Errorf("parse decisions %s: %w", path, err)
	}
	if len(doc.Decisions) == 0 {
		return carryoverDecisionsDoc{}, fmt.Errorf("decisions %s has an empty `decisions` array", path)
	}
	return doc, nil
}

// validateCarryoverDecisions rejects a decisions file that would license an
// unjustified or malformed mutation. Runs entirely in memory before any lock or
// write, so a rejection leaves state.json byte-identical.
func validateCarryoverDecisions(doc carryoverDecisionsDoc) error {
	seen := make(map[string]bool, len(doc.Decisions))
	for i, d := range doc.Decisions {
		if d.ID == "" {
			return fmt.Errorf("decision[%d] has an empty id", i)
		}
		if seen[d.ID] {
			return fmt.Errorf("decision id %q appears more than once", d.ID)
		}
		seen[d.ID] = true
		switch d.Decision {
		case "keep", "drop", "cluster":
		default:
			return fmt.Errorf("decision id %q has invalid decision %q (want keep|drop|cluster)", d.ID, d.Decision)
		}
		if strings.TrimSpace(d.Reason) == "" {
			return fmt.Errorf("decision id %q has an empty reason (every classification must justify itself)", d.ID)
		}
		if d.Decision == "cluster" && strings.TrimSpace(d.ClusterGroup) == "" {
			return fmt.Errorf("decision id %q is `cluster` but names no cluster_group", d.ID)
		}
	}
	return nil
}

// applyCarryoverDecisions removes the drop + cluster ids from
// state.json:carryoverTodos through the sanctioned locked RMW path. The doc is
// assumed already validated by validateCarryoverDecisions.
func applyCarryoverDecisions(statePath string, doc carryoverDecisionsDoc) (carryoverApplyResult, error) {
	remove := make(map[string]string, len(doc.Decisions)) // id -> decision (drop|cluster)
	for _, d := range doc.Decisions {
		if d.Decision == "drop" || d.Decision == "cluster" {
			remove[d.ID] = d.Decision
		}
	}

	// Fail loud on a missing state file (an operator command applying against
	// nothing is a path mistake, not an empty apply).
	if _, err := os.Stat(statePath); err != nil {
		return carryoverApplyResult{}, fmt.Errorf("read state %s: %w", statePath, err)
	}

	// Advisory pre-read: skip the write entirely (no revision/mtime churn)
	// when no decision id is present. statemap.UpdateStateMap re-reads
	// authoritatively under the CANONICAL lock, so this is purely a
	// no-op-write optimization, never a correctness gate.
	if pre, err := statemap.ReadStateMap(statePath); err == nil {
		found := false
		entries, _ := pre["carryoverTodos"].([]any)
		for _, e := range entries {
			if m, ok := e.(map[string]any); ok {
				if id, _ := m["id"].(string); remove[id] != "" {
					found = true
					break
				}
			}
		}
		if !found {
			return carryoverApplyResult{Before: len(entries), After: len(entries)}, nil
		}
	}

	// The locked RMW goes through statemap (cycle-999/1001 fixes): the path is
	// symlink-resolved so a worktree link writes THROUGH to canonical and
	// survives; the lock is taken on the resolved path (one lock per data
	// file, cross-tree); stateRevision auto-bumps and a stale write is refused.
	var res carryoverApplyResult
	err := statemap.UpdateStateMap(statePath, func(state map[string]any) {
		entries, _ := state["carryoverTodos"].([]any)
		res.Before = len(entries)
		kept := make([]any, 0, len(entries))
		for _, e := range entries {
			m, ok := e.(map[string]any)
			if !ok {
				kept = append(kept, e) // preserve un-modeled data
				continue
			}
			id, _ := m["id"].(string)
			switch remove[id] {
			case "drop":
				res.Dropped++
			case "cluster":
				res.Clustered++
			default:
				kept = append(kept, e)
			}
		}
		res.After = len(kept)
		state["carryoverTodos"] = kept
	})
	return res, err
}
