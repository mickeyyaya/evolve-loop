package cyclecost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeCycle_SkipsUnusableUsageSidecars(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-21")
	writeUsageSidecar(t, ws, "build", 0.30, 500, 60, 7, 8)
	unusable := map[string]string{
		"broken-usage.json":   "{not json",
		"nameless-usage.json": `{"cost_usd":9.99}`,
	}
	for name, body := range unusable {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(ws, "unreadable-usage.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, err := SummarizeCycle(ws, 21)
	if err != nil {
		t.Fatalf("SummarizeCycle: %v", err)
	}

	if len(s.Phases) != 1 || s.Phases[0].Phase != "build" {
		t.Fatalf("only the valid sidecar may count, got %+v", s.Phases)
	}
	if diffAbs(s.Total.CostUSD, 0.30) > 1e-9 {
		t.Errorf("total cost=%v want 0.30 (unusable sidecars must add nothing)", s.Total.CostUSD)
	}
}
