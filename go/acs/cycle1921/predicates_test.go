//go:build acs

package cycle1921

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const priorSuiteRel = "go/acs/cycle1915/predicates_test.go"

func priorSuiteSource(vertexInTarget, vertexInLaterFunc bool) string {
	target, later := `"WebM",`, `"WebM": true,`
	if vertexInTarget {
		target = `"WebM", "Vertex AI",`
	}
	if vertexInLaterFunc {
		later = `"Vertex AI": true,`
	}
	return "package cycle1915\n\n" +
		"func TestC1915_002_DocExplainsV2_24_0AsData(t *testing.T) {\n\t_ = []string{" + target + "}\n}\n\n" +
		"func TestC1915_004_NewestSectionCitesOnlySnapshotIdentifiers(t *testing.T) {\n\t_ = map[string]bool{" + later + "}\n}\n"
}

func buildCycle1920Binary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "cycle1920.test")
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := exec.Command("go", "-C", goDir, "test", "-tags", "acs", "-c", "-o", bin, "./acs/cycle1920").CombinedOutput()
	if err != nil {
		t.Fatalf("RED: cannot build cycle1920 test binary: %v\n%s", err, out)
	}
	return bin
}

func runTestC1920_005Against(t *testing.T, bin, suiteSource string) (string, bool) {
	t.Helper()
	repo := gittest.Fixture(t)
	dest := filepath.Join(repo.Dir, priorSuiteRel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(suiteSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-test.run", "^TestC1920_005_", "-test.v")
	cmd.Dir = repo.Dir
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func TestC1921_001_RejectsPriorSuiteWhoseTargetLacksVertexAIButLaterFuncHasIt(t *testing.T) {
	bin := buildCycle1920Binary(t)
	out, passed := runTestC1920_005Against(t, bin, priorSuiteSource(false, true))
	if passed {
		t.Errorf("RED: TestC1920_005 passed although TestC1915_002 lacks %q and only a later function has it:\n%s", "Vertex AI", out)
	}
	if !strings.Contains(out, "TestC1915_002 item list does not require") {
		t.Errorf("RED: TestC1920_005 did not fail for the expected reason:\n%s", out)
	}
}

func TestC1921_002_AcceptsPriorSuiteWhoseTargetRequiresVertexAI(t *testing.T) {
	bin := buildCycle1920Binary(t)
	out, passed := runTestC1920_005Against(t, bin, priorSuiteSource(true, false))
	if !passed {
		t.Errorf("RED: TestC1920_005 failed although TestC1915_002 requires %q:\n%s", "Vertex AI", out)
	}
}

func TestC1921_003_RejectsPriorSuiteWithNoVertexAIAnywhere(t *testing.T) {
	bin := buildCycle1920Binary(t)
	out, passed := runTestC1920_005Against(t, bin, priorSuiteSource(false, false))
	if passed {
		t.Errorf("RED: TestC1920_005 passed on a suite with no %q at all:\n%s", "Vertex AI", out)
	}
}

func TestC1921_004_AcceptsWhenTargetIsLastFunctionInFile(t *testing.T) {
	bin := buildCycle1920Binary(t)
	src := "package cycle1915\n\nfunc TestC1915_002_DocExplainsV2_24_0AsData(t *testing.T) {\n\t_ = []string{\"Vertex AI\"}\n}\n"
	out, passed := runTestC1920_005Against(t, bin, src)
	if !passed {
		t.Errorf("RED: TestC1920_005 failed when TestC1915_002 is the last function:\n%s", out)
	}
}

func TestC1921_005_RealPriorSuiteStillPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-tags", "acs", "-count=1", "./acs/cycle1920")
	if err != nil || code != 0 {
		t.Fatalf("RED: cycle1920 suite fails (exit %d): %s", code, out)
	}
}
