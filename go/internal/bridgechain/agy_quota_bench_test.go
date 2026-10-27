package bridgechain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBenchOnEscalation_AgyQuotaWallBenchesTheAgyFamily(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	now := func() time.Time { return time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC) }
	b, _ := json.Marshal(map[string]any{"captured_at": now(), "cli": "agy-tmux", "pattern_name": "quota_exhausted",
		"pane_tail": "You have exceeded your daily limit for the free tier"})
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: "agy-tmux", DispatchStart: now().Add(-time.Minute)}, now, nil)
	if _, ok := clihealth.NewStore(root, now).Active()["agy"]; !ok {
		t.Fatal("agy's quota_exhausted wall left the family unbenched; every later dispatch pays a dead round-trip")
	}
}

func TestBenchOnEscalation_AgyWallScopeFollowsTheDispatchedTier(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 17, 2, 17, 0, 0, time.UTC) }
	cases := []struct {
		cli, tier, wantKey string
	}{
		{"agy-tmux", "deep", "agy@Gemini 4 Argon"},
		{"agy-tmux", "top", "agy@Gemini 4 Argon"},
		{"agy-tmux", "balanced", "agy@Gemini 3.8 Flash"},
		{"agy-tmux", "", "agy"},
		{"agy-tmux", "sonnet", "agy"},
		{"claude-tmux", "deep", "claude"},
		{"agy-claude-tmux", "deep", "agy-claude"},
	}
	for _, tc := range cases {
		root, ws := t.TempDir(), t.TempDir()
		b, _ := json.Marshal(map[string]any{"captured_at": now(), "cli": tc.cli, "pattern_name": "quota_exhausted",
			"pane_tail": "Individual quota reached for this model. Resets in 3h."})
		if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: tc.cli, Tier: tc.tier, DispatchStart: now().Add(-time.Minute)}, now, nil)
		active := clihealth.NewStore(root, now).Active()
		if _, ok := active[tc.wantKey]; !ok || len(active) != 1 {
			t.Errorf("%s wall at tier %q: benches = %v, want the single key %q", tc.cli, tc.tier, active, tc.wantKey)
		}
	}
}

func TestEscalationFor_TheTierOfTheTryScopesTheAgyWallToItsModelGroup(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	now := func() time.Time { return time.Date(2026, 10, 19, 1, 3, 0, 0, time.UTC) }
	b, _ := json.Marshal(map[string]any{"captured_at": now(), "cli": "agy-tmux", "pattern_name": "exhausted",
		"pane_tail": "Individual quota reached for this model."})
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	req := core.BridgeRequest{CLI: "agy-tmux", Model: "deep", ProjectRoot: root, Workspace: ws}

	bridgechain.BenchOnEscalation(bridgechain.EscalationFor(req, now().Add(-time.Minute)), now, nil)

	active := clihealth.NewStore(root, now).Active()
	if _, ok := active["agy@Gemini 4 Argon"]; !ok || len(active) != 1 {
		t.Fatalf("benches %v; an escalation made from a deep agy try must bench only the Argon model group (cycle 1951)", active)
	}
}
