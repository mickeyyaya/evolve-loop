// Package cyclecost computes per-phase and per-cycle cost from the phase event streams and usage sidecars.
// See docs/architecture/packages/internal-cyclecost.md.
package cyclecost

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PhaseCost struct {
	Phase                    string  `json:"phase,omitempty"`
	CostUSD                  float64 `json:"cost_usd"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	InputTokens              int64   `json:"input_tokens"`
}

type Summary struct {
	Cycle  int         `json:"cycle"`
	Phases []PhaseCost `json:"phases"`
	Total  PhaseCost   `json:"total"`
}

var ErrNoLogs = errors.New("cyclecost: no *-events.ndjson files in workspace")

var ErrNoWorkspace = errors.New("cyclecost: workspace does not exist")

var globFn = filepath.Glob

var maxScannerBufBytes = 1 << 24

func SummarizeCycle(workspace string, cycle int) (Summary, error) {
	if err := statWorkspace(workspace); err != nil {
		return Summary{Cycle: cycle}, err
	}

	logs, err := globFn(filepath.Join(workspace, "*-events.ndjson"))
	if err != nil {
		return Summary{Cycle: cycle}, fmt.Errorf("glob: %w", err)
	}
	sort.Strings(logs)

	sidecars := sidecarPhaseCosts(workspace)
	if len(logs) == 0 && len(sidecars) == 0 {
		return Summary{Cycle: cycle}, ErrNoLogs
	}

	summary := Summary{Cycle: cycle}
	fromEvents := map[string]bool{}
	for _, log := range logs {
		pc, ok := parseEventsLog(log)
		if !ok {
			continue
		}
		summary.Phases = append(summary.Phases, pc)
		fromEvents[pc.Phase] = true
	}

	for _, pc := range sidecars {
		if fromEvents[pc.Phase] {
			continue
		}
		summary.Phases = append(summary.Phases, pc)
	}
	sort.Slice(summary.Phases, func(i, j int) bool { return summary.Phases[i].Phase < summary.Phases[j].Phase })
	summary.Total = sumPhaseCosts(summary.Phases)
	return summary, nil
}

func statWorkspace(workspace string) error {
	info, err := os.Stat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoWorkspace
		}
		return fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return ErrNoWorkspace
	}
	return nil
}

func sumPhaseCosts(phases []PhaseCost) PhaseCost {
	var total PhaseCost
	for _, pc := range phases {
		total.CostUSD += pc.CostUSD
		total.CacheReadInputTokens += pc.CacheReadInputTokens
		total.CacheCreationInputTokens += pc.CacheCreationInputTokens
		total.OutputTokens += pc.OutputTokens
		total.InputTokens += pc.InputTokens
	}
	return total
}

func ParseEventsLog(logPath string) (PhaseCost, bool) {
	return parseEventsLog(logPath)
}

type eventEnvelope struct {
	Kind string `json:"kind"`
	Data struct {
		CostUSD float64 `json:"cost_usd"`
		Tokens  struct {
			In     int64 `json:"in"`
			Out    int64 `json:"out"`
			CacheR int64 `json:"cache_r"`
			CacheC int64 `json:"cache_c"`
		} `json:"tokens"`
	} `json:"data"`
}

func parseEventsLog(logPath string) (PhaseCost, bool) {
	f, err := os.Open(logPath)
	if err != nil {
		return PhaseCost{}, false
	}
	defer func() { _ = f.Close() }()

	phase := strings.TrimSuffix(filepath.Base(logPath), "-events.ndjson")

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<10), maxScannerBufBytes)

	last, found := lastResultEnvelope(scanner)
	if err := scanner.Err(); err != nil {
		return PhaseCost{}, false
	}
	if !found {
		return PhaseCost{}, false
	}

	return PhaseCost{
		Phase:                    phase,
		CostUSD:                  last.Data.CostUSD,
		CacheReadInputTokens:     last.Data.Tokens.CacheR,
		CacheCreationInputTokens: last.Data.Tokens.CacheC,
		OutputTokens:             last.Data.Tokens.Out,
		InputTokens:              last.Data.Tokens.In,
	}, true
}

func lastResultEnvelope(scanner *bufio.Scanner) (eventEnvelope, bool) {
	var last eventEnvelope
	var found bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.Contains(line, `"kind":"result"`) {
			continue
		}
		var ev eventEnvelope
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Kind == "result" {
			last = ev
			found = true
		}
	}
	return last, found
}

const usageSidecarSuffix = "-usage.json"

const tokenUsageSidecarName = "token-usage.json"

type usageSidecar struct {
	Phase   string  `json:"phase"`
	CostUSD float64 `json:"cost_usd"`
	Tokens  struct {
		Input      int64 `json:"input"`
		Output     int64 `json:"output"`
		CacheRead  int64 `json:"cache_read"`
		CacheWrite int64 `json:"cache_write"`
	} `json:"tokens"`
}

func sidecarPhaseCosts(workspace string) []PhaseCost {
	matches, err := globFn(filepath.Join(workspace, "*"+usageSidecarSuffix))
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)
	var out []PhaseCost
	for _, m := range matches {
		if filepath.Base(m) == tokenUsageSidecarName {
			continue
		}
		pc, ok := parseUsageSidecar(m)
		if !ok {
			continue
		}
		out = append(out, pc)
	}
	return out
}

func parseUsageSidecar(path string) (PhaseCost, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PhaseCost{}, false
	}
	var s usageSidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return PhaseCost{}, false
	}
	if s.Phase == "" {
		return PhaseCost{}, false
	}
	return PhaseCost{
		Phase:                    s.Phase,
		CostUSD:                  s.CostUSD,
		CacheReadInputTokens:     s.Tokens.CacheRead,
		CacheCreationInputTokens: s.Tokens.CacheWrite,
		OutputTokens:             s.Tokens.Output,
		InputTokens:              s.Tokens.Input,
	}, true
}
