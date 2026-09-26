package deliverable

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// BadVerdictBaselineFile is the baseline sidecar's basename under .evolve, shared by the writer and the CLI.
const BadVerdictBaselineFile = "bad-verdict-baseline.jsonl"

// badVerdictEventType is filtered on because the sidecar is shared: a foreign event must not deflate the rate.
const badVerdictEventType = "bad_verdict_classified"

// BaselineSummary aggregates the baseline: Rate is Recoverable/Total (0, never NaN, when empty) and ByPattern holds only seen shapes.
type BaselineSummary struct {
	Total       int `json:"total"`
	Recoverable int `json:"recoverable"`
	// Saved is what the gate actually salvaged; Recoverable is only the potential, and refusals make them differ.
	Saved int `json:"saved"`
	// Malformed counts skipped unreadable records; it is in the envelope because a JSON consumer never sees the WARN.
	Malformed int                    `json:"malformed"`
	Rate      float64                `json:"rate"`
	ByPattern map[SalvagePattern]int `json:"by_pattern"`
}

// CountSalvageApplied counts salvage-applied records across every run; torn lines come back as malformed, and foreign events never count.
func CountSalvageApplied(r io.Reader) (saved int, malformed int, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec struct {
			EventType string `json:"event_type"`
		}
		if jsonErr := json.Unmarshal([]byte(line), &rec); jsonErr != nil {
			malformed++
			continue
		}
		if rec.EventType == salvageAppliedEventType {
			saved++
		}
	}
	// The unread remainder would make any count an undercount presented as a total.
	if scErr := sc.Err(); scErr != nil {
		return 0, malformed, fmt.Errorf("read %s: %w", salvageAppliedFile, scErr)
	}
	return saved, malformed, nil
}

// SalvageAppliedFile is the applied-salvage sidecar's basename under .evolve, shared by the writer and the CLI.
const SalvageAppliedFile = salvageAppliedFile

// SummarizeBadVerdictBaseline folds the baseline into a BaselineSummary; a torn line is a loud error, since skipping it would bias the rate.
func SummarizeBadVerdictBaseline(r io.Reader) (BaselineSummary, error) {
	sum := BaselineSummary{ByPattern: map[SalvagePattern]int{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec struct {
			EventType   string         `json:"event_type"`
			Recoverable bool           `json:"recoverable"`
			Pattern     SalvagePattern `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return BaselineSummary{}, fmt.Errorf(
				"%s line %d is not valid JSON: %w (a torn append is never skipped silently — dropping it would under-count the denominator and bias the rate)",
				BadVerdictBaselineFile, lineNo, err)
		}
		if rec.EventType != badVerdictEventType {
			continue
		}
		sum.Total++
		if rec.Recoverable {
			sum.Recoverable++
		}
		if rec.Pattern != SalvagePatternNone {
			sum.ByPattern[rec.Pattern]++
		}
	}
	if err := sc.Err(); err != nil {
		return BaselineSummary{}, fmt.Errorf("read %s: %w", BadVerdictBaselineFile, err)
	}
	if sum.Total > 0 {
		sum.Rate = float64(sum.Recoverable) / float64(sum.Total)
	}
	return sum, nil
}
