//go:build acs

package cycle1891

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const archiveRel = "docs/private/research/archived-2026-10-16/superseded-predicate-packages"

var retiredPackages = []string{"cycle1741", "cycle1742", "cycle1759", "cycle1770"}

func runGoTest(t *testing.T, run, pkg string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "test", "-tags", "acs", "-count=1", "-run", run, pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC1891_001_RetiredPackagesLeaveActiveTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, pkg := range retiredPackages {
		if _, err := os.Stat(filepath.Join(root, "go", "acs", pkg)); !os.IsNotExist(err) {
			t.Errorf("RED: go/acs/%s still in the active tree (stat err=%v)", pkg, err)
		}
	}
}

func TestC1891_002_RetiredPackagesArchivedWithSources(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, pkg := range retiredPackages {
		src := filepath.Join(root, archiveRel, pkg, "predicates_test.go")
		info, err := os.Stat(src)
		if err != nil || info.Size() == 0 {
			t.Errorf("RED: archived source %s missing or empty (err=%v)", src, err)
			continue
		}
		out, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", filepath.Join(archiveRel, pkg, "predicates_test.go"))
		if code != 0 {
			t.Errorf("RED: %s untracked, would be dropped at ship: %s", pkg, out)
		}
	}
}

func TestC1891_003_ArchiveReadmeListsEachPackageWithReason(t *testing.T) {
	root := acsassert.RepoRoot(t)
	readme := filepath.Join(root, archiveRel, "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("RED: archive README unreadable: %v", err)
	}
	lines := strings.Split(string(raw), "\n")
	for _, pkg := range retiredPackages {
		found := false
		for _, line := range lines {
			if strings.Contains(line, pkg) && len(strings.Fields(line)) >= 6 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RED: README has no line naming %s together with a reason", pkg)
		}
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", filepath.Join(archiveRel, "README.md"))
	if code != 0 {
		t.Errorf("RED: README untracked")
	}
}

func TestC1891_004_Cycle50StrictPassPredicatePassesOnFormattedSource(t *testing.T) {
	out, err := runGoTest(t, "^TestC50B_006_ReleasePipelineOptions_HasStrictPassField$", "./acs/cycle50")
	if err != nil {
		t.Errorf("RED: TestC50B_006 fails on gofmt-aligned struct: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("RED: no ok receipt from cycle50 run:\n%s", out)
	}
}

func TestC1891_005_Cycle50PredicateHasNoSingleSpaceAnchor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "acs", "cycle50", "predicates_test.go")
	if !acsassert.FileNotContains(t, f, `"StrictPass bool"`) {
		t.Errorf("RED: cycle50 still anchors on the single-space literal")
	}
}

func TestC1891_006_FieldPatternDiscriminatesRealSourceFromRenamedField(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "go", "internal", "releasepipeline", "releasepipeline.go"))
	if err != nil {
		t.Fatal(err)
	}
	fieldPattern := regexp.MustCompile(`StrictPass\s+bool`)
	if !fieldPattern.Match(src) {
		t.Errorf("RED: pattern does not match the real gofmt-aligned source")
	}
	renamed := strings.ReplaceAll(string(src), "StrictPass", "Strictpass")
	if fieldPattern.MatchString(renamed) {
		t.Errorf("RED: pattern matches a source with the field renamed away")
	}
	cycle50, err := os.ReadFile(filepath.Join(root, "go", "acs", "cycle50", "predicates_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cycle50), `StrictPass\s+bool`) {
		t.Errorf("RED: cycle50 predicate does not use the whitespace-tolerant pattern")
	}
}

func TestC1891_007_Cycle1702DependentSuiteGreen(t *testing.T) {
	out, err := runGoTest(t, "^TestC1702_001_Cycle50SuiteGreen$", "./acs/cycle1702")
	if err != nil {
		t.Errorf("RED: cycle1702 dependent suite fails: %v\n%s", err, out)
	}
}
