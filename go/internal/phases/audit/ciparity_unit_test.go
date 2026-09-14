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

// fakeRunFunc returns a sysexec.RunFunc that emits fixed stdout/stderr and a
// fixed (code, err) — lets the CI-parity gate logic be exercised without forking.
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

// apicover enforced-package sources: the exported-symbol content decides whether
// the IN-PROCESS apicover.Run reports an offender (an exported func no test names
// → uncovered) or is clean (no exports at all).
const (
	apicoverOffenderPkg = "package p\n\n// Exported is public but no test names it → uncovered.\nfunc Exported() {}\n"
	apicoverCleanPkg    = "package p\n\nfunc helper() {}\n"
	// apicoverBrokenPkg has a syntax error, so apicover.Run's Enumerate returns a
	// measurement error (code 2) — the gate must FAIL, not silently WARN.
	apicoverBrokenPkg = "package p\n\nfunc (\n"
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

// apicoverPipelineRunner fakes only the toolchain subprocesses the gate still
// forks (go test -coverprofile, go tool cover -func, go list). apicover itself
// now runs IN-PROCESS (apicover.Run), so there is no apicover subprocess to fake
// and no `go build -o bin/apicover` — it records every invocation into seen (when
// non-nil) so a test can assert neither a build nor an apicover fork happens.
func apicoverPipelineRunner(goDir string, seen *[]string) sysexec.RunFunc {
	return func(_ context.Context, name string, _ string, args, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if seen != nil {
			*seen = append(*seen, name+" "+strings.Join(args, " "))
		}
		if name == "go" && len(args) > 0 && args[0] == "list" {
			_, _ = io.WriteString(so, filepath.Join(goDir, "internal", "p")+"\n")
			return 0, nil
		}
		return 0, nil // go test (coverprofile) + go tool cover -func succeed (no-op)
	}
}

func TestApicoverEnforceChanged_NoOps(t *testing.T) {
	if off, err := apicoverEnforceChangedDefault(core.PhaseRequest{Worktree: t.TempDir(), Cycle: 1}); off != nil || err != nil {
		t.Errorf("no module: (%v,%v)", off, err)
	}
	root, _ := goWorktree(t) // go/ but no .apicover-enforce / no handoff
	if off, err := apicoverEnforceChangedDefault(core.PhaseRequest{Worktree: root, Cycle: 1}); off != nil || err != nil {
		t.Errorf("no enforce list: (%v,%v)", off, err)
	}
}

// TestApicoverEnforceChanged_Pipeline drives the gate end-to-end with apicover
// running IN-PROCESS: a clean enforced package (no exports) passes; one with an
// exported symbol no test names yields offenders.
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

// TestCiparity_ApicoverRunsInProcess_NoBinaryCreated pins one-binary S1: a cycle
// touching an enforced package runs the API-coverage gate to completion WITHOUT
// forking a `go build -o bin/apicover` and WITHOUT leaving a bin/apicover artifact
// on the worktree — apicover.Run is folded into the evolve binary.
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
	// No apicover binary was built or left behind.
	if _, statErr := os.Stat(filepath.Join(goDir, "bin", "apicover")); !os.IsNotExist(statErr) {
		t.Errorf("bin/apicover must NOT exist after an in-process gate; stat err=%v", statErr)
	}
	// No forked command builds apicover.
	for _, c := range seen {
		if strings.Contains(c, "build") && strings.Contains(c, "apicover") {
			t.Errorf("gate forked an apicover build (%q); it must run in-process", c)
		}
	}
}

// TestCiparity_NoExecutableFileCreatedByGate is the one-binary S3 runtime
// guarantee (complementing the acs/regression/norebuild source-scan): running
// the API-coverage gate over an enforced package must not create ANY executable
// file anywhere in the worktree — not just the historic bin/apicover. It walks
// the whole tree before and after and asserts the executable-file set is
// unchanged.
//
// SCOPE (honest): the subprocess seam (runCmd) is replaced with a no-op fake
// here, so this proves the IN-PROCESS work — apicover.Run plus any direct
// os.WriteFile/os.Chmod the gate itself does — drops no executable. It does NOT
// exercise a real forked `go build` (that path is faked out); catching a NEW
// forked-build site is the acs/regression/norebuild source-scan's job. The two
// guards are complementary: source-scan catches the site at ship/CI time, this
// catches an in-process binary drop at runtime.
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

// executableFiles returns the set of regular files under root whose owner-exec
// bit is set (a first-party built binary would be one).
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
		if info.Mode().Perm()&0o111 != 0 { // any exec bit (owner/group/other)
			out[path] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// TestApicoverEnforceChanged_MeasurementError_Fails: when apicover.Run itself
// errors (a touched package won't parse → code 2), the gate must FAIL
// (offenders, nil) — the same bucket the old bin/apicover exit-2 fell into — NOT
// silently downgrade to a WARN (nil, err). In-process there is no exec-start
// failure mode, so any measurement error is a real gate failure (cf. the
// underivable-changed-set hard-FAIL, cycle-581 D1).
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

// --- integration-tier flake-absorb (post-v22.4.2 false-RED class) ----------
//
// Post-release audit of the verification batch proved 3 audit-FAILs (cycles
// 943/950/955) were tier false-REDs: every named test PASSES in isolation in
// the failed cycles' own preserved worktrees. Two mechanisms, two remedies:
//   - env-inheritance: the gate subprocess inherited the lane's full
//     environment (sysexec nil-env → os.Environ()) while CI runs clean — a
//     CI-parity bug; the tier now ALWAYS runs with a scrubbed allowlist env;
//   - fleet contention: -race integration tests starve under live lanes; on
//     red the tier retakes ONCE under a cross-lane exclusive lock — a green
//     retake is a flake (absorbed → WARN), a red retake is genuine (FAIL).

// seqRunFunc scripts one (code, stdout) per successive call and records the
// env each call received.
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

// killedAtDeadline makes a scripted runner faithful to a process the ctx
// deadline KILLED: it returns only once ctx is done — a real SIGKILL follows
// the deadline, never precedes it — so the 1 ns budgets below reach the
// deadline arms deterministically. Without it context.WithTimeout(…, 1ns) may
// arm a timer instead of expiring synchronously and an instant fake is
// observed before ctx.Err() is set (the leaf saw 2/40 such runs, 2026-09-14).
func killedAtDeadline(fn sysexec.RunFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, in io.Reader, so, se io.Writer) (int, error) {
		<-ctx.Done()
		return fn(ctx, name, dir, args, env, in, so, se)
	}
}

// tierFixture builds a root with a go module, a build handoff naming a
// NON-env-exclusive package, and a workspace dir — everything
// integrationTierCheckDefault needs to reach the run seam.
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

// TestIntegrationTier_GreenFirstAttempt_SingleRunCleanEnv — the fast path is
// unchanged (exactly one run) AND that one run already gets the scrubbed env:
// PATH survives, a lane-leaked EVOLVE_* canary does not (CI parity — CI's env
// is clean, so inheriting the lane's environment was a parity bug even when
// nothing flaked).
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

// TestIntegrationTier_RedThenGreen_FlakeAbsorbedToWarn — a red first attempt
// retakes once (serialized) and a GREEN retake is absorbed as a contention
// flake: (nil, error) so applyCIGate surfaces a WARN, never a false FAIL. Both
// attempts persist to integration-tier.log for the retro.
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

// TestIntegrationTier_RedThenRed_GenuineOffendersFromRetake — a red retake is a
// genuine failure: FAIL with the RETAKE's offender lines (the serialized,
// clean-env attempt is the truthful one) plus the log pointer.
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
