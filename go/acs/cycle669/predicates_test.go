//go:build acs

package cycle669

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamedTest(t *testing.T, extra []string, pattern, pkg string) (string, int) {
	t.Helper()
	args := append([]string{"test", "-C", goDir(t), "-count=1"}, extra...)
	args = append(args, "-run", pattern, "-v", pkg)
	out, errOut, code, _ := acsassert.SubprocessOutput("go", args...)
	return out + "\n" + errOut, code
}

func assertPassed(t *testing.T, out string, code int, names ...string) {
	t.Helper()
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("expected `--- PASS: %s` (exit=%d) — the behavioural test is missing, renamed, or failing.\nOutput:\n%s",
				n, code, tail(out))
			return
		}
	}
	if code != 0 {
		t.Errorf("named tests present but the package exited %d (a sibling test failed or the package does not build).\nOutput:\n%s",
			code, tail(out))
	}
}

func tail(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

func TestC669_001_TimeoutArmNeverClosesSink(t *testing.T) {
	out, code := runNamedTest(t, []string{"-race"},
		"TestCoreAdapter_NoSinkCloseRaceOnTimeout",
		"./internal/adapters/observer/")
	assertPassed(t, out, code, "TestCoreAdapter_NoSinkCloseRaceOnTimeout")
}

func TestC669_002_DoneArmClosesExactlyOnce(t *testing.T) {
	out, code := runNamedTest(t, []string{"-race"},
		"TestCoreAdapter_SinkClosedOnNormalDone|TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe",
		"./internal/adapters/observer/")
	assertPassed(t, out, code,
		"TestCoreAdapter_SinkClosedOnNormalDone",
		"TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe")
}

func TestC669_003_ObserverPackageVetAndRaceClean(t *testing.T) {
	vout, verr, vcode, _ := acsassert.SubprocessOutput("go", "vet", "-C", goDir(t),
		"./internal/adapters/observer/")
	if vcode != 0 {
		t.Errorf("AC-3: `go vet ./internal/adapters/observer/` exited %d.\nOutput:\n%s",
			vcode, tail(vout+"\n"+verr))
	}
	rout, rerr, rcode, _ := acsassert.SubprocessOutput("go", "test", "-C", goDir(t),
		"-race", "-count=1", "./internal/adapters/observer/")
	if rcode != 0 {
		t.Errorf("AC-3: package must be race-clean; `go test -race ./internal/adapters/observer/` exited %d.\nOutput:\n%s",
			rcode, tail(rout+"\n"+rerr))
	}
}
