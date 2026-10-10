package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const MaxEntries = 50

var ErrStateMissing = errors.New("failurelog: state.json missing")

type RecordRequest struct {
	Cycle          int
	Classification string
	ReportPath     string
	Summary        string
	Now            time.Time
}

type Recorded struct {
	Cycle          int            `json:"cycle"`
	Classification Classification `json:"classification"`
	Summary        string         `json:"summary,omitempty"`
	RecordedAt     string         `json:"recordedAt"`
	ExpiresAt      string         `json:"expiresAt"`
}

func Record(statePath, runsDir string, req RecordRequest) (Recorded, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	class := NormalizeLegacy(req.Classification)

	state, err := loadRecordState(statePath)
	if err != nil {
		return Recorded{}, err
	}

	entry := Recorded{
		Cycle:          req.Cycle,
		Classification: class,
		Summary:        resolveRecordSummary(req, runsDir),
		RecordedAt:     now.UTC().Format(time.RFC3339),
		ExpiresAt:      ComputeExpiresAt(class, now),
	}
	appendFailedApproach(state, entry, req.Cycle)

	if err := atomicWriteJSON(statePath, state); err != nil {
		return Recorded{}, fmt.Errorf("failurelog: write state: %w", err)
	}
	return entry, nil
}

func loadRecordState(statePath string) (map[string]any, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w at %s", ErrStateMissing, statePath)
		}
		return nil, fmt.Errorf("failurelog: read state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("failurelog: parse state: %w", err)
	}
	return state, nil
}

func resolveRecordSummary(req RecordRequest, runsDir string) string {
	if req.Summary != "" {
		return req.Summary
	}
	report := req.ReportPath
	if report == "" && runsDir != "" {
		report = filepath.Join(runsDir, fmt.Sprintf("cycle-%d", req.Cycle), "orchestrator-report.md")
	}
	if report == "" {
		return ""
	}
	summary := extractSummary(report)
	if strings.TrimSpace(summary) == "" {
		summary = extractSummaryForCycle(filepath.Dir(report))
	}
	return summary
}

func appendFailedApproach(state map[string]any, entry Recorded, cycle int) {
	existing, _ := state["failedApproaches"].([]any)
	existing = append(existing, mustMarshalToAny(entry))
	if len(existing) > MaxEntries {
		existing = existing[len(existing)-MaxEntries:]
	}
	state["failedApproaches"] = existing

	if cur, _ := state["lastCycleNumber"].(float64); float64(cycle) > cur {
		state["lastCycleNumber"] = float64(cycle)
	}
}

var summaryFallbacks = []string{"orchestrator-report.md", "audit-report.md", "build-report.md"}

func extractSummaryForCycle(workspace string) string {
	if workspace == "" {
		return ""
	}
	for _, name := range summaryFallbacks {
		if s := extractSummary(filepath.Join(workspace, name)); strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

const (
	summaryMaxLines = 8
	summaryMaxBytes = 400
)

func isSummarySectionHeader(line string) bool {
	return strings.HasPrefix(line, "## Failure") ||
		strings.HasPrefix(line, "## Verdict") ||
		strings.HasPrefix(line, "## Phase Outcomes")
}

func extractSummary(reportPath string) string {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")

	var out []string
	capturing := false
	for _, line := range lines {
		if capturing {
			if len(out) > 0 && strings.HasPrefix(line, "## ") {
				break
			}
			out = append(out, line)
			if len(out) >= summaryMaxLines {
				break
			}
			continue
		}
		capturing = isSummarySectionHeader(line)
	}
	if len(out) == 0 {
		return ""
	}
	joined := strings.Join(strings.Fields(strings.Join(out, " ")), " ")
	if len(joined) > summaryMaxBytes {
		joined = joined[:summaryMaxBytes]
	}
	return joined
}

func mustMarshalToAny(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

var atomicWriteJSON = func(path string, state map[string]any) error {
	return atomicwrite.JSON(statemap.ResolveWriteTarget(path), state)
}
