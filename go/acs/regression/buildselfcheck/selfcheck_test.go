//go:build acs

package buildselfcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const artifactRepoPath = ".evolve/build-selfcheck.json"

type pkgFailure struct {
	Pkg    string `json:"pkg"`
	Output string `json:"output"`
}

func parseSelfCheckFailures(data []byte) []pkgFailure {
	var fails []pkgFailure
	if err := json.Unmarshal(data, &fails); err != nil {
		return nil
	}
	return fails
}

func TestParseSelfCheckFailures(t *testing.T) {
	if got := parseSelfCheckFailures([]byte(`[]`)); len(got) != 0 {
		t.Errorf("empty array → %d failures, want 0", len(got))
	}
	two := `[{"pkg":"./cmd/evolve","output":"vet: string(Stage)"},{"pkg":"./internal/flagregistry","output":"--- FAIL"}]`
	got := parseSelfCheckFailures([]byte(two))
	if len(got) != 2 {
		t.Fatalf("two entries → %d failures, want 2", len(got))
	}
	if got[0].Pkg != "./cmd/evolve" {
		t.Errorf("first failure pkg = %q, want ./cmd/evolve", got[0].Pkg)
	}
	if got := parseSelfCheckFailures([]byte(`not json`)); got != nil {
		t.Errorf("malformed → %v, want nil (fail-open)", got)
	}
}

func TestChangedPackagesToolchainGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, artifactRepoPath))
	if err != nil {
		t.Skipf("no build-selfcheck artifact (%v); toolchain gate inert", err)
	}
	fails := parseSelfCheckFailures(data)
	if len(fails) == 0 {
		return
	}
	names := make([]string, len(fails))
	for i, f := range fails {
		names[i] = f.Pkg
	}
	t.Fatalf("toolchain gate: %d changed package(s) FAIL build/vet/test — a cycle cannot ship code that does not compile/vet/test green: %s\n"+
		"first failure output:\n%s",
		len(fails), strings.Join(names, ", "), truncate(fails[0].Output, 1200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}
