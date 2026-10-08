package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecAdapterDeps_CarriesThePolicyTierEffort(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"bridge":{"tier_effort":{"deep":"medium"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := execAdapterDeps(map[string]string{"HOME": t.TempDir(), "EVOLVE_PROJECT_ROOT": root})
	if d.TierEffort["deep"] != "medium" || d.TierEffort["balanced"] != "medium" {
		t.Errorf("execAdapterDeps TierEffort = %v, want deep=medium from policy and balanced=medium compiled", d.TierEffort)
	}
}
