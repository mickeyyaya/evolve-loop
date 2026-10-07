package loopwave

import (
	"path/filepath"
	"testing"
)

func TestRefill_RanksLikeTheSeedAndTheWiden(t *testing.T) {
	h := newHarness(t)
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "a-mention.json"), map[string]any{"id": "mention", "weight": 0.5, "files": []string{"the wave planner"}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "b-declared.json"), map[string]any{"id": "declared", "weight": 0.5, "files": []string{"pkg/declared.go"}})
	spec, ok := h.e.refill()(map[string]bool{})
	if !ok || spec.Scope[0] != "declared" {
		t.Errorf("equal scores break on a declared surface, as inboxrank.Order orders the seed and the widen: %+v %v", spec, ok)
	}
}
