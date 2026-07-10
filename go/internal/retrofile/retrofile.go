// Package retrofile closes the learning→action loop: it lifts a retro
// report's machine-readable "preventive_actions" block into weighted
// .evolve/inbox todos, so a diagnosed recurrence becomes queued work instead
// of a lesson that is written and then forgotten.
//
// It is a stdlib-only leaf package deliberately kept free of loop/policy
// imports so it can be wired at the same deterministic post-cycle seam the
// FAILED_UNEXPLAINED classifier already uses
// (cmd/evolve/cmd_loop_outcome.go:fileUnexplainedOutcomeDefect). The caller
// supplies the default weight (resolved from policy, never a Go literal) and
// the clock, keeping this package pure and its output byte-stable.
package retrofile

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// preventiveHeading is the retro-report section under which the autofiler
// looks for the machine-readable action block. The retro agent doc
// (agents/evolve-retrospective.md) documents the matching FORMAT.
const preventiveHeading = "## Recommended preventive actions"

// PreventiveAction is one structured recommendation lifted from a retro
// report. ID is the stable inbox slug (reused across cycles so a recurrence
// deduplicates rather than spamming). A positive WeightHint overrides the
// caller's default weight — the recurrence-escalation lever. Recurrence is the
// count the retro attached (advisory; surfaced in the filed item's evidence).
type PreventiveAction struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	WeightHint float64  `json:"weight_hint"`
	Files      []string `json:"files"`
	Evidence   string   `json:"evidence"`
	Recurrence int      `json:"recurrence"`
}

// ParsePreventiveActions lifts the JSON array from the fenced ```json block
// under the "## Recommended preventive actions" heading of a retro report.
// A report with no such block yields (nil, nil) — the common
// no-recommendations case must no-op, not error. A present-but-malformed
// block is a real error the caller should surface.
func ParsePreventiveActions(report []byte) ([]PreventiveAction, error) {
	text := string(report)
	hIdx := strings.Index(text, preventiveHeading)
	if hIdx < 0 {
		return nil, nil
	}
	rest := text[hIdx+len(preventiveHeading):]
	fence := strings.Index(rest, "```json")
	if fence < 0 {
		return nil, nil
	}
	body := rest[fence+len("```json"):]
	end := strings.Index(body, "```")
	if end < 0 {
		return nil, fmt.Errorf("retrofile: unterminated ```json preventive_actions block")
	}
	raw := strings.TrimSpace(body[:end])
	var actions []PreventiveAction
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return nil, fmt.Errorf("retrofile: parse preventive_actions block: %w", err)
	}
	return actions, nil
}

// FileActions writes one auto-retro-<cycle>-<slug>.json inbox item per action
// into inboxDir, at the action's WeightHint when positive else defaultWeight.
// An action whose ID already exists as an inbox item — open (anywhere under
// inboxDir) or already consumed (under inboxDir/processed/**) — is skipped, so
// a recurrence does not re-file a fix that is still queued or already done.
// It returns the paths actually written. now stamps injected_at, keeping the
// output deterministic for the caller.
func FileActions(inboxDir string, cycle int, actions []PreventiveAction, defaultWeight float64, now time.Time) (written []string, err error) {
	if len(actions) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		return nil, fmt.Errorf("retrofile: create inbox dir: %w", err)
	}
	existing, err := existingIDs(inboxDir)
	if err != nil {
		return nil, err
	}
	for _, a := range actions {
		if a.ID == "" || existing[a.ID] {
			continue
		}
		weight := defaultWeight
		if a.WeightHint > 0 {
			weight = a.WeightHint
		}
		path := filepath.Join(inboxDir, fmt.Sprintf("auto-retro-%d-%s.json", cycle, a.ID))
		item := map[string]any{
			"id":               a.ID,
			"action":           a.Title,
			"priority":         "HIGH",
			"weight":           weight,
			"files":            a.Files,
			"evidence_pointer": a.Evidence,
			"recurrence":       a.Recurrence,
			"injected_at":      now.UTC().Format(time.RFC3339),
			"injected_by":      "retro-preventive-actions-autofiler",
		}
		body, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return written, fmt.Errorf("retrofile: encode item %s: %w", a.ID, err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return written, fmt.Errorf("retrofile: write item %s: %w", a.ID, err)
		}
		written = append(written, path)
		// Guard against duplicate IDs within a single batch.
		existing[a.ID] = true
	}
	return written, nil
}

// existingIDs returns the set of "id" values carried by every *.json item
// anywhere under inboxDir (open items at the top level and consumed items
// under processed/**), so an action already queued or already shipped is
// deduplicated regardless of where its item currently lives.
func existingIDs(inboxDir string) (map[string]bool, error) {
	ids := map[string]bool{}
	err := filepath.WalkDir(inboxDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("retrofile: read existing item %s: %w", path, err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			// A non-item JSON file (or garbage) must not abort dedup.
			return nil
		}
		if id, ok := m["id"].(string); ok && id != "" {
			ids[id] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}
