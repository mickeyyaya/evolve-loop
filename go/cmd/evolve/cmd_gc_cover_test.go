package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func TestGCDispatchProcesses_AFailedListingSkipsTheSweepAndFails(t *testing.T) {
	var signalled []int
	orig := gcInjectedProcessHost
	t.Cleanup(func() { gcInjectedProcessHost = orig })
	gcInjectedProcessHost = &gcProcessHost{
		list:   func(context.Context) ([]proctree.Process, error) { return nil, errors.New("ps: exit 1") },
		signal: func(pid int, _ syscall.Signal) error { signalled = append(signalled, pid); return nil },
		sleep:  func(time.Duration) {},
		alive:  func(int) bool { return false },
	}
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), false, &stdout, &stderr).dispatchProcesses(t.TempDir(), 24)

	if !failed || !strings.Contains(stderr.String(), "evolve gc: dispatch process sweep skipped: ps: exit 1") || len(signalled) != 0 {
		t.Errorf("failed=%v stderr=%q signalled=%v, want a failed sweep, the skip line and no signal", failed, stderr.String(), signalled)
	}
}

func TestGCRecordedTree_ACorruptTreeWarnsAndLeavesOnlyTheTagProof(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := "01R/1835/build/p999n1"
	if err := os.WriteFile(proctree.TreeFile(dir, id), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	got := newGCRun(context.Background(), true, &stdout, &stderr).recordedTree(dir)(id)

	if got != nil || !strings.Contains(stderr.String(), `evolve gc: WARN: dispatch "01R/1835/build/p999n1": decode dispatch tree`) || !strings.Contains(stderr.String(), "only the tag proof applies") {
		t.Errorf("recordedTree = %v, stderr=%q, want no tree and the decode WARN", got, stderr.String())
	}
}

func TestGCReportDispatchReap_AnErrorOrASurvivorFailsTheSweep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		rep  proctree.Report
		want string
	}{
		{proctree.Report{Errors: []string{"pid 30469: operation not permitted"}}, "evolve gc: dispatch process error: pid 30469: operation not permitted"},
		{proctree.Report{Killed: []proctree.Process{{Pid: 30469}}, Survivors: []proctree.Process{{Pid: 30469, Comm: "node"}}}, "evolve gc: ERROR: pid 30469 (node) is alive after SIGKILL"},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		failed := newGCRun(context.Background(), false, &stdout, &stderr).reportDispatchReap(tc.rep)
		if !failed || !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("reportDispatchReap(%+v) = %v, stderr=%q, want a failure and %q", tc.rep, failed, stderr.String(), tc.want)
		}
	}
	var stdout, stderr bytes.Buffer
	if newGCRun(context.Background(), false, &stdout, &stderr).reportDispatchReap(proctree.Report{Terminated: []proctree.Process{{Pid: 1}}}) || !strings.Contains(stdout.String(), "stopped 1 dispatch process(es) (SIGKILL: 0)") {
		t.Errorf("a clean reap: stdout=%q, want no failure and the stopped count", stdout.String())
	}
}

func TestRunGC_ABadFlagIsAUsageError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	if rc := runGC([]string{"--bogus"}, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), "flag provided but not defined: -bogus") {
		t.Errorf("rc=%d stderr=%q, want 10 and the flag error", rc, stderr.String())
	}
}

func TestGCProject_AMutatingRunWithoutARootIsRefused(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	rc := newGCRun(context.Background(), false, &stdout, &stderr).project("")

	if rc != 1 || !strings.Contains(stderr.String(), "mutating run refused: --project-root must be explicitly set") {
		t.Errorf("rc=%d stderr=%q, want the refusal", rc, stderr.String())
	}
}

func TestGCProject_AnUnparseablePolicyWarnsAndSweepsWithTheZeroPolicy(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	withGCProcessHost(t, &fakeGCProcessHost{})
	if err := os.WriteFile(filepath.Join(projectRoot, ".evolve", "policy.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	newGCRun(context.Background(), true, &stdout, &stderr).project(projectRoot)

	if !strings.Contains(stderr.String(), "evolve gc: WARN: policy load failed:") || !strings.Contains(stderr.String(), "using zero-value gc policy") {
		t.Errorf("stderr=%q, want the policy WARN", stderr.String())
	}
}

func TestGCRunDirs_ADiscoveryOrPlanFailureIsReported(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeBoundaryFile(t, filepath.Join(evolveDir, "runs"), "")
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)

	if failed := r.runDirs(evolveDir, gc.Policy{}); !failed || !strings.Contains(stderr.String(), "evolve gc: run-dir discovery failed: gc: read runs dir") {
		t.Errorf("runs as a file: failed=%v stderr=%q, want the discovery failure", failed, stderr.String())
	}
	stderr.Reset()
	if failed := r.runDirs("relative/.evolve", gc.Policy{}); !failed || !strings.Contains(stderr.String(), "evolve gc: run-dir retention plan failed: gc: EvolveDir must be absolute") {
		t.Errorf("relative dir: failed=%v stderr=%q, want the plan failure", failed, stderr.String())
	}
}

func TestRunRunDirGC_ADiscoveryOrPlanFailureIsAWarning(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeBoundaryFile(t, filepath.Join(evolveDir, "runs"), "")
	workspace := t.TempDir()
	var stderr bytes.Buffer

	runRunDirGC(loopConfig{EvolveDir: evolveDir}, workspace, "shadow", gc.Policy{}, &stderr)
	if !strings.Contains(stderr.String(), "[gc] WARN: discover failed: gc: read runs dir") || !strings.Contains(stderr.String(), "[gc] shadow: 0 items") {
		t.Errorf("stderr=%q, want the discover WARN and an empty shadow manifest", stderr.String())
	}
	stderr.Reset()
	runRunDirGC(loopConfig{EvolveDir: "relative/.evolve"}, workspace, "enforce", gc.Policy{}, &stderr)
	if !strings.Contains(stderr.String(), "[gc] WARN: plan failed: gc: EvolveDir must be absolute") || strings.Contains(stderr.String(), "enforce: applied") {
		t.Errorf("stderr=%q, want the plan WARN and nothing applied", stderr.String())
	}
}

func TestDispatchLogTTLDays_AnUnparseablePolicyUsesTheDefaultCatalog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBoundaryFile(t, filepath.Join(root, ".evolve", "policy.json"), "{")
	var stderr bytes.Buffer

	got := dispatchLogTTLDays(root, &stderr)

	if got != 30 || !strings.Contains(stderr.String(), "[prune-ephemeral] WARN: policy load failed:") {
		t.Errorf("dispatchLogTTLDays = %d, stderr=%q, want 30 and the WARN", got, stderr.String())
	}
}

func TestRunRunDirGC_AFailedEnforceApplyIsAWarning(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes from a read-only directory")
	}
	projectRoot := t.TempDir()
	dirs := gcAgedRunDirs(t, projectRoot, 12)
	var pol gc.Policy
	if err := json.Unmarshal([]byte(`{"runs":{"delete_after_days":14}}`), &pol); err != nil {
		t.Fatal(err)
	}
	runsDir := filepath.Dir(dirs[0])
	if err := os.Chmod(runsDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(runsDir, 0o755) })
	var stderr bytes.Buffer

	runRunDirGC(loopConfig{EvolveDir: filepath.Join(projectRoot, ".evolve")}, t.TempDir(), "enforce", pol, &stderr)

	if !strings.Contains(stderr.String(), "[gc] WARN: enforce apply failed:") || strings.Contains(stderr.String(), "[gc] enforce: applied") {
		t.Errorf("stderr=%q, want the apply WARN and no applied line", stderr.String())
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s is gone after a failed apply: %v", d, err)
		}
	}
}
