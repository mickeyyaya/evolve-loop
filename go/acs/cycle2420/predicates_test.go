//go:build acs

package cycle2420

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	modulePath        = "github.com/mickeyyaya/evolve-loop/go"
	shipPackage       = "./internal/phases/ship"
	tiaPackage        = "./internal/regressiontia"
	scorePackage      = "./internal/stagescore"
	purityTestFile    = "internal/phases/ship/scanshadow_purity_test.go"
	pureTestFile      = "internal/regressiontia/pure_test.go"
	scoreTestFile     = "internal/stagescore/stagescore_test.go"
	enrollmentLine    = scorePackage
	enrollmentListing = "go/.apicover-enforce"
)

var (
	pureTests    = []string{"TestPure_ProvesAPackageWhoseClosureReachesNoEscapeHatch", "TestPure_RefusesEveryEscapeHatchDirectAndTransitive", "TestPure_FailsClosedWhenItCannotProve"}
	defaultDeny  = []string{"TestCacheable_RefusesPackagesThatReadTheEnvironmentOrGit", "TestCacheable_DefaultDenyForUnlistedImpurePackage"}
	handOverride = []string{"TestCacheable_AHandListedOverrideStillDeniesAPurePackage"}
	ambientEnv   = []string{"TestScanShadow_AmbientEnvChangeDoesNotHit"}
	scorerTests  = []string{"TestScorerCountsFalseStopEscapeConflict", "TestScorerCountsEachKindIndependently", "TestScorerRefusesADecisionItDoesNotKnow", "TestScorerOfNoRecordsIsTheZeroTally"}
)

func goTest(t *testing.T, goDir, pkg string, names []string) (string, int) {
	t.Helper()
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-timeout", "300s", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	return out + errOut, code
}

func requireGreen(t *testing.T, pkg string, names []string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, filepath.Join(root, "go"), pkg, names)
	if code != 0 {
		t.Fatalf("RED: go test %s exited %d\n%s", pkg, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2420_001_PureProvesOnlyHatchFreeClosures(t *testing.T) {
	requireGreen(t, tiaPackage, pureTests)
}

func TestC2420_002_CacheableIsDefaultDenyForEnvAndGitReaders(t *testing.T) {
	requireGreen(t, shipPackage, defaultDeny)
}

func TestC2420_003_HandListStaysAnOverride(t *testing.T) {
	requireGreen(t, shipPackage, handOverride)
}

func TestC2420_004_AmbientEnvChangeDoesNotHit(t *testing.T) {
	requireGreen(t, shipPackage, ambientEnv)
}

func TestC2420_005_ScorerCountsFalseStopEscapeConflict(t *testing.T) {
	requireGreen(t, scorePackage, scorerTests)
}

func TestC2420_006_ShipScanReachesPurityAndTheScorerFromProductionCode(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "list", "-f", `{{join .Imports "\n"}}`, shipPackage)
	if err != nil || code != 0 {
		t.Fatalf("go list %s exited %d: %v\n%s", shipPackage, code, err, errOut)
	}
	for _, want := range []string{modulePath + "/internal/regressiontia", modulePath + "/internal/stagescore"} {
		if !strings.Contains("\n"+out+"\n", "\n"+want+"\n") {
			t.Errorf("RED: the non-test code of %s does not import %s, so the scan skip cannot reach it", shipPackage, want)
		}
	}
}

func TestC2420_007_NewTestsAreRedWithoutTheChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, errOut, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("no baseline revision: exit %d: %v\n%s", code, err, errOut)
	}
	baseline := t.TempDir()
	archive := filepath.Join(t.TempDir(), "base.tar")
	if _, errOut, code, err := acsassert.SubprocessOutput("git", "-C", root, "archive", "--format=tar", "-o", archive, strings.TrimSpace(base), "go"); err != nil || code != 0 {
		t.Fatalf("git archive exited %d: %v\n%s", code, err, errOut)
	}
	if _, errOut, code, err := acsassert.SubprocessOutput("tar", "-xf", archive, "-C", baseline); err != nil || code != 0 {
		t.Fatalf("tar exited %d: %v\n%s", code, err, errOut)
	}
	for _, rel := range []string{purityTestFile, pureTestFile, scoreTestFile} {
		raw, readErr := os.ReadFile(filepath.Join(root, "go", filepath.FromSlash(rel)))
		if readErr != nil {
			t.Fatalf("the new test file %s is missing: %v", rel, readErr)
		}
		dst := filepath.Join(baseline, "go", filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			t.Fatal(mkErr)
		}
		if writeErr := os.WriteFile(dst, raw, 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	redOnBaseline := map[string][]string{
		shipPackage:  append(append([]string{}, defaultDeny...), ambientEnv...),
		tiaPackage:   pureTests,
		scorePackage: scorerTests,
	}
	for pkg, names := range redOnBaseline {
		out, code := goTest(t, filepath.Join(baseline, "go"), pkg, names)
		if code == 0 {
			t.Errorf("the new tests of %s pass without the change:\n%s", pkg, acsassert.OutputTail(out, 20))
			continue
		}
		if pkg == shipPackage {
			for _, name := range names {
				if !strings.Contains(out, "--- FAIL: "+name+" ") {
					t.Errorf("%s must fail by assertion on the baseline, not by a build error:\n%s", name, acsassert.OutputTail(out, 20))
				}
			}
			continue
		}
		if !strings.Contains(out, "undefined") {
			t.Errorf("%s must fail on the baseline because the new API is missing:\n%s", pkg, acsassert.OutputTail(out, 20))
		}
	}
}

func TestC2420_008_NewPackageIsEnrolledAndEveryExportIsNamed(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	if !acsassert.FileContains(t, filepath.Join(root, enrollmentListing), "\n"+enrollmentLine+"\n") {
		t.Errorf("RED: %s is not enrolled in %s", enrollmentLine, enrollmentListing)
	}
	work := t.TempDir()
	profile := filepath.Join(work, "cover.out")
	if out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile", profile, scorePackage); code != 0 {
		t.Fatalf("RED: go test %s exited %d\n%s", scorePackage, code, acsassert.OutputTail(out+errOut, 30))
	}
	funcs, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func", profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exited %d: %v\n%s", code, err, errOut)
	}
	funcFile := filepath.Join(work, "cover.func.txt")
	if writeErr := os.WriteFile(funcFile, []byte(funcs), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	var report strings.Builder
	rc, runErr := apicover.Run(context.Background(), apicover.Config{Dirs: []string{filepath.Join(goDir, "internal", "stagescore")}, CoverPath: funcFile, Enforce: true}, &report)
	if runErr != nil || rc != 0 {
		t.Errorf("RED: apicover -enforce on internal/stagescore exited %d: %v\n%s", rc, runErr, report.String())
	}
}
