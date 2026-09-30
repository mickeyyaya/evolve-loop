//go:build acs

package cycle504

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func runGoTest(t *testing.T, runFilter string, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-race", "-v"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

func TestC504_001_CatalogPolicySchemaRoundTrips(t *testing.T) {
	out, code := runGoTest(t,
		"TestCatalogConfig_AllowedFamilies|TestLoad_ParsesCatalogAllowedFamilies",
		policyPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("catalog.allowed_families schema is red (exit=%d) — CatalogPolicy.AllowedFamilies is missing, not resolved by CatalogConfig(), or not parsed by Load()\n%s", code, out)
	}
}

func TestC504_002_RefreshFiltersByFamilyBeforeClassify(t *testing.T) {
	out, code := runGoTest(t,
		"TestRefreshFamilyFilterAppliedBeforeClassify",
		modelqueryPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("family-filter production wiring is red (exit=%d) — RefreshDeps.AllowedFamilies is missing or not applied before Classify\n%s", code, out)
	}
}

func TestC504_003_RefreshNoFamiliesIsPassthroughRegression(t *testing.T) {
	out, code := runGoTest(t,
		"TestRefreshNoAllowedFamiliesIsPassthrough",
		modelqueryPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("no-constraint passthrough regression is red (exit=%d) — an unconfigured CLI's ids must reach Classify unfiltered\n%s", code, out)
	}
}
