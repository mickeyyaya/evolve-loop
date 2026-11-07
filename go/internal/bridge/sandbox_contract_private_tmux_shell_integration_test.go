//go:build integration && darwin

package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sessionShellThatExitsAtOnce = `#!/bin/sh
if [ "${1:-}" = "-c" ]; then
	shift
	exec /bin/sh -c "$@"
fi
exit 0
`

func dyingSessionShellShim(t *testing.T) string {
	t.Helper()
	shell := filepath.Join(contractTempDir(t, "", "sbc-shell"), "dying-shell")
	if err := os.WriteFile(shell, []byte(sessionShellThatExitsAtOnce), 0o755); err != nil {
		t.Fatal(err)
	}
	return tmuxShimDir(t, "#!/bin/sh\nif [ -z \"${TMUX:-}\" ]; then\n\tsleep "+lateClientDelaySeconds+"\n\tSHELL="+shell+"\n\texport SHELL\nfi\nexec %s \"$@\"\n")
}

func TestPrivateTmuxRow_TheServerOutlivesTheSessionShellThatEndsBeforeTheWindowStarts(t *testing.T) {
	t.Parallel()
	row := privateTmuxRow(t)
	shimDir := dyingSessionShellShim(t)
	for i := 1; i <= 3; i++ {
		h, out, err := runRowWithoutSandbox(t, row, shimDir)
		if failure := contractFailure(row.want, out, err); failure != nil {
			t.Fatalf("run %d with a session shell that ends at once: %v", i, failure)
		}
		row.check(t, h, out)
	}
}

func TestPrivateTmuxRow_AServerThatNeverStartsIsReportedWithTheCauseOfTmux(t *testing.T) {
	t.Parallel()
	row := privateTmuxRow(t)
	_, out, err := runRowWithoutSandbox(t, row, tmuxShimDir(t, absentServerShim))
	if err == nil {
		t.Fatalf("a row whose private server never started must not pass, got output %q", out)
	}
	if !reportsCauseBesidesNotReady(out) {
		t.Fatalf("the failure must carry the own words of tmux for the missing server, not only that it was not ready, got %q", out)
	}
}

func reportsCauseBesidesNotReady(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" && !strings.Contains(line, "never became ready") {
			return true
		}
	}
	return false
}
