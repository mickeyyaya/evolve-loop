package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
)

const agyOnlyPolicy = `{"cli_routing":{"clis":["agy"],"default":["agy"],"floors":{}}}`

func agyOnlyTrackedRoot(t *testing.T) string {
	t.Helper()
	src := checkedInRoot(t)
	root := t.TempDir()
	copyTreeForTest(t, filepath.Join(src, ".evolve", "profiles"), filepath.Join(root, ".evolve", "profiles"))
	copyFileForTest(t, filepath.Join(src, "docs", "architecture", "phase-registry.json"), filepath.Join(root, "docs", "architecture", "phase-registry.json"))
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(agyOnlyPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCLIRouting_CheckOnAnAgyOnlyTableReportsOnlyTheRouter(t *testing.T) {
	rc, out, _ := runRoutingVerb(t, "check", "--json", "--project-root", agyOnlyTrackedRoot(t))
	var result routingCheckResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("check --json output: %v: %q", err, out)
	}
	var errorKeys []string
	for _, f := range result.Findings {
		if f.Severity == cliroute.SeverityError {
			errorKeys = append(errorKeys, f.Key)
		}
	}
	if want := []string{"agent.router"}; !slices.Equal(errorKeys, want) {
		t.Fatalf("error findings = %v, want %v (rc=%d)", errorKeys, want, rc)
	}
}
