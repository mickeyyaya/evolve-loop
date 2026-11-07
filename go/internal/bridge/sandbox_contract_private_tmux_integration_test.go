//go:build integration && darwin

package bridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const privateTmuxRowAction = "start a private tmux server (-L) and run a window on its own pty"

const lateClientDelaySeconds = "0.4"

const loadedClientShim = `#!/bin/sh
if [ -z "${TMUX:-}" ]; then sleep ` + lateClientDelaySeconds + `; fi
exec %s "$@"
`

const absentServerShim = `#!/bin/sh
if [ -z "${TMUX:-}" ]; then
	for a in "$@"; do
		if [ "$a" = "new-session" ]; then exit 0; fi
	done
fi
exec %s "$@"
`

var sleepWord = regexp.MustCompile(`\bsleep\b`)

func privateTmuxRow(t *testing.T) contractRow {
	t.Helper()
	for _, row := range agentContract {
		if row.action == privateTmuxRowAction {
			return row
		}
	}
	t.Fatalf("the contract has no row %q", privateTmuxRowAction)
	return contractRow{}
}

func tmuxShimDir(t *testing.T, body string) string {
	t.Helper()
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skipf("tmux is not installed: %v", err)
	}
	dir := contractTempDir(t, "", "sbc-shim")
	shim := fmt.Sprintf(body, realTmux)
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runRowWithoutSandbox(t *testing.T, row contractRow, shimDir string) (contractHost, string, error) {
	t.Helper()
	h := contractHost{scratch: contractTempDir(t, "/tmp", "sbc-agent")}
	defer reapPrivateTmuxServers(h.scratch)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", row.script(h))
	cmd.WaitDelay = contractAgentWaitDelay
	cmd.Env = paneEnv(os.Environ())
	if shimDir != "" {
		cmd.Env = append(cmd.Env, "PATH="+shimDir+":"+os.Getenv("PATH"))
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("inconclusive: the row timed out: %q", out)
	}
	return h, strings.TrimSpace(string(out)), err
}

func TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer(t *testing.T) {
	t.Parallel()
	row := privateTmuxRow(t)
	shimDir := tmuxShimDir(t, loadedClientShim)
	for i := 1; i <= 3; i++ {
		h, out, err := runRowWithoutSandbox(t, row, shimDir)
		if failure := contractFailure(row.want, out, err); failure != nil {
			t.Fatalf("run %d under a host that delays each tmux client: %v", i, failure)
		}
		row.check(t, h, out)
	}
}

func TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep(t *testing.T) {
	t.Parallel()
	script := privateTmuxRow(t).script(contractHost{scratch: "/tmp/sbc-agent-fixture"})
	if sleepWord.MatchString(script) {
		t.Fatalf("the row must wait for its server with a readiness probe, not a sleep: %q", script)
	}
}

func TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady(t *testing.T) {
	t.Parallel()
	row := privateTmuxRow(t)
	shimDir := tmuxShimDir(t, absentServerShim)
	_, out, err := runRowWithoutSandbox(t, row, shimDir)
	if err == nil {
		t.Fatalf("a row whose private server never started must not pass, got output %q", out)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "private tmux server") || !strings.Contains(lower, "ready") {
		t.Fatalf("the failure must say that the private tmux server never became ready, got %q", out)
	}
}

func TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty(t *testing.T) {
	t.Parallel()
	row := privateTmuxRow(t)
	h, out, err := runRowWithoutSandbox(t, row, "")
	if failure := contractFailure(row.want, out, err); failure != nil {
		t.Fatal(failure)
	}
	row.check(t, h, out)
}
