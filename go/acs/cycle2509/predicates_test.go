//go:build acs

package cycle2509

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type suiteRun struct {
	once   sync.Once
	stdout string
	code   int
}

var contract suiteRun

func contractSuite(t *testing.T) (string, int) {
	t.Helper()
	contract.once.Do(func() {
		moduleDir := acsassert.GoModuleDir(t)
		contract.stdout, _, contract.code, _ = acsassert.SubprocessOutput(
			"go", "-C", moduleDir, "test", "-count=1", "-v", "-run", "^TestContract$", "./internal/faketmux")
	})
	return contract.stdout, contract.code
}

func tmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func requireCasePasses(t *testing.T, name string) {
	t.Helper()
	out, _ := contractSuite(t)
	if !strings.Contains(out, "--- PASS: TestContract/fake/"+name+" (") {
		t.Errorf("contract case %s did not pass on faketmux:\n%s", name, acsassert.OutputTail(out, 40))
	}
	if tmuxInstalled() {
		if !strings.Contains(out, "--- PASS: TestContract/real/"+name+" (") {
			t.Errorf("contract case %s did not pass on real tmux:\n%s", name, acsassert.OutputTail(out, 40))
		}
		return
	}
	if !strings.Contains(out, "--- SKIP: TestContract/real") {
		t.Errorf("tmux is absent, so the real-tmux half must skip, not vanish:\n%s", acsassert.OutputTail(out, 40))
	}
}

func TestC2509_001_ContractSuiteExitsZeroOnFakeAndRealTmux(t *testing.T) {
	out, code := contractSuite(t)
	if code != 0 {
		t.Fatalf("go test -run TestContract exited %d:\n%s", code, acsassert.OutputTail(out, 60))
	}
	if !strings.Contains(out, "--- PASS: TestContract/fake (") {
		t.Errorf("the fake half of the contract did not run:\n%s", acsassert.OutputTail(out, 40))
	}
}

func TestC2509_002_CapturePaneWithoutPPrintsNothingLikeRealTmux(t *testing.T) {
	requireCasePasses(t, "capture_pane_without_p_prints_nothing")
}

func TestC2509_003_CapturePaneWithPStillPrintsTheScreen(t *testing.T) {
	requireCasePasses(t, "capture_pane_p_prints_the_screen")
}

func TestC2509_004_CarriageReturnOverwritesFromTheLineStart(t *testing.T) {
	requireCasePasses(t, "carriage_return_overwrites_from_the_line_start")
}

func TestC2509_005_CarriageReturnWithLineFeedStaysOneLineBreak(t *testing.T) {
	requireCasePasses(t, "carriage_return_with_a_line_feed_is_one_line_break")
}

func TestC2509_006_ResizeWindowChangesTheReportedSize(t *testing.T) {
	requireCasePasses(t, "resize_window_sets_the_reported_size")
	requireCasePasses(t, "resize_window_accepts_the_size_limits")
}

func TestC2509_007_ResizeWindowRefusesABadSizeAndKeepsTheSize(t *testing.T) {
	requireCasePasses(t, "resize_window_refuses_a_bad_size_and_keeps_the_size")
}

func TestC2509_008_TheBridgeJiggleNudgeFlagsStillResizeByTheAdjustment(t *testing.T) {
	requireCasePasses(t, "resize_window_nudge_flags_move_the_size_by_the_adjustment")
}

func TestC2509_009_MissingSessionStillFailsForCaptureAndResize(t *testing.T) {
	requireCasePasses(t, "capture_pane_of_a_missing_session_fails")
	requireCasePasses(t, "resize_window_of_a_missing_session_fails")
}

func TestC2509_010_TheFakeTmuxCommandLineReachesTheContractBehaviour(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	bin := filepath.Join(t.TempDir(), "evolve-fake-cli")
	if _, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", bin, "./cmd/evolve-fake-cli"); code != 0 {
		t.Fatalf("build of evolve-fake-cli exited %d:\n%s", code, stderr)
	}
	root := t.TempDir()
	tmux := func(args ...string) (string, string, int) {
		out, errOut, code, _ := acsassert.SubprocessOutput(bin, append([]string{"--fake-tmux", root, "-L", "contract"}, args...)...)
		return out, errOut, code
	}
	t.Cleanup(func() { tmux("kill-server") })

	if _, errOut, code := tmux("new-session", "-d", "-s", "a", "-x", "80", "-y", "24", "exec cat"); code != 0 {
		t.Fatalf("new-session exited %d: %s", code, errOut)
	}
	if out, _, code := tmux("capture-pane", "-t", "=a:"); code != 0 || out != "" {
		t.Errorf("capture-pane without -p: exit %d, stdout %q; want exit 0 and no output", code, out)
	}
	if _, errOut, code := tmux("resize-window", "-t", "=a:", "-x", "abc"); code == 0 || errOut == "" {
		t.Errorf("resize-window -x abc: exit %d, stderr %q; want a failure with a message", code, errOut)
	}
	if _, errOut, code := tmux("resize-window", "-t", "=a:", "-x", "100", "-y", "30"); code != 0 {
		t.Fatalf("resize-window -x 100 -y 30 exited %d: %s", code, errOut)
	}
	out, _, _ := tmux("display-message", "-p", "-t", "=a:", "#{window_width}x#{window_height}")
	if got := strings.TrimSpace(out); got != "100x30" {
		t.Errorf("window size after resize = %q, want 100x30", got)
	}
}
