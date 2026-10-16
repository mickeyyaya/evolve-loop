//go:build acs

package cycle1906

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	subagentPkg   = "./internal/subagent"
	retiredScript = "legacy/scripts/dispatch/subagent-run.sh"
	parityFile    = "internal/subagent/validateprofile_parity_test.go"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func listSubagentTests(t *testing.T, pattern string) string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-count=1", "-list", pattern, subagentPkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -list exit=%d err=%v\n%s%s", code, err, out, errOut)
	}
	return out
}

func TestC1906_001_BashParityTestIsNotRegisteredInSubagentPackage(t *testing.T) {
	out := listSubagentTests(t, "BashParity")
	if strings.Contains(out, "TestValidateProfile_BashParity") {
		t.Errorf("RED: dead TestValidateProfile_BashParity still registered:\n%s", out)
	}
}

func TestC1906_002_ParityFileIsDeletedFromDisk(t *testing.T) {
	path := filepath.Join(goDir(t), parityFile)
	if _, err := os.Stat(path); err == nil {
		t.Errorf("RED: %s still exists", parityFile)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", parityFile, err)
	}
}

func TestC1906_003_NoSubagentSourceReferencesRetiredDispatchScript(t *testing.T) {
	dir := filepath.Join(goDir(t), "internal", "subagent")
	var offenders []string
	walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), retiredScript) || strings.Contains(string(b), `"subagent-run.sh"`) {
			offenders = append(offenders, strings.TrimPrefix(p, dir+string(filepath.Separator)))
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", dir, walkErr)
	}
	if len(offenders) > 0 {
		t.Errorf("RED: files still reference the retired bash dispatch script: %v", offenders)
	}
}

func TestC1906_004_ValidateProfileCoverageSurvivesRemoval(t *testing.T) {
	out := listSubagentTests(t, "^TestValidateProfile")
	if !strings.Contains(out, "TestValidateProfile") {
		t.Fatalf("ValidateProfile unit tests missing from package:\n%s", out)
	}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "TestValidateProfile") {
			count++
		}
	}
	if count < 10 {
		t.Errorf("only %d ValidateProfile tests remain, want >= 10 (validateprofile_test.go coverage must be intact)", count)
	}
}

func TestC1906_005_SubagentPackageTestsPassAfterRemoval(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-count=1", "-run", "TestValidateProfile", subagentPkg)
	if err != nil || code != 0 {
		t.Errorf("subagent ValidateProfile tests failed exit=%d err=%v\n%s%s", code, err, out, errOut)
	}
}

func TestC1906_006_PackageVetsCleanAndParityHelpersAreGone(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "vet", subagentPkg)
	if err != nil || code != 0 {
		t.Fatalf("go vet exit=%d err=%v\n%s%s", code, err, out, errOut)
	}
	dir := filepath.Join(goDir(t), "internal", "subagent")
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, fileErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if fileErr != nil {
			t.Fatal(fileErr)
		}
		for _, helper := range []string{"func readPlanJSON(", "func mapsEqual(", "func valuesEqual("} {
			if strings.Contains(string(b), helper) {
				t.Errorf("RED: %s still declares %s", e.Name(), helper)
			}
		}
	}
}
