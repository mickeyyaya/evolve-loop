//go:build acs

package cycle2216

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	guardTestName = "TestEveryTmuxTestPackageUsesTheOrphanGuard"
	probeFileName = "zz2216_descriptor_probe_test.go"
	probeTestName = "TestZZ2216DescriptorProbe"
	leakedMarker  = "CYCLE2216-FD3=LEAKED"
	closedMarker  = "CYCLE2216-FD3=CLOSED"
	probeSource   = `package PKGNAME

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestZZ2216DescriptorProbe(t *testing.T) {
	out, _ := exec.Command("sh", "-c", "if { echo x >&3; } 2>/dev/null; then echo CYCLE2216-FD3=LEAKED; else echo CYCLE2216-FD3=CLOSED; fi").CombinedOutput()
	fmt.Print(string(out))
}
`
)

func goModule(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runIn(t *testing.T, dir string, limit time.Duration, env []string, name string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 10 * time.Second
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out.String(), exitErr.ExitCode()
	}
	t.Fatalf("run %s %v in %s: %v", name, args, dir, err)
	return "", -1
}

func runGuardTest(t *testing.T) (string, int) {
	t.Helper()
	return runIn(t, goModule(t), 3*time.Minute, nil,
		"go", "test", "-count=1", "-v", "-run", "^"+guardTestName+"$", "./internal/tmuxtest")
}

func writeTestFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func packageName(t *testing.T, module, pkgRel string) string {
	t.Helper()
	out, code := runIn(t, module, time.Minute, nil, "go", "list", "-f", "{{.Name}}", "./"+pkgRel)
	if code != 0 {
		t.Fatalf("go list ./%s exited %d: %s", pkgRel, code, out)
	}
	return strings.TrimSpace(out)
}

func inheritedDescriptorVerdict(t *testing.T, pkgRel string) string {
	t.Helper()
	module := goModule(t)
	pkgDir := filepath.Join(module, filepath.FromSlash(pkgRel))
	probePath := filepath.Join(pkgDir, probeFileName)
	t.Cleanup(func() { _ = os.Remove(probePath) })
	writeTestFile(t, pkgDir, probeFileName, strings.Replace(probeSource, "PKGNAME", packageName(t, module, pkgRel), 1))

	binary := filepath.Join(t.TempDir(), "pkg.test")
	if out, code := runIn(t, module, 10*time.Minute, nil, "go", "test", "-c", "-o", binary, "./"+pkgRel); code != 0 {
		t.Fatalf("compile the test binary of %s exited %d: %s", pkgRel, code, acsassert.OutputTail(out, 30))
	}

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run", "^"+probeTestName+"$", "-test.v", "-test.count=1")
	cmd.Dir = pkgDir
	cmd.WaitDelay = 10 * time.Second
	cmd.ExtraFiles = []*os.File{write}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	write.Close()
	text := out.String()
	switch {
	case strings.Contains(text, leakedMarker):
		return leakedMarker
	case strings.Contains(text, closedMarker):
		return closedMarker
	}
	t.Fatalf("the probe printed no verdict for %s (run error %v): %s", pkgRel, runErr, acsassert.OutputTail(text, 30))
	return ""
}

func TestC2216_001_EveryPackageThatStartsTmuxRunsTheOrphanGuard(t *testing.T) {
	out, code := runGuardTest(t)
	if code != 0 || !strings.Contains(out, "--- PASS: "+guardTestName) {
		t.Errorf("RED: the guard test did not pass (exit %d): %s", code, acsassert.OutputTail(out, 12))
	}
}

func TestC2216_002_SwarmTestBinaryDoesNotLeakItsOutputPipeToChildren(t *testing.T) {
	if got := inheritedDescriptorVerdict(t, "internal/swarm"); got != closedMarker {
		t.Errorf("RED: a tmux server started by an internal/swarm test inherits the go test output pipe (%s); TestMain must call tmuxtest.Main from an external swarm_test file", got)
	}
}

func TestC2216_003_CmdEvolveTestBinaryDoesNotLeakItsOutputPipeToChildren(t *testing.T) {
	if got := inheritedDescriptorVerdict(t, "cmd/evolve"); got != closedMarker {
		t.Errorf("RED: a tmux server started by a cmd/evolve test inherits the go test output pipe (%s); its TestMain must run the tests through tmuxtest.Main", got)
	}
}

func TestC2216_004_GuardNamesAnUnguardedPackageAndSparesGuardedAndLiteralOnes(t *testing.T) {
	internalDir := filepath.Join(goModule(t), "internal")
	unguarded := filepath.Join(internalDir, "zz2216unguarded")
	guarded := filepath.Join(internalDir, "zz2216guarded")
	literal := filepath.Join(internalDir, "zz2216literal")
	for _, dir := range []string{unguarded, guarded, literal} {
		dir := dir
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
	}
	writeTestFile(t, unguarded, "start_test.go", `package zz2216unguarded

import (
	"os/exec"
	"testing"
)

func TestStart(t *testing.T) {
	_ = exec.Command("tmux", "new-session", "-d").Run()
}
`)
	writeTestFile(t, guarded, "start_test.go", `package zz2216guarded

import (
	"os/exec"
	"testing"
)

func TestStart(t *testing.T) {
	_ = exec.Command("tmux", "new-session", "-d").Run()
}
`)
	writeTestFile(t, guarded, "main_test.go", `package zz2216guarded_test

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestMain(m *testing.M) {
	os.Exit(tmuxtest.Main(m))
}
`)
	writeTestFile(t, literal, "fixture_test.go", `package zz2216literal

import (
	"fmt"
	"testing"
)

func TestFixture(t *testing.T) {
	rows := []string{"tmux", "sleep"}
	fmt.Sprint(rows, "exec.Command(\"tmux\")")
}
`)

	out, code := runGuardTest(t)
	if code == 0 {
		t.Fatalf("RED: the guard passed although internal/zz2216unguarded starts tmux with no tmuxtest.Main: %s", acsassert.OutputTail(out, 12))
	}
	if !strings.Contains(out, "internal/zz2216unguarded") {
		t.Errorf("RED: the guard failed without naming internal/zz2216unguarded: %s", acsassert.OutputTail(out, 12))
	}
	if strings.Contains(out, "internal/zz2216guarded") {
		t.Errorf("the guard flagged a package whose TestMain calls tmuxtest.Main: %s", acsassert.OutputTail(out, 12))
	}
	if strings.Contains(out, "internal/zz2216literal") {
		t.Errorf("the guard flagged a package that holds tmux only in string literals: %s", acsassert.OutputTail(out, 12))
	}
}

func TestC2216_005_ControlGuardedPackageReportsItsPipeClosed(t *testing.T) {
	if got := inheritedDescriptorVerdict(t, "internal/looppreflight"); got != closedMarker {
		t.Errorf("the probe reports %s for internal/looppreflight, which already runs tmuxtest.Main: the probe is not a valid detector", got)
	}
}
