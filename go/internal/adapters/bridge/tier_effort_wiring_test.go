package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductionDepsCarryTheRoutingTableEfforts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	body := `{"cli_routing":{"clis":["claude"],"tiers":{"deep":{"effort":"high"}},"agents":{"scout":{"effort":"low"}}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	deps := NewDefault(root, nil).productionEngineDeps(map[string]string{"HOME": root})
	if got, _ := deps.Efforts.Resolve("deep"); got != "high" {
		t.Errorf("deep effort = %q, want high: cli_routing.tiers.deep.effort never reaches the engine", got)
	}
	if got, _ := deps.Efforts.Resolve("deep", "scout"); got != "low" {
		t.Errorf("scout effort = %q, want low: cli_routing.agents.scout.effort never reaches the engine", got)
	}
}
