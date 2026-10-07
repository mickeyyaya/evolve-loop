package loopwave

import (
	"path/filepath"
	"testing"
)

func rankedInbox(t *testing.T, evolveDir string) {
	t.Helper()
	for _, it := range []map[string]any{
		{"id": "heavy-security", "weight": 0.6, "priority_class": "security", "files": []string{"pkg/a/a.go"}},
		{"id": "mid-stability", "weight": 0.57, "priority_class": "stability", "files": []string{"pkg/b/b.go"}},
		{"id": "light-correctness", "weight": 0.55, "priority_class": "correctness", "files": []string{"pkg/c/c.go"}},
	} {
		writeJSON(t, filepath.Join(evolveDir, "inbox", it["id"].(string)+".json"), it)
	}
}

func TestSeedWavePlanFromInbox_SeedsTheFirstRankedLanesNotTheHeaviest(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	rankedInbox(t, evolveDir)
	data, err := SeedWavePlanFromInbox(evolveDir, 2, nil)
	want := `{"top_n":[{"files":["pkg/c/c.go"],"id":"light-correctness"},{"files":["pkg/b/b.go"],"id":"mid-stability"}]}`
	if err != nil || string(data) != want {
		t.Errorf("seed = %s %v, want the rank's first two lanes %s", data, err, want)
	}
}

func TestWidenNarrowDecision_BackfillsTheFirstRankedLane(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	rankedInbox(t, evolveDir)
	narrow := []byte(`{"top_n":[{"id":"kept","files":["pkg/k/k.go"]}]}`)
	want := `{"top_n":[{"files":["pkg/k/k.go"],"id":"kept"},{"files":["pkg/c/c.go"],"id":"light-correctness"}]}`
	if got := string(WidenNarrowDecision(narrow, evolveDir, 2, nil)); got != want {
		t.Errorf("widen = %s, want %s", got, want)
	}
}

func TestRefill_TakesTheFirstRankedItemNotTheHeaviest(t *testing.T) {
	h := newHarness(t)
	rankedInbox(t, h.evolveDir)
	spec, ok := h.e.refill()(map[string]bool{"light-correctness": true})
	if !ok || spec.Scope[0] != "mid-stability" {
		t.Errorf("refill = %+v %v, want the first-ranked item not excluded", spec, ok)
	}
}
