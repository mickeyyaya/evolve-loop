package audit

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// runCIGate maps the runner result to the hook contract: exit 0 → clean; any
// non-zero exit → offenders (FAIL); an exec-start error → fail-open WARN.
func TestRunCIGate_ExitCodeMapping(t *testing.T) {
	root, _ := goWorktree(t)
	req := core.PhaseRequest{Worktree: root}

	withFakeRunner(t, fakeRunFunc(0, "", "", nil))
	if off, err := runCIGate(req, "x", time.Second, "go", "vet"); off != nil || err != nil {
		t.Errorf("exit 0: (%v,%v), want (nil,nil)", off, err)
	}

	withFakeRunner(t, fakeRunFunc(1, "", "bad.go:5: import cycle not allowed", nil))
	if off, err := runCIGate(req, "x", time.Second, "go", "vet"); err != nil || len(off) == 0 {
		t.Errorf("exit 1: (%v,%v), want offenders+nil", off, err)
	}

	withFakeRunner(t, fakeRunFunc(2, "", "", nil)) // non-zero, no output → synthesized line
	if off, err := runCIGate(req, "x", time.Second, "go", "vet"); err != nil || len(off) == 0 {
		t.Errorf("exit 2 no-output: (%v,%v), want a synthesized offender", off, err)
	}

	withFakeRunner(t, fakeRunFunc(-1, "", "", errors.New("executable file not found")))
	if off, err := runCIGate(req, "x", time.Second, "go", "vet"); off != nil || err == nil {
		t.Errorf("start error: (%v,%v), want (nil,err) fail-open", off, err)
	}
}

func TestRunCIGate_NoModule_NoOp(t *testing.T) {
	if off, err := runCIGate(core.PhaseRequest{Worktree: t.TempDir()}, "x", time.Second, "go", "vet"); off != nil || err != nil {
		t.Errorf("no go/ dir: (%v,%v), want (nil,nil)", off, err)
	}
	if off, err := runCIGate(core.PhaseRequest{}, "x", time.Second, "go", "vet"); off != nil || err != nil {
		t.Errorf("empty root: (%v,%v), want (nil,nil)", off, err)
	}
}

func TestOffenderLines(t *testing.T) {
	got := offenderLines("noise\nbad.go:1: import cycle not allowed\nmore\n--- FAIL: X")
	if len(got) != 2 {
		t.Errorf("marker extraction: %v, want 2", got)
	}
	if got := offenderLines("a\nb\nc"); len(got) == 0 {
		t.Errorf("no-marker fallback returned empty")
	}
	long := strings.Repeat("FAIL line\n", 40)
	if got := offenderLines(long); len(got) > 12 {
		t.Errorf("cap: %d lines, want <=12", len(got))
	}
}

func TestCycleTouchedGo(t *testing.T) {
	if cycleTouchedGo(core.PhaseRequest{Worktree: t.TempDir(), Cycle: 1}) {
		t.Error("no build handoff → false")
	}
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/internal/p/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cycleTouchedGo(core.PhaseRequest{ProjectRoot: root, Cycle: 1}) {
		t.Error("handoff with a changed Go package → true")
	}
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
