//go:build acs

package cycle1947

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	convergencePkg = "./internal/convergence"
	propertyRun    = "^TestProperty_"
)

func goTest(t *testing.T, root string, overlay string, run string) (string, int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v", "-run", run}
	if overlay != "" {
		args = append(args, "-overlay", overlay)
	}
	cmd := exec.Command("go", append(args, convergencePkg)...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("go test did not start: %v", err)
	}
	return string(out), exitErr.ExitCode()
}

func mutantOverlay(t *testing.T, root, file, from, to string) string {
	t.Helper()
	src := filepath.Join(root, "go", "internal", "convergence", file)
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), from) != 1 {
		t.Fatalf("%s must contain the mutation site exactly once: %q", file, from)
	}
	dir := t.TempDir()
	mutated := filepath.Join(dir, file)
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(body), from, to, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	doc, err := json.Marshal(map[string]map[string]string{"Replace": {src: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, doc, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func requireMutantKilled(t *testing.T, file, from, to string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, root, mutantOverlay(t, root, file, from, to), propertyRun)
	if code == 0 || !strings.Contains(out, "--- FAIL: TestProperty_") {
		t.Errorf("RED: the property test survives the mutant (exit %d); it must FAIL under it:\n%s", code, out)
	}
}

func TestC1947_001_PropertyTestPassesOnTheRealPolicyWithinTheInputBand(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, root, "", propertyRun)
	if code != 0 || !strings.Contains(out, "--- PASS: TestProperty_") {
		t.Fatalf("RED: no passing TestProperty_ test in %s (exit %d):\n%s", convergencePkg, code, out)
	}
	inputs := regexp.MustCompile(`inputs=(\d+)`).FindAllStringSubmatch(out, -1)
	if len(inputs) == 0 {
		t.Fatalf("RED: the property test must t.Logf(\"inputs=%%d\", n) so the sweep size is observable:\n%s", out)
	}
	for _, m := range inputs {
		n, _ := strconv.Atoi(m[1])
		if n < 2000 || n > 5000 {
			t.Errorf("RED: a property sweep ran %d inputs, want 2000 to 5000 (fixed seed, fast tier)", n)
		}
	}
}

func TestC1947_002_PropertyTestKillsAKeepBestMassFirstSwap(t *testing.T) {
	requireMutantKilled(t, "findings.go",
		"return r.strict < than.strict || (r.strict == than.strict && r.mass < than.mass)",
		"return r.mass < than.mass || (r.mass == than.mass && r.strict < than.strict)")
}

func TestC1947_003_PropertyTestKillsRemovingTheCriticalClauseInExempted(t *testing.T) {
	requireMutantKilled(t, "findings.go",
		"return f.Severity != SeverityCritical && slices.Contains(exempt, componentOf(f))",
		"return slices.Contains(exempt, componentOf(f))")
}

func TestC1947_004_TheLandingGuardStopNamesItsOwnCauseAndOtherStopsKeepTheirs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, root, "", "^TestLandingGuard_")
	if code != 0 || strings.Count(out, "--- PASS: TestLandingGuard_") < 3 {
		t.Fatalf("RED: the three TestLandingGuard_ tests must pass (exit %d):\n%s", code, out)
	}
}

func TestC1947_005_TheOnceOnlyRaiseIsPinnedAtMaxFixRoundsFourAndStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, root, "", "^TestFeedbackChange_OnceOnlyAtMaxFixRoundsFour$")
	if code != 0 || !strings.Contains(out, "--- PASS: TestFeedbackChange_OnceOnlyAtMaxFixRoundsFour") {
		t.Fatalf("the once-only raise pin must pass (exit %d):\n%s", code, out)
	}
}

// acs-predicate: config-check
func TestC1947_006_SpecAdrAndPackageDocStateTheSameOnceOnlyRaiseRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs := []string{
		"docs/architecture/convergence-policy.md",
		"docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md",
		"docs/architecture/packages/internal-convergence.md",
	}
	for _, rel := range docs {
		path := filepath.Join(root, rel)
		if !acsassert.FileContains(t, path, "once-only raise") {
			t.Errorf("RED: %s does not state the once-only raise rule", rel)
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is not git-tracked", rel)
		}
	}
}
