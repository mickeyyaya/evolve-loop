//go:build acs

package cycle1253

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	modulePkg      = "github.com/mickeyyaya/evolve-loop/go/..."
)

func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s",
				name, stdout)
		}
	}
}

func TestC1253_001_ImporterClosureReproducer(t *testing.T) {
	runGoTest(t, changedpkgsPkg,
		"^TestImporterClosure_(RouterRoutingtest|ExcludesNonImporters)$",
		[]string{
			"TestImporterClosure_RouterRoutingtest",
			"TestImporterClosure_ExcludesNonImporters",
		})
}

func TestC1253_002_TransitiveBestEffortAndShape(t *testing.T) {
	runGoTest(t, changedpkgsPkg,
		"^TestImporterClosure_(Transitive|BestEffortOnBadInput|SortedDedupedAndModuleRoot)$",
		[]string{
			"TestImporterClosure_Transitive",
			"TestImporterClosure_BestEffortOnBadInput",
			"TestImporterClosure_SortedDedupedAndModuleRoot",
		})
}

func TestC1253_003_NewExportCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-coverprofile="+profile, changedpkgsPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -coverprofile %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			changedpkgsPkg, code, err, stdout, stderr)
	}

	funcs, ferr, fcode, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if fcode != 0 || err != nil {
		t.Fatalf("go tool cover -func exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			fcode, err, funcs, ferr)
	}

	var line string
	for _, l := range strings.Split(funcs, "\n") {
		if strings.Contains(l, "ImporterClosure") {
			line = strings.TrimSpace(l)
			break
		}
	}
	if line == "" {
		t.Fatalf("ImporterClosure absent from the coverage profile — the export does not exist or no test names it\n%s", funcs)
	}
	if strings.HasSuffix(line, "0.0%") {
		t.Errorf("ImporterClosure has 0.0%% coverage — named but never executed (apicover false-green): %q", line)
	}
}

func TestC1253_004_ModuleBuildsAndVets(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}

	stdout, stderr, code, err = acsassert.SubprocessOutput("go", "vet", changedpkgsPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			changedpkgsPkg, code, err, stdout, stderr)
	}
}
