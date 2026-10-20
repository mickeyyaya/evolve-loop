package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefault_WiresTheModelChainOnlyWhenThePolicyTurnsItOn(t *testing.T) {
	for policyJSON, want := range map[string]bool{`{"bridge":{"model_failover":"on"}}`: true, `{"bridge":{}}`: false, "": false} {
		root := t.TempDir()
		if policyJSON != "" {
			if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := NewDefault(root, nil).productionEngineDeps(nil).ModelChain != nil; got != want {
			t.Errorf("policy %q: model chain wired = %v, want %v", policyJSON, got, want)
		}
	}
}
