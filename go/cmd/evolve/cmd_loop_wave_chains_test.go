package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestQuotaAwareWaveConfig_ABenchWithAFallbackInTheRoutingTableKeepsTheLane(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	table := `{"cli_routing":{"clis":["agy","agy-claude","claude"],"default":["agy","agy-claude","claude"],"tiers":{"deep":{"clis":["agy-claude","claude"]},"top":{"clis":["agy-claude","claude"]}}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := clihealth.NewStore(root, nil).Bench(clihealth.Entry{Family: "agy-claude", Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder

	got, _ := quotaAwareWaveConfig(policy.FleetConfig{Count: 4, MinLanes: 2}, root, &warn, nil, budgethistory.Throughput{}, now)

	if got.Count != 4 {
		t.Fatalf("count = %d, want 4: the production engine reads the routing chains and claude follows agy-claude; log %q", got.Count, warn.String())
	}
	if !strings.Contains(warn.String(), `"agy-claude" (rate_limit) costs no lane: a lane walks to "agy"`) {
		t.Errorf("log = %q, want the INFO line that names the fallback", warn.String())
	}
}
