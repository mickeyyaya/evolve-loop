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

func writeApicoverFixture(t *testing.T) (root, goDir string) {
	t.Helper()
	root, goDir = goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
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

// apicover pipeline: go list emits the touched dir; the final apicover -enforce
// exit code decides clean (0) vs offenders (non-zero).
func apicoverPipelineRunner(goDir string, enforceCode int, enforceOut string) sysexec.RunFunc {
	return func(_ context.Context, name string, _ string, args, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if name == "go" && len(args) > 0 && args[0] == "list" {
			_, _ = io.WriteString(so, filepath.Join(goDir, "internal", "p")+"\n")
			return 0, nil
		}
		if strings.HasSuffix(name, "apicover") {
			_, _ = io.WriteString(se, enforceOut)
			return enforceCode, nil
		}
		return 0, nil // go build / go test / go tool cover succeed
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

func TestApicoverEnforceChanged_Pipeline(t *testing.T) {
	root, goDir := writeApicoverFixture(t)
	req := core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1}

	withFakeRunner(t, apicoverPipelineRunner(goDir, 0, ""))
	if off, err := apicoverEnforceChangedDefault(req); off != nil || err != nil {
		t.Errorf("clean pipeline: (%v,%v), want (nil,nil)", off, err)
	}

	withFakeRunner(t, apicoverPipelineRunner(goDir, 1, "UNCOVERED (no test names it): 1\n  type Foo"))
	if off, err := apicoverEnforceChangedDefault(req); err != nil || len(off) == 0 {
		t.Errorf("offender pipeline: (%v,%v), want offenders", off, err)
	}
}
