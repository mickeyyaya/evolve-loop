package bridgechain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestBenchOnEscalation_AgyQuotaWallBenchesTheAgyFamily(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	now := func() time.Time { return time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC) }
	b, _ := json.Marshal(map[string]any{"captured_at": now(), "cli": "agy-tmux", "pattern_name": "quota_exhausted",
		"pane_tail": "You have exceeded your daily limit for the free tier"})
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: "agy-tmux", DispatchStart: now().Add(-time.Minute), Env: map[string]string{}}, now, nil)
	if _, ok := clihealth.NewStore(root, now).Active()["agy"]; !ok {
		t.Fatal("agy's quota_exhausted wall left the family unbenched; every later dispatch pays a dead round-trip")
	}
}
