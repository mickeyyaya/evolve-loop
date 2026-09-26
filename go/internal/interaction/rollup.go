package interaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const summarySchemaVersion = 1

// Summary is the per-cycle interaction rollup written to interaction-summary.json.
type Summary struct {
	SchemaVersion int `json:"schema_version"`
	// Total is the number of interaction outcomes recorded this cycle.
	Total int `json:"total"`
	// ByKind, ByResult and ByRung count outcomes per kind, result and ladder rung ("none" outside the ladder).
	ByKind   map[string]int `json:"by_kind"`
	ByResult map[string]int `json:"by_result"`
	ByRung   map[string]int `json:"by_rung"`
	// Decisions counts distinct non-empty DecisionIDs, so re-dispatches averted are computable.
	Decisions int `json:"decisions"`
	// CostUSD is the advisor spend attributed to interactions this cycle.
	CostUSD float64 `json:"cost_usd"`
}

// Rollup aggregates every *-interactions.ndjson under workspace, skipping corrupt lines.
// ok is false when there is nothing to summarize.
func Rollup(workspace string) (Summary, bool) {
	if workspace == "" {
		return Summary{}, false
	}
	paths, err := filepath.Glob(filepath.Join(workspace, "*-interactions.ndjson"))
	if err != nil || len(paths) == 0 {
		return Summary{}, false
	}
	sort.Strings(paths)
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
				continue
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

// WriteRollup writes interaction-summary.json beside the ledgers via tmp and rename;
// a workspace with nothing to summarize gets no file.
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
