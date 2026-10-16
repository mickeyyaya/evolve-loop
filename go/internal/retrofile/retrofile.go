// Package retrofile lifts the preventive actions of a retro report into weighted inbox todos.
// See docs/architecture/packages/internal-retrofile.md.
package retrofile

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const preventiveHeading = "## Recommended preventive actions"

type PreventiveAction struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	WeightHint    float64  `json:"weight_hint"`
	Files         []string `json:"files"`
	Evidence      string   `json:"evidence"`
	Recurrence    int      `json:"recurrence"`
	PriorityClass string   `json:"priority_class"`
}

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
		if err := inboxbatch.RequirePriorityClass(a.PriorityClass); err != nil {
			return written, fmt.Errorf("retrofile: action %s: %w", a.ID, err)
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
			"priority_class":   a.PriorityClass,
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
		existing[a.ID] = true
	}
	return written, nil
}

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
