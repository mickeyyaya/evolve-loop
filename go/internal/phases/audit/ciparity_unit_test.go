package audit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func fakeRunFunc(code int, stdout, stderr string, runErr error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		_, _ = io.WriteString(so, stdout)
		_, _ = io.WriteString(se, stderr)
		return code, runErr
	}
}

func withFakeRunner(t *testing.T, r sysexec.RunFunc) {
	t.Helper()
	orig := runCmd
	runCmd = r
	t.Cleanup(func() { runCmd = orig })
}

func goWorktree(t *testing.T) (root, goDir string) {
	t.Helper()
	root = t.TempDir()
	goDir = filepath.Join(root, "go")
	if err := os.MkdirAll(filepath.Join(goDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module ciparitytest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, goDir
}

const (
	apicoverOffenderPkg = "package p\n\n// Exported is public but no test names it → uncovered.\nfunc Exported() {}\n"
	apicoverCleanPkg    = "package p\n\nfunc helper() {}\n"
	apicoverBrokenPkg   = "package p\n\nfunc (\n"
)

func writeApicoverFixture(t *testing.T, pkgSrc string) (root, goDir string) {
	t.Helper()
	root, goDir = goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pDir := filepath.Join(goDir, "internal", "p")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "x.go"), []byte(pkgSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/internal/p/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, goDir
}

func apicoverPipelineRunner(goDir string, seen *[]string) sysexec.RunFunc {
	return func(_ context.Context, name string, _ string, args, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if seen != nil {
			*seen = append(*seen, name+" "+strings.Join(args, " "))
		}
		if name == "go" && len(args) > 0 && args[0] == "list" {
			_, _ = io.WriteString(so, filepath.Join(goDir, "internal", "p")+"\n")
			return 0, nil
		}
		return 0, nil
	}
}

func TestApicoverEnforceChanged_NoOps(t *testing.T) {
	if off, err := apicoverEnforceChangedDefault(core.PhaseRequest{Worktree: t.TempDir(), Cycle: 1}); off != nil || err != nil {
		t.Errorf("no module: (%v,%v)", off, err)
	}
	root, _ := goWorktree(t)
	if off, err := apicoverEnforceChangedDefault(core.PhaseRequest{Worktree: root, Cycle: 1}); off != nil || err != nil {
		t.Errorf("no enforce list: (%v,%v)", off, err)
	}
}

func TestApicoverEnforceChanged_Pipeline(t *testing.T) {
	rootClean, goClean := writeApicoverFixture(t, apicoverCleanPkg)
	withFakeRunner(t, apicoverPipelineRunner(goClean, nil))
	if off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: rootClean, Worktree: rootClean, Cycle: 1}); off != nil || err != nil {
		t.Errorf("clean pipeline: (%v,%v), want (nil,nil)", off, err)
	}

	rootBad, goBad := writeApicoverFixture(t, apicoverOffenderPkg)
	withFakeRunner(t, apicoverPipelineRunner(goBad, nil))
	if off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: rootBad, Worktree: rootBad, Cycle: 1}); err != nil || len(off) == 0 {
		t.Errorf("offender pipeline: (%v,%v), want offenders", off, err)
	}
}

func TestCiparity_ApicoverRunsInProcess_NoBinaryCreated(t *testing.T) {
	root, goDir := writeApicoverFixture(t, apicoverOffenderPkg)
	var seen []string
	withFakeRunner(t, apicoverPipelineRunner(goDir, &seen))

	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("gate errored: %v", err)
	}
	if len(off) == 0 {
		t.Fatal("expected offenders for an uncovered export — proves apicover actually ran in-process")
	}
	if _, statErr := os.Stat(filepath.Join(goDir, "bin", "apicover")); !os.IsNotExist(statErr) {
		t.Errorf("bin/apicover must NOT exist after an in-process gate; stat err=%v", statErr)
	}
	for _, c := range seen {
		if strings.Contains(c, "build") && strings.Contains(c, "apicover") {
			t.Errorf("gate forked an apicover build (%q); it must run in-process", c)
		}
	}
}

func TestCiparity_NoExecutableFileCreatedByGate(t *testing.T) {
	root, goDir := writeApicoverFixture(t, apicoverOffenderPkg)
	withFakeRunner(t, apicoverPipelineRunner(goDir, nil))

	before := executableFiles(t, root)
	if _, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1}); err != nil {
		t.Fatalf("gate errored: %v", err)
	}
	after := executableFiles(t, root)

	for p := range after {
		if !before[p] {
			t.Errorf("gate created an executable file %q — the one-binary invariant forbids "+
				"a runtime-built executable in a target-repo cycle", p)
		}
	}
}

func executableFiles(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		if info.Mode().Perm()&0o111 != 0 {
			out[path] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func TestApicoverEnforceChanged_MeasurementError_Fails(t *testing.T) {
	root, goDir := writeApicoverFixture(t, apicoverBrokenPkg)
	withFakeRunner(t, apicoverPipelineRunner(goDir, nil))
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("measurement error must FAIL (offenders,nil), not WARN (nil,err); got err=%v", err)
	}
	if len(off) == 0 {
		t.Fatal("measurement error must produce offenders (FAIL), got a clean pass")
	}
}

func seqRunFunc(t *testing.T, script []struct {
	Code int
	Out  string
}) (sysexec.RunFunc, *int, *[][]string) {
	t.Helper()
	calls := 0
	envs := [][]string{}
	fn := func(_ context.Context, _, _ string, _, env []string, _ io.Reader, so, _ io.Writer) (int, error) {
		if calls >= len(script) {
			t.Fatalf("run func called %d times, script has %d entries", calls+1, len(script))
		}
		step := script[calls]
		calls++
		envs = append(envs, env)
		_, _ = io.WriteString(so, step.Out)
		return step.Code, nil
	}
	return fn, &calls, &envs
}

func killedAtDeadline(fn sysexec.RunFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, in io.Reader, so, se io.Writer) (int, error) {
		<-ctx.Done()
		return fn(ctx, name, dir, args, env, in, so, se)
	}
}

func tierFixture(t *testing.T) core.PhaseRequest {
	t.Helper()
	root, _ := goWorktree(t)
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-3")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/internal/widget/w.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.PhaseRequest{Cycle: 3, ProjectRoot: root, Worktree: root, Workspace: t.TempDir()}
}

func TestIntegrationTier_GreenFirstAttempt_SingleRunCleanEnv(t *testing.T) {
	t.Setenv("EVOLVE_LEAK_CANARY", "1")
	req := tierFixture(t)
	fn, calls, envs := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{0, "ok"}})
	withFakeRunner(t, fn)

	off, err := integrationTierCheckDefault(req)
	if off != nil || err != nil {
		t.Fatalf("green first attempt: (%v, %v), want (nil, nil)", off, err)
	}
	if *calls != 1 {
		t.Fatalf("green path must run exactly once, ran %d times", *calls)
	}
	env := (*envs)[0]
	if env == nil {
		t.Fatal("tier subprocess env must be an explicit scrubbed allowlist, not nil (nil inherits the lane's os.Environ())")
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "PATH=") {
		t.Errorf("scrubbed env must keep PATH; got %d vars", len(env))
	}
	if strings.Contains(joined, "EVOLVE_LEAK_CANARY") {
		t.Error("lane-leaked EVOLVE_* vars must NOT reach the tier subprocess")
	}
}

func TestIntegrationTier_RedThenGreen_FlakeAbsorbedToWarn(t *testing.T) {
	req := tierFixture(t)
	fn, calls, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}})
	withFakeRunner(t, fn)

	off, err := integrationTierCheckDefault(req)
	if off != nil {
		t.Fatalf("green retake must not FAIL the audit; got offenders %v", off)
	}
	if err == nil || !strings.Contains(err.Error(), "flake") {
		t.Fatalf("green retake must surface a WARN-carrying error naming the flake; got %v", err)
	}
	if *calls != 2 {
		t.Fatalf("red first attempt must retake exactly once, ran %d times", *calls)
	}
	logB, rerr := os.ReadFile(filepath.Join(req.Workspace, "integration-tier.log"))
	if rerr != nil {
		t.Fatalf("both attempts must persist to integration-tier.log: %v", rerr)
	}
	log := string(logB)
	if !strings.Contains(log, "attempt 1") || !strings.Contains(log, "attempt 2") || !strings.Contains(log, "TestFlaky") {
		t.Errorf("log must carry both attempts (got %d bytes)", len(log))
	}
}

func TestIntegrationTier_RedThenRed_GenuineOffendersFromRetake(t *testing.T) {
	req := tierFixture(t)
	fn, calls, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n"}})
	withFakeRunner(t, fn)

	off, err := integrationTierCheckDefault(req)
	if err != nil {
		t.Fatalf("double red must FAIL via offenders, not error: %v", err)
	}
	if *calls != 2 {
		t.Fatalf("want exactly 2 attempts, got %d", *calls)
	}
	joined := strings.Join(off, "\n")
	if !strings.Contains(joined, "TestGenuine") {
		t.Errorf("offenders must come from the serialized retake; got %v", off)
	}
	if !strings.Contains(joined, "integration-tier.log") {
		t.Errorf("offenders must carry the log pointer; got %v", off)
	}
}
