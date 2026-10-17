//go:build acs

package cycle1930

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func namedPackageBudget(t *testing.T) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(addedtests.PackageWallBudget)
	if err != nil {
		t.Fatalf("addedtests.PackageWallBudget %q is not a duration: %v", addedtests.PackageWallBudget, err)
	}
	return d
}

func buildTestlatency(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "testlatency")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/testlatency")
	cmd.Dir = goModuleDir(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build testlatency: %v\n%s", err, out)
	}
	return bin
}

func runTestlatency(t *testing.T, bin, stream string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(stream)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run testlatency: %v", err)
	}
	return stdout.String(), stderr.String(), code
}

func packageStream(pkg string, wallSeconds float64) string {
	return fmt.Sprintf("{\"Action\":\"run\",\"Package\":%q,\"Test\":\"TestSlow\"}\n"+
		"{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestSlow\",\"Elapsed\":%.2f}\n"+
		"{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":%.2f}\n", pkg, pkg, wallSeconds, pkg, wallSeconds)
}

func overBudgetSection(report string) string {
	start := strings.Index(report, "## Packages over budget")
	if start < 0 {
		return ""
	}
	rest := report[start:]
	if end := strings.Index(rest, "- Packages:"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func TestC1930_001_PackageWallBudgetIsOneNamedValueWellBelowPackageTimeout(t *testing.T) {
	budget := namedPackageBudget(t)
	timeout, err := time.ParseDuration(addedtests.PackageTimeout)
	if err != nil {
		t.Fatalf("addedtests.PackageTimeout %q: %v", addedtests.PackageTimeout, err)
	}
	if budget <= 0 {
		t.Fatalf("PackageWallBudget %v must be positive", budget)
	}
	if budget > timeout/4 {
		t.Errorf("PackageWallBudget %v is not well below PackageTimeout %v (want <= %v)", budget, timeout, timeout/4)
	}
}

func TestC1930_002_PackageBudgetFlagFailsAndNamesPackageWallAndBudget(t *testing.T) {
	budget := namedPackageBudget(t)
	bin := buildTestlatency(t)
	overWall := budget.Seconds() + 7.5
	stream := packageStream("example.com/m/slowpkg", overWall) + packageStream("example.com/m/fastpkg", 0.5)
	stdout, stderr, code := runTestlatency(t, bin, stream, "-package-budget")
	if code != 1 {
		t.Fatalf("exit %d, want 1 for a package over the named budget\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	wantBudget := fmt.Sprintf("> %.1fs wall", budget.Seconds())
	wantRow := fmt.Sprintf("| example.com/m/slowpkg | %.2f |", overWall)
	for _, want := range []string{wantBudget, wantRow} {
		if !strings.Contains(stdout, want) {
			t.Errorf("report misses %q (package, its wall time and the budget must all be named):\n%s", want, stdout)
		}
	}
	if strings.Contains(overBudgetSection(stdout), "fastpkg") {
		t.Errorf("a package under the budget is listed as over budget:\n%s", stdout)
	}
}

func TestC1930_003_PackageBudgetFlagPassesWhenEveryPackageIsWithinBudget(t *testing.T) {
	budget := namedPackageBudget(t)
	bin := buildTestlatency(t)
	stream := packageStream("example.com/m/atbudget", budget.Seconds()) + packageStream("example.com/m/fastpkg", 0.5)
	stdout, stderr, code := runTestlatency(t, bin, stream, "-package-budget")
	if code != 0 {
		t.Fatalf("exit %d, want 0 when no package exceeds the named budget\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "All packages are within the budget.") {
		t.Errorf("report lacks the within-budget line:\n%s", stdout)
	}
}

func TestC1930_004_ExistingBudgetFlagsKeepTheirContract(t *testing.T) {
	bin := buildTestlatency(t)
	if _, stderr, code := runTestlatency(t, bin, packageStream("x/p", 1), "-fail-over-budget"); code != 2 || !strings.Contains(stderr, "--fail-over-budget needs --budget") {
		t.Errorf("-fail-over-budget without --budget: exit %d stderr %q, want exit 2", code, stderr)
	}
	if _, _, code := runTestlatency(t, bin, packageStream("x/p", 500), "-budget", "180s"); code != 0 {
		t.Errorf("-budget alone is advisory: exit %d, want 0", code)
	}
}

func TestC1930_005_MakeTestBudgetTargetPipesGoTestJSONIntoThePackageBudgetCheck(t *testing.T) {
	cmd := exec.Command("make", "-n", "test-budget")
	cmd.Dir = goModuleDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n test-budget: %v\n%s", err, out)
	}
	plan := string(out)
	for _, want := range []string{"test", "-json", "testlatency", "-package-budget"} {
		if !strings.Contains(plan, want) {
			t.Errorf("make test-budget plan lacks %q:\n%s", want, plan)
		}
	}
}
