package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

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
	case "list":
		return runCarryoverList(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve carryover: unknown subcommand %q\n", args[0])
		return 10
	}
}

type carryoverListItem struct {
	ID             string `json:"id"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	Age            int    `json:"age"`
	ExpiresAt      string `json:"expiresAt"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
	Priority       string `json:"priority"`
}

func runCarryoverList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve carryover list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		jsonOutput bool
		sortMode   string
		limit      int
		statePath  string
	)
	fs.BoolVar(&jsonOutput, "json", false, "output carryover items as JSON array")
	fs.StringVar(&sortMode, "sort", "", "sort order: age or unpicked")
	fs.IntVar(&limit, "limit", 0, "limit output to N items")
	fs.StringVar(&statePath, "state", ".evolve/state.json", "path to state.json")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if sortMode != "" && sortMode != "age" && sortMode != "unpicked" {
		fmt.Fprintf(stderr, "evolve carryover: unknown sort %q (want age|unpicked)\n", sortMode)
		return 10
	}
	if limit < 0 {
		fmt.Fprintf(stderr, "evolve carryover: limit must be non-negative: %d\n", limit)
		return 10
	}
	state, err := statemap.ReadStateMap(statePath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve carryover: %v\n", err)
		return 2
	}
	items := readCarryoverItems(state)
	items = sortAndLimitCarryoverItems(items, sortMode, limit)
	return renderCarryoverItems(items, jsonOutput, stdout, stderr)
}

func readCarryoverItems(state map[string]any) []carryoverListItem {
	lastCycle := 0
	if v, ok := state["lastCycleNumber"].(float64); ok {
		lastCycle = int(v)
	} else if v, ok := state["lastCycleNumber"].(int); ok {
		lastCycle = v
	}
	rawEntries, ok := state["carryoverTodos"].([]any)
	if !ok {
		return []carryoverListItem{}
	}
	items := make([]carryoverListItem, 0, len(rawEntries))
	for _, e := range rawEntries {
		if m, ok := e.(map[string]any); ok {
			items = append(items, parseCarryoverListItem(m, lastCycle))
		}
	}
	return items
}

func parseCarryoverListItem(m map[string]any, lastCycle int) carryoverListItem {
	var item carryoverListItem
	if v, ok := m["id"].(string); ok {
		item.ID = v
	}
	if v, ok := m["first_seen_cycle"].(float64); ok {
		item.FirstSeenCycle = int(v)
	} else if v, ok := m["first_seen_cycle"].(int); ok {
		item.FirstSeenCycle = v
	}
	if v, ok := m["cycles_unpicked"].(float64); ok {
		item.CyclesUnpicked = int(v)
	} else if v, ok := m["cycles_unpicked"].(int); ok {
		item.CyclesUnpicked = v
	}
	if v, ok := m["expiresAt"].(string); ok {
		item.ExpiresAt = v
	}
	if v, ok := m["priority"].(string); ok {
		item.Priority = v
	}
	if item.FirstSeenCycle > 0 && lastCycle >= item.FirstSeenCycle {
		item.Age = lastCycle - item.FirstSeenCycle
	}
	return item
}

func sortAndLimitCarryoverItems(items []carryoverListItem, sortMode string, limit int) []carryoverListItem {
	switch sortMode {
	case "age":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Age > items[j].Age
		})
	case "unpicked":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].CyclesUnpicked > items[j].CyclesUnpicked
		})
	}
	if limit > 0 && limit < len(items) {
		return items[:limit]
	}
	return items
}

func renderCarryoverItems(items []carryoverListItem, jsonOutput bool, stdout, stderr io.Writer) int {
	if jsonOutput {
		raw, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve carryover: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(raw))
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tPRIORITY\tAGE\tUNPICKED\tEXPIRES")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", item.ID, item.Priority, item.Age, item.CyclesUnpicked, item.ExpiresAt)
	}
	tw.Flush()
	return 0
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

	if res, ok := carryoverNoOpFastPath(statePath, remove); ok {
		return res, nil
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

// carryoverNoOpFastPath does an unlocked pre-read to short-circuit a no-op
// apply; UpdateStateMap re-reads under the lock, so this is never a
// correctness gate. ok=false (on a read error or a matching removal id)
// means the caller must fall through to the locked update.
func carryoverNoOpFastPath(statePath string, remove map[string]string) (carryoverApplyResult, bool) {
	if _, err := os.Stat(statePath); err != nil {
		return carryoverApplyResult{}, false
	}
	pre, err := statemap.ReadStateMap(statePath)
	if err != nil {
		return carryoverApplyResult{}, false
	}
	entries, _ := pre["carryoverTodos"].([]any)
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			if id, _ := m["id"].(string); remove[id] != "" {
				return carryoverApplyResult{}, false
			}
		}
	}
	return carryoverApplyResult{Before: len(entries), After: len(entries)}, true
}
