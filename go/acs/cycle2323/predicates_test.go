//go:build acs

package cycle2323

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const linkTest = "TestResumableReasonsAreValidCheckpointReasons"

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func runLinkTest(t *testing.T, extraArgs ...string) (string, error) {
	t.Helper()
	args := append([]string{"test", "-count=1", "-v"}, extraArgs...)
	args = append(args, "-run", "^"+linkTest+"$", "./internal/checkpoint")
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleDir(t)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func bogusKeyOverlay(t *testing.T) string {
	t.Helper()
	module := moduleDir(t)
	resumeSource := filepath.Join(module, "internal", "core", "resume.go")
	original, err := os.ReadFile(resumeSource)
	if err != nil {
		t.Fatal(err)
	}
	anchor := regexp.MustCompile(`("stall-inactivity":\s+true,\n)`)
	if !anchor.Match(original) {
		t.Fatalf("resumableReasons anchor not found in %s", resumeSource)
	}
	mutant := anchor.ReplaceAll(original, []byte("${1}\t\"bogus-not-a-checkpoint-reason\": true,\n"))
	dir := t.TempDir()
	mutantPath := filepath.Join(dir, "resume_mutant.go")
	if err := os.WriteFile(mutantPath, mutant, 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {resumeSource: mutantPath}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlayPath
}

func TestC2323_001_TheRealResumableReasonsAreAllValidCheckpointReasons(t *testing.T) {
	out, err := runLinkTest(t)
	if err != nil {
		t.Fatalf("%s did not pass on the real tree: %v\n%s", linkTest, err, acsassert.OutputTail(out, 25))
	}
	if !strings.Contains(out, "--- PASS: "+linkTest) {
		t.Errorf("%s did not run and pass (a missing test exits 0 with no PASS line):\n%s", linkTest, acsassert.OutputTail(out, 25))
	}
}

func TestC2323_002_AResumableReasonThatIsNotACheckpointReasonFailsTheTest(t *testing.T) {
	out, err := runLinkTest(t, "-overlay="+bogusKeyOverlay(t))
	if err == nil {
		t.Fatalf("overlay mutant with a bogus resumable key left %s green:\n%s", linkTest, acsassert.OutputTail(out, 25))
	}
	if !strings.Contains(out, "--- FAIL: "+linkTest) {
		t.Errorf("mutant must fail through the %s assertion, not through a build error:\n%s", linkTest, acsassert.OutputTail(out, 25))
	}
	if !strings.Contains(out, "bogus-not-a-checkpoint-reason") {
		t.Errorf("failure must name the bogus reason:\n%s", acsassert.OutputTail(out, 25))
	}
}

func TestC2323_003_SettledCheckpointReasonsStayNonResumable(t *testing.T) {
	for _, reason := range []checkpoint.Reason{checkpoint.ReasonPhaseComplete, checkpoint.ReasonRetired} {
		if !reason.IsValid() {
			t.Errorf("%q must stay a valid checkpoint reason", reason)
		}
		if core.IsResumableReason(string(reason)) {
			t.Errorf("%q must not be resumable", reason)
		}
	}
	for _, reason := range []checkpoint.Reason{checkpoint.ReasonQuotaLikely, checkpoint.ReasonBatchCapNear, checkpoint.ReasonOperatorRequest, checkpoint.ReasonStallInactivity} {
		if !core.IsResumableReason(string(reason)) {
			t.Errorf("%q must stay resumable", reason)
		}
	}
}
