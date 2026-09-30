//go:build acs

package cycle433

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	panestreamImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	bridgeImportPath     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	adrRelPath           = "docs/architecture/adr/0068-bridge-signal-center-concurrency.md"
)

func runRace(t *testing.T, runFilter string, pkgs ...string) (string, string, int) {
	t.Helper()
	args := []string{"test", "-race", "-count=1"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout, stderr, code
}

func adrPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), adrRelPath)
}

func TestC433_001_ParallelEvaluateStressRaceClean(t *testing.T) {
	runFilter := "TestSignalCenter_ParallelEvaluateStress_MixedOpsRaceClean|" +
		"TestSignalCenter_EmptyCenter_DefinedState|TestSignalCenter_UnknownKeyIsQuiet"
	stdout, stderr, code := runRace(t, runFilter, panestreamImportPath)
	if code != 0 {
		t.Errorf("C433_001: mixed-op stress test exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

func TestC433_002_SameKeyObserveAggregateRaceClean(t *testing.T) {
	_, stderr, code := runRace(t, "TestSignalCenter_ObserveAggregateSameKeyRaceClean", panestreamImportPath)
	if code != 0 {
		t.Errorf("C433_002: same-key race test (BA2 guard) exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC433_003_StressTestExercisesAllFiveOps(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge", "panestream",
		"livenesscenter_parallelevaluate_test.go")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("C433_003: read %s: %v", path, err)
	}
	src := string(b)
	for _, op := range []string{"NewLivenessCenter(", ".Observe(", ".Aggregate(", ".Busy(", ".Changed(", "RegisterHandler("} {
		if !strings.Contains(src, op) {
			t.Errorf("C433_003: livenesscenter_parallelevaluate_test.go missing a call to %q — the stress test must exercise ALL FIVE SignalCenter ops on a shared center, not a cheap fake that only drives Observe on distinct keys", op)
		}
	}
}

func TestC433_004_ApicoverEnforceCleanBothPackages(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "apicover433")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", goDir, "-o", binPath, "./cmd/apicover",
	); code != 0 {
		t.Fatalf("C433_004: build apicover binary exit=%d: %s", code, stderr)
	}

	coverPath := filepath.Join(tmp, "coverage433.txt")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1",
		"-coverprofile="+coverPath,
		"./internal/bridge/...", "./internal/bridge/panestream/...",
	); code != 0 {
		t.Fatalf("C433_004: coverage run exit=%d: %s", code, stderr)
	}

	funcOut, funcErr, code, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+coverPath)
	if code != 0 {
		t.Fatalf("C433_004: go tool cover -func exit=%d: %s", code, funcErr)
	}
	funcPath := filepath.Join(tmp, "coverage433.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("C433_004: write func profile: %v", err)
	}

	dirOut, dirErr, code, _ := acsassert.SubprocessOutput(
		"go", "list", "-C", goDir, "-f", "{{.Dir}}",
		"./internal/bridge", "./internal/bridge/panestream",
	)
	if code != 0 {
		t.Fatalf("C433_004: go list package dirs exit=%d: %s", code, dirErr)
	}
	dirs := strings.Fields(dirOut)
	if len(dirs) != 2 {
		t.Fatalf("C433_004: expected 2 package dirs, got %v", dirs)
	}

	args := append([]string{"-cover", funcPath, "-enforce"}, dirs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(binPath, args...)
	if code != 0 {
		t.Errorf("C433_004: apicover -enforce exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

func TestC433_005_ADRNoLongerDeferred(t *testing.T) {
	acsassert.FileNotContains(t, adrPath(t), "Deferred (S5)")
}

func TestC433_006_ADRDocumentsConcurrencyModel(t *testing.T) {
	path := adrPath(t)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("C433_006: read ADR: %v", err)
	}
	lower := strings.ToLower(string(b))

	hasLockOrder := strings.Contains(lower, "lock order") || strings.Contains(lower, "acquisition order")
	if !hasLockOrder {
		t.Errorf("C433_006: ADR-0068 missing lock-ordering documentation (want 'lock order'/'lock ordering'/'acquisition order')")
	}

	hasOwnership := strings.Contains(lower, "ownership") || strings.Contains(lower, "mutat")
	if !hasOwnership {
		t.Errorf("C433_006: ADR-0068 missing ownership/mutation documentation (want 'ownership' or 'mutat(e/ion)')")
	}
}

func TestC433_007_ContentionBenchmarkExistsAndRuns(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-run", "^$", "-bench", "SignalCenter_ParallelObserve",
		"-benchtime=10x", "./internal/bridge/panestream/...",
	)
	if code != 0 {
		t.Errorf("C433_007: benchmark run exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
		return
	}
	if !strings.Contains(stdout, "BenchmarkSignalCenter_ParallelObserve") {
		t.Errorf("C433_007: no BenchmarkSignalCenter_ParallelObserve in output — benchmark does not exist yet\nstdout=%s", stdout)
	}
}

func TestC433_008_FullRaceSuiteGreen(t *testing.T) {
	_, stderr, code := runRace(t, "", bridgeImportPath, panestreamImportPath)
	if code != 0 {
		t.Errorf("C433_008: full bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}
