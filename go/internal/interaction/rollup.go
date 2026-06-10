package interaction

// rollup.go — the per-cycle aggregation over every per-phase interaction
// ledger in a workspace: <phase>-interactions.ndjson → interaction-summary.json
// (phase-timing.json's sibling). The orchestrator writes it from RunCycle's
// deferred persistence block; because the rollup READS the ndjson files, it
// aggregates bridge-subprocess records and orchestrator records uniformly.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// summarySchemaVersion versions interaction-summary.json for readers.
const summarySchemaVersion = 1

// Summary is the per-cycle interaction rollup (ADR-0045 §10(d): the
// rung-distribution acceptance metric reads ByRung shifting toward salvage).
type Summary struct {
	SchemaVersion int `json:"schema_version"`
	// Total is the number of interaction outcomes recorded this cycle.
	Total int `json:"total"`
	// ByKind / ByResult / ByRung count outcomes per Event.Kind, per
	// Outcome.Result, and per ladder rung ("none" for non-ladder
	// interactions).
	ByKind   map[string]int `json:"by_kind"`
	ByResult map[string]int `json:"by_result"`
	ByRung   map[string]int `json:"by_rung"`
	// Decisions counts distinct correction decisions (non-empty
	// DecisionIDs), so re-dispatches AVERTED are computable per §10(a).
	Decisions int `json:"decisions"`
	// CostUSD is the advisor spend attributed to interactions this cycle.
	CostUSD float64 `json:"cost_usd"`
}

// Rollup aggregates every *-interactions.ndjson under workspace. ok=false
// when there is nothing to summarize (no files, empty workspace). Corrupt
// lines are skipped — the read side of a crash-safe ledger is tolerant.
func Rollup(workspace string) (Summary, bool) {
	if workspace == "" {
		return Summary{}, false
	}
	paths, err := filepath.Glob(filepath.Join(workspace, "*-interactions.ndjson"))
	if err != nil || len(paths) == 0 {
		return Summary{}, false
	}
	sort.Strings(paths) // deterministic aggregation order
	s := Summary{
		SchemaVersion: summarySchemaVersion,
		ByKind:        map[string]int{},
		ByResult:      map[string]int{},
		ByRung:        map[string]int{},
	}
	decisions := map[string]struct{}{}
	for _, p := range paths {
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			continue
		}
		for _, ln := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			var out Outcome
			if jerr := json.Unmarshal([]byte(ln), &out); jerr != nil {
				continue // tolerant reader: skip corrupt lines
			}
			s.Total++
			s.ByKind[out.Kind]++
			s.ByResult[out.Result]++
			rung := out.Rung
			if rung == "" {
				rung = "none"
			}
			s.ByRung[rung]++
			if out.DecisionID != "" {
				decisions[out.DecisionID] = struct{}{}
			}
			s.CostUSD += out.CostUSD
		}
	}
	if s.Total == 0 {
		return Summary{}, false
	}
	s.Decisions = len(decisions)
	return s, true
}

// WriteRollup writes interaction-summary.json beside the ledgers when there
// is anything to summarize; a workspace with no interactions stays clean (no
// empty-noise files). Best-effort atomic (tmp + rename).
func WriteRollup(workspace string) error {
	s, ok := Rollup(workspace)
	if !ok {
		return nil
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(workspace, "interaction-summary.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
