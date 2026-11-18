package faketmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type contractTmux interface {
	run(args ...string) (stdout, stderr string, code int)
}

type realTmux struct {
	t      *testing.T
	socket string
}

var realSockets atomic.Int64

func newRealTmux(t *testing.T) contractTmux {
	t.Helper()
	r := realTmux{t: t, socket: fmt.Sprintf("evolve-contract-%d-%d", os.Getpid(), realSockets.Add(1))}
	t.Cleanup(func() { _, _, _ = r.run("kill-server") })
	return r
}

func newFakeContractTmux(t *testing.T) contractTmux {
	t.Helper()
	return newFakeTmux(t)
}

func (r realTmux) run(args ...string) (string, string, int) {
	r.t.Helper()
	cmd := sysexec.Command(context.Background(), "tmux", append([]string{"-L", r.socket, "-f", "/dev/null"}, args...)...)
	cmd.Env = contractEnv(os.Environ())
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), errOut.String(), 0
	case errors.As(err, &exit):
		return out.String(), errOut.String(), exit.ExitCode()
	}
	r.t.Fatalf("tmux %q: %v", args, err)
	return "", "", -1
}

func contractEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); name != "TMUX" && name != "TMUX_PANE" {
			out = append(out, kv)
		}
	}
	return out
}

func contractMust(t *testing.T, tm contractTmux, args ...string) string {
	t.Helper()
	out, errOut, code := tm.run(args...)
	if code != 0 {
		t.Fatalf("tmux %q: exit %d: %s", args, code, errOut)
	}
	return out
}

func contractTarget(session string) string { return "=" + session + ":" }

func contractPane(t *testing.T, tm contractTmux, session, script string) {
	t.Helper()
	contractMust(t, tm, "new-session", "-d", "-s", session, "-x", "80", "-y", "24", "-c", t.TempDir(), script)
}

func contractAwait(t *testing.T, tm contractTmux, session, want string) {
	t.Helper()
	var screen string
	for range 1000 {
		screen = contractMust(t, tm, "capture-pane", "-p", "-S", "-", "-t", contractTarget(session))
		if strings.Contains(screen, want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the screen of %s never showed %q:\n%s", session, want, screen)
}

func contractRows(screen string) []string {
	rows := strings.Split(screen, "\n")
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	return rows
}

func contractScreen(t *testing.T, tm contractTmux, session string) []string {
	t.Helper()
	return contractRows(contractMust(t, tm, "capture-pane", "-p", "-S", "-", "-t", contractTarget(session)))
}

func contractSize(t *testing.T, tm contractTmux, session string) string {
	t.Helper()
	return strings.TrimSpace(contractMust(t, tm, "display-message", "-p", "-t", contractTarget(session), "#{window_width}x#{window_height}"))
}

func wantRows(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("screen rows = %q, want %q", got, want)
	}
}

func wantSize(t *testing.T, tm contractTmux, session, want string) {
	t.Helper()
	if got := contractSize(t, tm, session); got != want {
		t.Errorf("window size = %q, want %q", got, want)
	}
}

type contractCase struct {
	name string
	run  func(t *testing.T, tm contractTmux)
}

var contractCases = []contractCase{
	{"capture_pane_p_prints_the_screen", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `printf 'alpha\nbeta\ndone\n'; exec cat`)
		contractAwait(t, tm, "a", "done")
		wantRows(t, contractScreen(t, tm, "a"), "alpha", "beta", "done")
	}},
	{"capture_pane_without_p_prints_nothing", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `printf 'alpha\nbeta\ndone\n'; exec cat`)
		contractAwait(t, tm, "a", "done")
		for _, args := range [][]string{
			{"capture-pane", "-t", contractTarget("a")},
			{"capture-pane", "-b", "contractbuf", "-t", contractTarget("a")},
			{"capture-pane", "-S", "-", "-t", contractTarget("a")},
		} {
			out, errOut, code := tm.run(args...)
			if code != 0 || out != "" || errOut != "" {
				t.Errorf("tmux %q: exit %d, stdout %q, stderr %q; want exit 0 and no output", args, code, out, errOut)
			}
		}
	}},
	{"capture_pane_of_a_missing_session_fails", func(t *testing.T, tm contractTmux) {
		out, errOut, code := tm.run("capture-pane", "-p", "-t", contractTarget("nosuch"))
		if code == 0 || out != "" || errOut == "" {
			t.Errorf("exit %d, stdout %q, stderr %q; want a failure with a message and no output", code, out, errOut)
		}
	}},
	{"carriage_return_overwrites_from_the_line_start", func(t *testing.T, tm contractTmux) {
		for i, c := range []struct {
			printf string
			want   string
		}{
			{`aaaa\rbb\n`, "bbaa"},
			{`abc\r\r\rZ\n`, "Zbc"},
			{`\rX\n`, "X"},
		} {
			session := fmt.Sprintf("cr%d", i)
			contractPane(t, tm, session, fmt.Sprintf(`printf '%s'; printf 'done\n'; exec cat`, c.printf))
			contractAwait(t, tm, session, "done")
			t.Run(c.printf, func(t *testing.T) { wantRows(t, contractScreen(t, tm, session), c.want, "done") })
		}
	}},
	{"carriage_return_with_a_line_feed_is_one_line_break", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `printf 'hello\r\nworld\r\n'; printf 'done\n'; exec cat`)
		contractAwait(t, tm, "a", "done")
		wantRows(t, contractScreen(t, tm, "a"), "hello", "world", "done")
	}},
	{"resize_window_sets_the_reported_size", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `exec cat`)
		wantSize(t, tm, "a", "80x24")
		for _, step := range []struct {
			args []string
			want string
		}{
			{[]string{"-x", "100", "-y", "30"}, "100x30"},
			{[]string{"-x", "120"}, "120x30"},
			{[]string{"-y", "9"}, "120x9"},
		} {
			contractMust(t, tm, append([]string{"resize-window", "-t", contractTarget("a")}, step.args...)...)
			wantSize(t, tm, "a", step.want)
		}
	}},
	{"resize_window_accepts_the_size_limits", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `exec cat`)
		for _, size := range []string{"1", "10000"} {
			contractMust(t, tm, "resize-window", "-t", contractTarget("a"), "-x", size, "-y", size)
			wantSize(t, tm, "a", size+"x"+size)
		}
	}},
	{"resize_window_refuses_a_bad_size_and_keeps_the_size", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `exec cat`)
		for _, bad := range [][]string{
			{"-x", "abc"}, {"-x", "0"}, {"-x", "-5"}, {"-x", "5x"}, {"-x", "10001"},
			{"-y", "abc"}, {"-y", "0"}, {"-y", "10001"},
			{"-x", "100", "-y", "abc"},
		} {
			out, errOut, code := tm.run(append([]string{"resize-window", "-t", contractTarget("a")}, bad...)...)
			if code == 0 || out != "" || errOut == "" {
				t.Errorf("resize-window %q: exit %d, stdout %q, stderr %q; want a failure with a message", bad, code, out, errOut)
			}
			wantSize(t, tm, "a", "80x24")
		}
	}},
	{"resize_window_nudge_flags_move_the_size_by_the_adjustment", func(t *testing.T, tm contractTmux) {
		contractPane(t, tm, "a", `exec cat`)
		for _, step := range []struct {
			args []string
			want string
		}{
			{[]string{"-L", "1"}, "79x24"},
			{[]string{"-R", "1"}, "80x24"},
			{[]string{"-U", "2"}, "80x22"},
			{[]string{"-D", "2"}, "80x24"},
		} {
			contractMust(t, tm, append([]string{"resize-window", "-t", contractTarget("a")}, step.args...)...)
			wantSize(t, tm, "a", step.want)
		}
		if _, errOut, code := tm.run("resize-window", "-t", contractTarget("a"), "-L", "abc"); code == 0 || errOut == "" {
			t.Errorf("resize-window -L abc: exit %d, stderr %q; want a failure with a message", code, errOut)
		}
		wantSize(t, tm, "a", "80x24")
	}},
	{"resize_window_of_a_missing_session_fails", func(t *testing.T, tm contractTmux) {
		_, errOut, code := tm.run("resize-window", "-t", contractTarget("nosuch"), "-x", "10", "-y", "10")
		if code == 0 || errOut == "" {
			t.Errorf("exit %d, stderr %q; want a failure with a message", code, errOut)
		}
	}},
}

func runContract(t *testing.T, newTmux func(*testing.T) contractTmux) {
	for _, c := range contractCases {
		t.Run(c.name, func(t *testing.T) { c.run(t, newTmux(t)) })
	}
}

func TestContract(t *testing.T) {
	t.Run("fake", func(t *testing.T) { runContract(t, newFakeContractTmux) })
	t.Run("real", func(t *testing.T) {
		if _, err := exec.LookPath("tmux"); err != nil {
			t.Skip("tmux is not installed: the contract runs on the fake only")
		}
		runContract(t, newRealTmux)
	})
}
