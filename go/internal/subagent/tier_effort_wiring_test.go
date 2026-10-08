package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecAdapterDeps_CarriesTheRoutingTableEfforts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"cli_routing":{"clis":["claude"],"tiers":{"deep":{"effort":"high"}}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d := execAdapterDeps(map[string]string{"HOME": t.TempDir(), "EVOLVE_PROJECT_ROOT": root})
	if got, _ := d.Efforts.Resolve("deep"); got != "high" {
		t.Errorf("execAdapterDeps deep effort = %q, want high from cli_routing.tiers", got)
	}
	if got, _ := d.Efforts.Resolve("balanced"); got != "medium" {
		t.Errorf("execAdapterDeps balanced effort = %q, want the compiled medium", got)
	}
}
