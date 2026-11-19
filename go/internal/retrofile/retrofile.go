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
	Summary       string   `json:"summary,omitempty"`
	Acceptance    []string `json:"acceptance,omitempty"`
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
	existing, err := existingItems(inboxDir)
	if err != nil {
		return nil, err
	}
	for _, a := range actions {
		if a.ID == "" {
			continue
		}
		prev, exists := existing[a.ID]
		if exists && (!prev.open || a.Recurrence <= prev.recurrence) {
			continue
		}
		if err := inboxbatch.RequirePriorityClass(a.PriorityClass); err != nil {
			return written, fmt.Errorf("retrofile: action %s: %w", a.ID, err)
		}
		path := prev.path
		if !exists {
			path = filepath.Join(inboxDir, fmt.Sprintf("auto-retro-%d-%s.json", cycle, a.ID))
		}
		item := a.inboxItem(defaultWeight, now)
		body, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return written, fmt.Errorf("retrofile: encode item %s: %w", a.ID, err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return written, fmt.Errorf("retrofile: write item %s: %w", a.ID, err)
		}
		written = append(written, path)
		existing[a.ID] = existingItem{path: path, open: true, recurrence: a.Recurrence}
	}
	return written, nil
}

func (a PreventiveAction) inboxItem(defaultWeight float64, now time.Time) map[string]any {
	weight := defaultWeight
	if a.WeightHint > 0 {
		weight = a.WeightHint
	}
	item := map[string]any{
		"id":               a.ID,
		"title":            a.Title,
		"action":           a.Title,
		"priority":         "HIGH",
		"weight":           weight,
		"files":            a.Files,
		"evidence_pointer": a.Evidence,
		"recurrence":       a.Recurrence,
		"priority_class":   a.PriorityClass,
		"origin":           inboxbatch.OriginRetro,
		"injected_at":      now.UTC().Format(time.RFC3339),
		"injected_by":      "retro-preventive-actions-autofiler",
	}
	if a.Summary != "" {
		item["summary"] = a.Summary
	}
	if len(a.Acceptance) > 0 {
		item["acceptance"] = a.Acceptance
	}
	if !(inboxbatch.Item{Files: a.Files}).HasDeclaredSurface() {
		item["kind"] = inboxbatch.KindPipelineRepair
	}
	return item
}

type existingItem struct {
	path       string
	open       bool
	recurrence int
}

func existingItems(inboxDir string) (map[string]existingItem, error) {
	items := map[string]existingItem{}
	root := filepath.Clean(inboxDir)
	err := filepath.WalkDir(inboxDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return walkErr
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
			rec, _ := m["recurrence"].(float64)
			open := filepath.Dir(path) == root
			if prev, exists := items[id]; !exists || (!prev.open && open) {
				items[id] = existingItem{path: path, open: open, recurrence: int(rec)}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
