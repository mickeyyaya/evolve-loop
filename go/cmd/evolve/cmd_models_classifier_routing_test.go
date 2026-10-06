package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func classifierRouter(t *testing.T, pol policy.Policy) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{Policy: pol, Profiles: profiles.NewFromDir(t.TempDir()),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func legacyClassifierPreference(t *testing.T) []string {
	t.Helper()
	families, err := classifierFamilies(classifierRouter(t, policy.Policy{}))
	if err != nil {
		t.Fatalf("classifierFamilies: %v", err)
	}
	return families
}

func TestClassifierOrder_TheLegacyProjectionKeepsTodaysOrder(t *testing.T) {
	if got := legacyClassifierPreference(t); !reflect.DeepEqual(got, []string{"codex", "claude", "agy"}) {
		t.Fatalf("with no cli_routing block the classifier order stays codex > claude > agy: %v", got)
	}
}

func TestClassifierOrder_ComesFromTheRoutingTable(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	families, err := classifierFamilies(classifierRouter(t, policy.Policy{CLIRouting: &block}))
	if err != nil {
		t.Fatalf("classifierFamilies: %v", err)
	}
	if got := pickClassifierCLI([]string{"codex", "claude", "agy"}, families, ""); !reflect.DeepEqual(got, []string{"agy", "claude"}) {
		t.Fatalf("a table with clis [agy, claude] never launches codex to classify: %v", got)
	}
}

func TestClassifierPreference_ARefusedTableFailsTheRefresh(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"cli_routing":{"clis":["agy"],"default":["agy"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := classifierPreference(root); err == nil {
		t.Fatal("a table without claude cannot classify; the refresh fails loudly instead of falling back to the old order")
	}
}
