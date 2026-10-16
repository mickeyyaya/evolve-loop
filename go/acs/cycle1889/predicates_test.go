//go:build acs

package cycle1889

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTestRun(t *testing.T, pkg string, names ...string) string {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test %s %v failed: %v\n%s", pkg, names, err, out.String())
	}
	got := out.String()
	for _, name := range names {
		if !strings.Contains(got, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass\n%s", name, got)
		}
	}
	return got
}

func TestC1889_001_FailedBoundaryStepDetailCarriesItsStderrTailBesideRC(t *testing.T) {
	goTestRun(t, "./cmd/evolve",
		"TestWaveNext_FailedStepDetailCarriesTheLast40StderrLinesBesideRC",
		"TestWaveNext_FailedStepTailHoldsOnlyThatStepsStderr",
		"TestWaveNext_StepStderrStillReachesTheCallersStderr",
	)
}

func TestC1889_002_DoctorLiveFailureNamesItsFailingStageInsideTheTail(t *testing.T) {
	goTestRun(t, "./internal/cli/opscmd",
		"TestDoctorLive_FailureNamesItsFailingStage",
		"TestDoctorLive_EveryFailureCodeNamesExactlyOneStageInsideTheWaveTail",
		"TestDoctorLive_HealthyProbeNamesNoFailingStage",
	)
}

func TestC1889_003_BoundaryLogCarriesTheFailedStepsStderrTail(t *testing.T) {
	goTestRun(t, "./cmd/evolve",
		"TestBoundary_StepFailureWritesTheStepsStderrTailAfterTheFailureLine",
		"TestBoundary_PassingRunWritesNoStderrTail",
	)
}

// acs-predicate: config-check
func TestC1889_004_ChangedProductionGoAddsNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output()
	if err != nil {
		t.Fatalf("merge-base HEAD origin/main: %v", err)
	}
	diff, err := exec.Command("git", "-C", root, "diff", "-U0", strings.TrimSpace(string(base)), "--",
		"go/cmd/evolve/cmd_wave_next.go", "go/cmd/evolve/cmd_boundary.go", "go/internal/cli/opscmd/doctor_live.go").Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	addedComment := regexp.MustCompile(`(?m)^\+[^+].*(^|\s|;|\))//\s|^\+\s*//|^\+.*/\*`)
	if hits := addedComment.FindAllString(string(diff), -1); len(hits) > 0 {
		t.Errorf("changed production Go adds comments: %q", hits)
	}
}
