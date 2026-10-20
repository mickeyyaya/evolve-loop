//go:build acs

package cycle1568

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const retroPkg = "./internal/phases/retro"

func runRetroTest(t *testing.T, name string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", retroPkg, "-run", "^"+name+"$", "-count=1", "-v")
	return stdout + stderr, code
}

func assertNamedPass(t *testing.T, name string) {
	t.Helper()
	out, code := runRetroTest(t, name)
	if !strings.Contains(out, "=== RUN   "+name) {
		t.Fatalf("%s never ran (deleted, renamed, or filtered out)\n%s", name, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) || code != 0 {
		t.Errorf("%s did not PASS (exit=%d)\n%s", name, code, out)
	}
}

func TestC1568_005_explicit_model_tier_unchanged(t *testing.T) {
	assertNamedPass(t, "TestRun_ExplicitModel_PassesThroughUnchanged")
}
