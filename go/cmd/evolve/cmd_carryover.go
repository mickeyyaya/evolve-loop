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

// carryoverApplyCeiling is reported, never enforced: an apply that keeps a few
// extra live items still succeeds.
const carryoverApplyCeiling = 25

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

type carryoverApplyResult struct {
	Before    int
	After     int
	Dropped   int
	Clustered int
}

// runCarryover applies a reviewed keep/drop/cluster decisions file to the
// carryover todos, which the TTL prune cannot judge.
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
	// Validate before touching state.json so a bad file never half-mutates it.
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

// validateCarryoverDecisions requires a reason on every row, so no drop goes
// unjustified.
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

// applyCarryoverDecisions removes drop and cluster ids; cluster ids live on as
// sweep-group inbox items, so keeping them would count them twice. The doc must
// already be validated.
func applyCarryoverDecisions(statePath string, doc carryoverDecisionsDoc) (carryoverApplyResult, error) {
	remove := make(map[string]string, len(doc.Decisions))
	for _, d := range doc.Decisions {
		if d.Decision == "drop" || d.Decision == "cluster" {
			remove[d.ID] = d.Decision
		}
	}

	// A missing state file is a path mistake, not an empty apply.
	if _, err := os.Stat(statePath); err != nil {
		return carryoverApplyResult{}, fmt.Errorf("read state %s: %w", statePath, err)
	}

	// An unlocked pre-read only skips a no-op write; UpdateStateMap re-reads
	// under the lock, so it is never a correctness gate.
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

	var res carryoverApplyResult
	err := statemap.UpdateStateMap(statePath, func(state map[string]any) {
		entries, _ := state["carryoverTodos"].([]any)
		res.Before = len(entries)
		kept := make([]any, 0, len(entries))
		for _, e := range entries {
			m, ok := e.(map[string]any)
			if !ok {
				kept = append(kept, e)
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
