//go:build acs

package cycle657

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	full := append([]string{"test"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go test failed to run: %v\n%s", err, out)
	return string(out), -1
}

func TestC657_001_AutofileFromFixtureRetro(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1",
		"-run", "TestParsePreventiveActions_FromRetroReport|TestParsePreventiveActions_AbsentReturnsNil|TestFileActions_EndToEndFromFixtureRetro",
		"./internal/retrofile/")
	if code != 0 {
		t.Errorf("AC1: the retro→inbox parse+file suite must PASS; got exit=%d\n%s", code, out)
	}
}

func TestC657_002_DedupFilesOnceWhileOpen(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1",
		"-run", "TestFileActions_DedupSkipsExistingOpenItem|TestFileActions_DedupSkipsExistingProcessedItem|TestFileActions_DedupAcrossTwoConsecutiveFailsFilesOnce",
		"./internal/retrofile/")
	if code != 0 {
		t.Errorf("AC2: the dedup suite must PASS (files once while open, skips processed); got exit=%d\n%s", code, out)
	}
}

func TestC657_003_WeightFromPolicyWithRecurrenceHint(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1",
		"-run", "TestFileActions_UsesDefaultWeightWhenNoHint|TestFileActions_UsesHintForRecurrenceFlagged",
		"./internal/retrofile/")
	if code != 0 {
		t.Errorf("AC3a: the weight-default/hint suite must PASS; got exit=%d\n%s", code, out)
	}
	out2, code2 := goTest(t, "-count=1",
		"-run", "TestRetroAutofileDefaultWeight_DefaultAndOverride", "./internal/policy/")
	if code2 != 0 {
		t.Errorf("AC3b: the policy default-weight accessor test must PASS (weight sourced from policy.json); got exit=%d\n%s", code2, out2)
	}
}

// acs-predicate: config-check
func TestC657_004_RetroContractHasStructuredPreventiveActions(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "agents", "evolve-retrospective.md")
	if !acsassert.LineContainsAll(doc, "preventive_actions") {
		t.Errorf("AC4: %s must document a structured preventive_actions section for the autofiler", doc)
	}
	if !acsassert.FileContainsAny(doc, "weight_hint") {
		t.Errorf("AC4: %s must document the weight_hint field of the structured preventive_actions schema", doc)
	}
}

func TestC657_005_ModuleVetsAndPackageGraduated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("AC5: `go vet ./...` must be clean with the new package wired; got err=%v\n%s", err, out)
	}
	enforce := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.FileContains(t, enforce, "./internal/retrofile") {
		t.Errorf("AC5: internal/retrofile must be graduated into go/.apicover-enforce (new-package obligation)")
	}
}
