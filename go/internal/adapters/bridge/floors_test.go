package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefault_TheEngineCarriesTheProjectFloors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_routing":{"clis":["agy"],"default":["agy"],"floors":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	floors := NewDefault(root, nil).productionEngineDeps(nil).Floors
	if floors == nil || len(floors) != 0 {
		t.Fatalf("Deps.Floors = %v, want the declared empty floors (nil reads as the compiled default)", floors)
	}
}
