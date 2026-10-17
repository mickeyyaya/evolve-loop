//go:build acs

package cycle1940

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	baseSource        = "package p\n\nfunc F(x int) int {\n\treturn x\n}\n"
	branchingSource   = "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n"
	fullCoverageTests = "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(1) != 1 || F(-1) != -1 {\n\t\tt.Fatal(\"F\")\n\t}\n}\n"
	partialTest       = "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(-1) != -1 {\n\t\tt.Fatal(\"F\")\n\t}\n}\n"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectRoot(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	if policyJSON != "" {
		write(t, filepath.Join(root, ".evolve", "policy.json"), policyJSON)
	}
	return root
}

func cycleDiff(t *testing.T, changed, testBody string) (repoDir, base string) {
	t.Helper()
	repo := gittest.Fixture(t)
	write(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	write(t, filepath.Join(repo.Dir, "go", "p", "a.go"), baseSource)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base = repo.Git("rev-parse", "HEAD")
	if changed != "" {
		write(t, filepath.Join(repo.Dir, "go", "p", "a.go"), changed)
	}
	if testBody != "" {
		write(t, filepath.Join(repo.Dir, "go", "p", "a_test.go"), testBody)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "--allow-empty", "-m", "build")
	return repo.Dir, base
}

func floor(t *testing.T, root, worktree, base string) []string {
	t.Helper()
	return core.DefaultBuildFloorChecks(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseBuild), ProjectRoot: root, Worktree: worktree, WorktreeBaseSHA: base,
	})
}

func coverFailures(failures []string) []string {
	var out []string
	for _, f := range failures {
		if strings.Contains(f, "100% statement coverage") {
			out = append(out, f)
		}
	}
	return out
}

func TestC1940_001_CoverChangedCompiledDefaultIsEnforce(t *testing.T) {
	for name, tc := range map[string]struct {
		policyJSON string
		want       string
	}{
		"no policy file":        {"", "enforce"},
		"empty policy":          {`{}`, "enforce"},
		"empty cover_changed":   {`{"cover_changed":{}}`, "enforce"},
		"explicit shadow kept":  {`{"cover_changed":{"stage":"shadow"}}`, "shadow"},
		"explicit off kept":     {`{"cover_changed":{"stage":"off"}}`, "off"},
		"explicit enforce kept": {`{"cover_changed":{"stage":"enforce"}}`, "enforce"},
	} {
		p := policy.Policy{}
		if tc.policyJSON != "" {
			path := filepath.Join(t.TempDir(), "policy.json")
			write(t, path, tc.policyJSON)
			loaded, err := policy.Load(path)
			if err != nil {
				t.Fatalf("%s: load: %v", name, err)
			}
			p = loaded
		}
		if got := p.CoverChangedConfig().Stage; got != tc.want {
			t.Errorf("%s: stage = %q, want %q", name, got, tc.want)
		}
	}
}

func TestC1940_002_BuildFloorRefusesUncoveredChangedFunctionByName(t *testing.T) {
	repoDir, base := cycleDiff(t, branchingSource, partialTest)
	failures := coverFailures(floor(t, projectRoot(t, ""), repoDir, base))
	if len(failures) != 1 {
		t.Fatalf("with no cover_changed policy the production floor must refuse the uncovered branch once, got %v", failures)
	}
	if !strings.Contains(failures[0], "p.F") || !strings.Contains(failures[0], "p/a.go") {
		t.Errorf("refusal must name the function and file, got %q", failures[0])
	}
}

func TestC1940_003_FullyCoveredChangeIsNotRefused(t *testing.T) {
	repoDir, base := cycleDiff(t, branchingSource, fullCoverageTests)
	if failures := coverFailures(floor(t, projectRoot(t, ""), repoDir, base)); len(failures) != 0 {
		t.Errorf("a fully covered change must pass, got %v", failures)
	}
}

func TestC1940_004_EmptyDiffIsNotRefused(t *testing.T) {
	repoDir, base := cycleDiff(t, "", "")
	if failures := coverFailures(floor(t, projectRoot(t, ""), repoDir, base)); len(failures) != 0 {
		t.Errorf("an empty diff must pass, got %v", failures)
	}
}

func TestC1940_005_ExplicitShadowAndOffDoNotRefuseAnUncoveredChange(t *testing.T) {
	for _, word := range []string{"shadow", "off"} {
		repoDir, base := cycleDiff(t, branchingSource, partialTest)
		root := projectRoot(t, `{"cover_changed":{"stage":"`+word+`"}}`)
		if failures := coverFailures(floor(t, root, repoDir, base)); len(failures) != 0 {
			t.Errorf("stage %s must not refuse, got %v", word, failures)
		}
	}
}

func TestC1940_006_FunctionOnlyTheBaseChangedIsNotRefused(t *testing.T) {
	repo := gittest.Fixture(t)
	write(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	write(t, filepath.Join(repo.Dir, "go", "p", "a.go"), branchingSource)
	write(t, filepath.Join(repo.Dir, "go", "q", "b.go"), "package q\n\nfunc G() int {\n\treturn 1\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	write(t, filepath.Join(repo.Dir, "go", "q", "b.go"), "package q\n\nfunc G() int {\n\treturn 2\n}\n")
	write(t, filepath.Join(repo.Dir, "go", "q", "b_test.go"), "package q\n\nimport \"testing\"\n\nfunc TestG(t *testing.T) {\n\tif G() != 2 {\n\t\tt.Fatal(\"G\")\n\t}\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build")
	failures := coverFailures(floor(t, projectRoot(t, ""), repo.Dir, base))
	if len(failures) != 0 {
		t.Errorf("an untouched uncovered function in another package must not be refused, got %v", failures)
	}
}

const (
	failingTest                = "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(1) != 99 {\n\t\tt.Fatal(\"F(1) != 99\")\n\t}\n}\n"
	failingOnlyIntegrationTest = "//go:build integration\n\npackage p\n\nimport \"testing\"\n\nfunc TestFIntegration(t *testing.T) {\n\tif F(1) != 99 {\n\t\tt.Fatal(\"F(1) != 99\")\n\t}\n}\n"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func goTestInCore(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "go", append([]string{"test", "-count=1"}, args...)...)
	cmd.Dir = moduleDir(t)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC1940_007_CommittedFailingPackageIsRefusedOnceUnderTheEnforceDefault(t *testing.T) {
	repoDir, base := cycleDiff(t, branchingSource, failingTest)
	failures := floor(t, projectRoot(t, ""), repoDir, base)
	if len(failures) != 1 {
		t.Fatalf("under the enforce default a changed package whose tests fail must be refused exactly once (cover-changed must not repeat the unit-test refusal), got %d: %v", len(failures), failures)
	}
	if !strings.Contains(failures[0], "p") || !strings.Contains(failures[0], "F(1) != 99") {
		t.Errorf("the one refusal must name the package and carry the test failure, got %q", failures[0])
	}
}

func TestC1940_008_PackageFailingOnlyUnderIntegrationTagIsStillRefused(t *testing.T) {
	repoDir, base := cycleDiff(t, branchingSource, "")
	write(t, filepath.Join(repoDir, "go", "p", "a_test.go"), fullCoverageTests)
	write(t, filepath.Join(repoDir, "go", "p", "integration_test.go"), failingOnlyIntegrationTest)
	failures := floor(t, projectRoot(t, ""), repoDir, base)
	if len(failures) == 0 {
		t.Fatalf("a changed package whose tests fail only under -tags integration must still be refused by the floor")
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, "F(1) != 99") {
		t.Errorf("the refusal must carry the integration-tagged test failure, got %v", failures)
	}
}

func TestC1940_009_ExistingCommittedBuilderWorkFloorTestPassesUnderTheIntegrationTag(t *testing.T) {
	out, err := goTestInCore(t, nil, "-tags", "integration", "-run", "^TestDefaultBuildFloorChecks_SeesCommittedBuilderWork$", "./internal/core")
	if err != nil {
		t.Fatalf("the audit's integration-tier offender must be green: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ok ") {
		t.Errorf("the named test must run and pass, got:\n%s", out)
	}
}

func TestC1940_010_FloorArgTestsAreGreenInsideAFleetLaneEnv(t *testing.T) {
	laneEnv := []string{"EVOLVE_FLEET=1", "EVOLVE_FLEET_WIDTH=1"}
	out, err := goTestInCore(t, laneEnv, "-v", "-run", "^(TestCoverTestArgs_|TestUnitTestArgs_|TestTaggedTestArgs_)", "./internal/core")
	if err != nil {
		t.Fatalf("the floor arg tests must not depend on the lane's fleet env (the integration tier runs inside it): %v\n%s", err, out)
	}
	if strings.Count(out, "--- PASS: Test") < 4 {
		t.Errorf("all four floor arg tests must run and pass, got:\n%s", out)
	}
}
