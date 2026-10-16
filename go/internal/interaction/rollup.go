package interaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const summarySchemaVersion = 1

type Summary struct {
	SchemaVersion int            `json:"schema_version"`
	Total         int            `json:"total"`
	ByKind        map[string]int `json:"by_kind"`
	ByResult      map[string]int `json:"by_result"`
	ByRung        map[string]int `json:"by_rung"`
	Decisions     int            `json:"decisions"`
	CostUSD       float64        `json:"cost_usd"`
}

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

var rollupMarshalIndent = json.MarshalIndent

func WriteRollup(workspace string) error {
	s, ok := Rollup(workspace)
	if !ok {
		return nil
	}
	b, err := rollupMarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return atomicwrite.Bytes(filepath.Join(workspace, "interaction-summary.json"), append(b, '\n'))
}
