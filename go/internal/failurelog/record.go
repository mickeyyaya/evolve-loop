package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxEntries caps state.json:failedApproaches at this many entries.
// Newest-wins FIFO trim when appending: keep the last MaxEntries.
// Ports the bash `if length > 50 then .[length-50:]` slice.
const MaxEntries = 50

// ErrStateMissing is returned by Record when state.json doesn't exist.
// Cmd_loop treats this as a soft WARN (matches bash dispatcher line
// 647) — pre-flight should have created the file, but if it didn't,
// recording is a no-op rather than a hard halt.
var ErrStateMissing = errors.New("failurelog: state.json missing")

// RecordRequest is the input to Record.
type RecordRequest struct {
	// Cycle is the cycle number that failed.
	Cycle int
	// Classification is the raw verdict from cycleclassify.Classify
	// OR a legacy string. NormalizeLegacy is applied before write.
	Classification string
	// ReportPath is the absolute path to the cycle's
	// orchestrator-report.md. Optional — empty leaves Summary blank.
	ReportPath string
	// Now is the timestamp the failure happened at. Defaults to
	// time.Now().UTC() when zero.
	Now time.Time
}

// Recorded captures what got written. Useful for cmd_loop's stderr
// log line and for tests.
type Recorded struct {
	Cycle          int            `json:"cycle"`
	Classification Classification `json:"classification"`
	Summary        string         `json:"summary,omitempty"`
	RecordedAt     string         `json:"recordedAt"`
	ExpiresAt      string         `json:"expiresAt"`
}

// Record appends a failed-cycle entry to state.json:failedApproaches[],
// FIFO-trims to MaxEntries, advances state.json:lastCycleNumber, and
// writes both updates atomically via tmp+mv.
//
// Returns the entry that was persisted. Returns an error if state.json
// is missing, unreadable, or unwritable — the bash equivalent treats
// unwritable state.json as a FATAL halt because every retry would hit
// the same workspace and overwrite diagnostic evidence.
//
// statePath is typically <projectRoot>/.evolve/state.json. runsDir is
// where cycle-<N>/orchestrator-report.md lives (used for summary
// extraction when req.ReportPath is empty).
func Record(statePath, runsDir string, req RecordRequest) (Recorded, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	class := NormalizeLegacy(req.Classification)

	// Load state.json. If missing, mirror the bash behavior (log WARN,
	// return without recording — we don't auto-create state.json
	// because the dispatcher's preflight is responsible for that).
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Recorded{}, fmt.Errorf("%w at %s", ErrStateMissing, statePath)
		}
		return Recorded{}, fmt.Errorf("failurelog: read state: %w", err)
	}

	// Decode as map[string]any so we preserve unknown top-level keys.
	// state.json carries many fields not modeled in core.State.
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return Recorded{}, fmt.Errorf("failurelog: parse state: %w", err)
	}

	// Resolve summary: explicit path > runsDir-derived path > "".
	report := req.ReportPath
	if report == "" && runsDir != "" {
		report = filepath.Join(runsDir, fmt.Sprintf("cycle-%d", req.Cycle), "orchestrator-report.md")
	}
	summary := ""
	if report != "" {
		summary = extractSummary(report)
	}

	entry := Recorded{
		Cycle:          req.Cycle,
		Classification: class,
		Summary:        summary,
		RecordedAt:     now.UTC().Format(time.RFC3339),
		ExpiresAt:      ComputeExpiresAt(class, now),
	}

	// Append + FIFO trim.
	existing, _ := state["failedApproaches"].([]any)
	existing = append(existing, mustMarshalToAny(entry))
	if len(existing) > MaxEntries {
		existing = existing[len(existing)-MaxEntries:]
	}
	state["failedApproaches"] = existing

	// Advance lastCycleNumber so the next attempt uses a fresh
	// workspace. Bash dispatcher does this in a separate jq pass; we
	// merge it with the failedApproaches update to atomicize both.
	state["lastCycleNumber"] = float64(req.Cycle)

	// Atomic write via tmp+mv.
	if err := atomicWriteJSON(statePath, state); err != nil {
		return Recorded{}, fmt.Errorf("failurelog: write state: %w", err)
	}
	return entry, nil
}

// extractSummary pulls the first ~8 lines of the Failure / Verdict /
// Phase Outcomes section from orchestrator-report.md, joined into one
// line, capped at 400 chars. Ports the bash awk extractor.
func extractSummary(reportPath string) string {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	const maxLines = 8
	const maxBytes = 400

	var out []string
	capturing := false
	captured := 0
	for _, line := range lines {
		if capturing {
			if len(out) > 0 && strings.HasPrefix(line, "## ") && captured > 0 {
				// New section start — stop capturing.
				break
			}
			out = append(out, line)
			captured++
			if captured >= maxLines {
				break
			}
			continue
		}
		// Look for the section markers.
		if strings.HasPrefix(line, "## Failure") ||
			strings.HasPrefix(line, "## Verdict") ||
			strings.HasPrefix(line, "## Phase Outcomes") {
			capturing = true
		}
	}
	if len(out) == 0 {
		return ""
	}
	joined := strings.Join(out, " ")
	joined = strings.Join(strings.Fields(joined), " ") // squeeze whitespace
	if len(joined) > maxBytes {
		joined = joined[:maxBytes]
	}
	return joined
}

// mustMarshalToAny round-trips entry through json.Marshal so the slot
// in state["failedApproaches"] is a plain map[string]any. Without
// this, the json.Encoder in atomicWriteJSON would serialize the typed
// Recorded struct alongside untyped legacy entries — fine, but a
// uniform shape simplifies test assertions.
func mustMarshalToAny(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		// json.Marshal of Recorded cannot fail in practice; defensive
		// branch falls through to empty map.
		return map[string]any{}
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// atomicWriteJSON serializes state to <path>.tmp.<pid> then renames
// over <path>. Mirrors the bash dispatcher's mv-of-tmp pattern (POSIX
// rename is atomic on the same filesystem). Returns an error if any
// step fails; leaves no partial file behind.
//
// Exposed via writeFn seam so tests can drive the write-error branch
// without contriving filesystem permissions.
var atomicWriteJSON = atomicWriteJSONReal

func atomicWriteJSONReal(path string, state map[string]any) error {
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
