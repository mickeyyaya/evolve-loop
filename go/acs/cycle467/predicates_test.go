//go:build acs

package cycle467

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fleetPkg = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	cmdPkg   = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

func TestC467_001_DirtyControlPlaneRefusedActionably(t *testing.T) {
	fleetOut, fleetCode := runGoTest(t, "TestPreflightControlPlane_DirtyPolicyRefusedWithActionableMessage|TestPreflightControlPlane_UntrackedControlPlaneAdditionRefused", true, fleetPkg)
	requireTestsRan(t, fleetOut, 2)
	if fleetCode != 0 {
		t.Errorf("PreflightControlPlane dirty-refusal contract is red (exit=%d)\n%s", fleetCode, fleetOut)
	}
	cmdOut, cmdCode := runGoTest(t, "TestDispatchIteration_PreflightRefusalNeverPlansNorLaunches", true, cmdPkg)
	requireTestsRan(t, cmdOut, 1)
	if cmdCode != 0 {
		t.Errorf("dispatchIteration preflight-refusal seam contract is red (exit=%d)\n%s", cmdCode, cmdOut)
	}
}

func TestC467_002_CleanAndInnocentTreesNeverFalsePositive(t *testing.T) {
	fleetOut, fleetCode := runGoTest(t, "TestPreflightControlPlane_CleanTreePasses|TestPreflightControlPlane_NonControlPlaneDirtIgnored|TestPreflightControlPlane_NotAGitRepoErrors", true, fleetPkg)
	requireTestsRan(t, fleetOut, 3)
	if fleetCode != 0 {
		t.Errorf("PreflightControlPlane no-false-positive/fail-loud contract is red (exit=%d)\n%s", fleetCode, fleetOut)
	}
	cmdOut, cmdCode := runGoTest(t, "TestDispatchIteration_PreflightCleanWaveProceeds|TestDispatchIteration_SequentialPathNeverRunsPreflight", true, cmdPkg)
	requireTestsRan(t, cmdOut, 2)
	if cmdCode != 0 {
		t.Errorf("dispatchIteration clean-preflight/sequential-skip contract is red (exit=%d)\n%s", cmdCode, cmdOut)
	}
}

func TestC467_003_QuotaAwareCountShrinksWithMinOneClamp(t *testing.T) {
	out, code := runGoTest(t, "TestQuotaAwareCount", true, fleetPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("QuotaAwareCount shrink/clamp/pass-through contract is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	cmdLoop := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	cmdLoopWave := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_wave.go")
	if !acsassert.FileContainsAny(cmdLoop, "QuotaAwareCount") && !acsassert.FileContainsAny(cmdLoopWave, "QuotaAwareCount") {
		t.Errorf("neither cmd_loop.go nor cmd_loop_wave.go references QuotaAwareCount — the shrink is not wired into the live wave path")
	}
}

func TestC467_004_WaveLevelFileDisjointnessPinned(t *testing.T) {
	out, code := runGoTest(t, "TestPlanWaves_WaveLevelFileDisjoint|TestPlanFromTriage_WaveLevelFileDisjoint", true, fleetPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("wave-level file-disjointness regression pin is red (exit=%d)\n%s", code, out)
	}
}

func TestC467_005_CtxThreadedThroughPlanPath(t *testing.T) {
	out, code := runGoTest(t, "TestDispatchIteration_CtxReachesPlanFn|TestDispatchIteration_CancelledCtxObservableInPlanFn", true, cmdPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("ctx-threading contract is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	acsassert.FileNotContains(t, filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_wave.go"), "context.Background()")
}

func TestC467_006_RaceVetApicoverCleanWithZeroGuardsEdits(t *testing.T) {
	out, code := runGoTest(t, "", true, fleetPkg, cmdPkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/fleet + cmd/evolve is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./cmd/evolve/... ./internal/fleet/...")
	if vetCode != 0 {
		t.Errorf("go vet ./cmd/evolve/... ./internal/fleet/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	apicoverCmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.s3guards467.txt ./internal/fleet/ >/dev/null && " +
		"go tool cover -func=coverage.s3guards467.txt > coverage.s3guards467.func.txt && " +
		"bin/apicover -enforce -cover coverage.s3guards467.func.txt $(go list -f '{{.Dir}}' ./internal/fleet)"
	apiOut, _, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/fleet is red (exit=%d)\n%s", apiCode, apiOut)
	}
	statusOut, statusErr, statusCode, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--", "go/internal/guards/")
	if statusCode != 0 {
		t.Errorf("git status on go/internal/guards/ failed (exit=%d)\n%s", statusCode, statusErr)
	} else if strings.TrimSpace(statusOut) != "" {
		t.Errorf("uncommitted edits under go/internal/guards/ (protected surface — the cycle must never touch it):\n%s", statusOut)
	}
	diffOut, diffErr, diffCode, _ := acsassert.SubprocessOutput("bash", "-c",
		`cd `+root+` && git diff --name-only "$(git merge-base main HEAD)" HEAD -- go/internal/guards/`)
	if diffCode != 0 {
		t.Errorf("git diff merge-base check on go/internal/guards/ failed (exit=%d)\n%s", diffCode, diffErr)
	} else if strings.TrimSpace(diffOut) != "" {
		t.Errorf("committed edits under go/internal/guards/ since main (protected surface):\n%s", diffOut)
	}
}
