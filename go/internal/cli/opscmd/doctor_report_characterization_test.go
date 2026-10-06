package opscmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func paneLines(first, last int) string {
	lines := make([]string, 0, last-first+1)
	for i := first; i <= last; i++ {
		lines = append(lines, fmt.Sprintf("pane line %02d", i))
	}
	return strings.Join(lines, "\n")
}

func checkDoctorReport(t *testing.T, name string, code, wantCode int, stdout, wantStdout, stderr, wantStderr string) {
	t.Helper()
	if code != wantCode {
		t.Errorf("%s: exit code = %d, want %d", name, code, wantCode)
	}
	if stdout != wantStdout {
		t.Errorf("%s: stdout =\n%q\nwant\n%q", name, stdout, wantStdout)
	}
	if stderr != wantStderr {
		t.Errorf("%s: stderr =\n%q\nwant\n%q", name, stderr, wantStderr)
	}
}

func TestDoctorCharacterization_LiveReport(t *testing.T) {
	pane := paneLines(1, 20) + "\n"
	cases := []struct {
		name                   string
		asJSON                 bool
		rc                     int
		pattern, scrollback    string
		wantCode               int
		wantStdout, wantStderr string
	}{
		{"ok json", true, bridge.ExitOK, "", pane, 0,
			"{\n  \"driver\": \"claude-tmux\",\n  \"exit_code\": 0,\n  \"healthy\": true\n}\n",
			"[doctor] LIVE OK: claude-tmux answered the probe\n"},
		{"unknown driver", false, bridge.ExitBadFlags, "", "", 10, "",
			"[doctor] live: \"claude-tmux\" is not a known *-tmux driver\n"},
		{"walled json", true, bridge.ExitUnknownPrompt, "rate_limit", pane, 1,
			"{\n  \"driver\": \"claude-tmux\",\n  \"exit_code\": 85,\n  \"healthy\": false,\n  \"pattern\": \"rate_limit\"\n}\n",
			"[doctor] LIVE WALLED: claude-tmux rc=85 pattern=rate_limit\n[doctor] final pane:\n" + paneLines(15, 20) + "\n"},
		{"failed", false, bridge.ExitArtifactTimeout, "", pane, 1, "",
			"[doctor] LIVE FAILED: claude-tmux rc=81\n[doctor] final pane:\n" + paneLines(9, 20) + "\n"},
		{"failed empty pane", false, bridge.ExitArtifactTimeout, "", "", 1, "",
			"[doctor] LIVE FAILED: claude-tmux rc=81\n"},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer
		code := reportDoctorLiveResult(liveProbeTarget{driver: "claude-tmux"}, tc.asJSON, tc.rc, tc.pattern, tc.scrollback, &out, &errb)
		checkDoctorReport(t, tc.name, code, tc.wantCode, out.String(), tc.wantStdout, errb.String(), tc.wantStderr)
	}
}

func TestDoctorCharacterization_BootReport(t *testing.T) {
	pane := paneLines(1, 20) + "\n"
	cases := []struct {
		name                   string
		sandbox, asJSON        bool
		rc                     int
		scrollback             string
		wantCode               int
		wantStdout, wantStderr string
	}{
		{"ok sandbox json", true, true, bridge.ExitOK, pane, 0,
			"{\n  \"driver\": \"claude-tmux\",\n  \"sandbox\": true,\n  \"exit_code\": 0,\n  \"booted\": true\n}\n",
			"[doctor] BOOT OK: claude-tmux REPL booted (sandbox=true)\n"},
		{"unknown driver", false, false, bridge.ExitBadFlags, "", 10, "",
			"[doctor] boot: \"claude-tmux\" is not a known *-tmux driver\n"},
		{"failed", false, false, bridge.ExitREPLBootTimeout, pane, 1, "",
			"[doctor] BOOT FAILED: claude-tmux rc=80 (sandbox=false)\n[doctor] final pane:\n" + paneLines(9, 20) + "\n"},
		{"failed empty pane json", false, true, bridge.ExitREPLBootTimeout, "", 1,
			"{\n  \"driver\": \"claude-tmux\",\n  \"sandbox\": false,\n  \"exit_code\": 80,\n  \"booted\": false\n}\n",
			"[doctor] BOOT FAILED: claude-tmux rc=80 (sandbox=false)\n"},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer
		code := reportDoctorBootResult("claude-tmux", tc.sandbox, tc.asJSON, tc.rc, tc.scrollback, &out, &errb)
		checkDoctorReport(t, tc.name, code, tc.wantCode, out.String(), tc.wantStdout, errb.String(), tc.wantStderr)
	}
}

func TestDoctorCharacterization_UnknownDriverThroughRunDoctor(t *testing.T) {
	cases := []struct {
		args                   []string
		wantStdout, wantStderr string
	}{
		{[]string{"live", "--json", "bogus-driver"},
			"{\n  \"driver\": \"bogus-driver\",\n  \"exit_code\": 10,\n  \"healthy\": false\n}\n",
			"[doctor] live: \"bogus-driver\" is not a known *-tmux driver\n"},
		{[]string{"boot", "--sandbox", "--json", "bogus-driver"},
			"{\n  \"driver\": \"bogus-driver\",\n  \"sandbox\": true,\n  \"exit_code\": 10,\n  \"booted\": false\n}\n",
			"[doctor] boot: \"bogus-driver\" is not a known *-tmux driver\n"},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer
		code := RunDoctor(tc.args, nil, &out, &errb)
		checkDoctorReport(t, strings.Join(tc.args, " "), code, 10, out.String(), tc.wantStdout, errb.String(), tc.wantStderr)
	}
}
