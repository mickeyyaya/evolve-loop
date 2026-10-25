package faketmux

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeTmux struct {
	t    *testing.T
	root string
}

func newFakeTmux(t *testing.T) fakeTmux {
	t.Helper()
	f := fakeTmux{t: t, root: t.TempDir()}
	t.Cleanup(func() { _, _, _ = f.run("kill-server") })
	return f
}

func (f fakeTmux) run(args ...string) (string, string, int) {
	var out, errOut bytes.Buffer
	code := Run(f.root, append([]string{"-L", "sock"}, args...), &out, &errOut)
	return out.String(), errOut.String(), code
}

func (f fakeTmux) must(args ...string) string {
	f.t.Helper()
	out, errOut, code := f.run(args...)
	if code != 0 {
		f.t.Fatalf("faketmux %q: exit %d: %s", args, code, errOut)
	}
	return out
}

func (f fakeTmux) refuses(want string, args ...string) {
	f.t.Helper()
	_, errOut, code := f.run(args...)
	if code != 1 || !strings.Contains(errOut, want) {
		f.t.Fatalf("faketmux %q: exit %d stderr %q; want exit 1 naming %q", args, code, errOut, want)
	}
}

func (f fakeTmux) server() *server {
	return &server{dir: filepath.Join(f.root, "sock"), stdout: &bytes.Buffer{}, procs: listProcesses}
}

func (f fakeTmux) awaitScreen(session, want string) string {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		screen := f.must("capture-pane", "-p", "-S", "-", "-t", "="+session+":")
		if strings.Contains(screen, want) {
			return screen
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("the screen of %s never showed %q:\n%s", session, want, screen)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRun_APaneRunsTheKeysItIsSentAndShowsTheirOutput(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "work", "-x", "80", "-y", "5", "-c", t.TempDir())
	f.must("has-session", "-t", "=work:")

	f.must("send-keys", "-t", "=work:", "echo $((40+2)) $TERM", "Enter")

	screen := f.awaitScreen("work", "42 screen")
	if !strings.Contains(screen, "echo $((40+2)) $TERM\n") {
		t.Errorf("the pane must echo the typed keys like a terminal; screen:\n%s", screen)
	}
}

func TestRun_APastedBufferReachesTheProgramAndTheBufferIsDeletedWithD(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "p", "/bin/sh", "-c", "read a; read b; echo got:$a:$b; sleep 30")
	buf := filepath.Join(t.TempDir(), "prompt")
	if err := os.WriteFile(buf, []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.must("load-buffer", "-b", "p", buf)

	f.must("paste-buffer", "-b", "p", "-t", "=p:", "-p")
	f.must("paste-buffer", "-b", "p", "-t", "=p:", "-d", "-p", "-r")

	f.awaitScreen("p", "got:first:second")
	f.refuses("no buffer p", "new-session", "-d", "-s", "q", ";", "paste-buffer", "-b", "p", "-t", "=q:")
}

func TestRun_ASessionOptionIsSetAndShownAndAnUnsetOneShowsNothing(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "o", "sleep 30", ";", "set-option", "-t", "=o:", "@evolve_env", "abc123")

	if got := f.must("show-options", "-qv", "-t", "=o:", "@evolve_env"); got != "abc123\n" {
		t.Errorf("show-options = %q, want the value that set-option stored", got)
	}
	if got := f.must("show-options", "-qv", "-t", "=o:", "@other"); got != "" {
		t.Errorf("an unset option shows %q, want nothing", got)
	}
	f.must("set-option", "-g", "default-shell", "/bin/sh")
}

func TestRun_KillSessionEndsThePaneAndTheSessionIsGone(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "k", "sleep 30")
	pid, err := strconv.Atoi(strings.TrimSpace(f.must("list-panes", "-s", "-t", "=k:", "-F", "#{pane_pid}")))
	if err != nil {
		t.Fatal(err)
	}

	f.must("kill-session", "-t", "=k:")

	f.refuses("can't find session: k", "has-session", "-t", "=k:")
	awaitGone(t, pid)
}

func awaitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("pane process %d still runs", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRun_APaneWhoseProgramExitedIsNoLongerASession(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "e", "sleep 0.2")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, _, code := f.run("has-session", "-t", "=e:"); code != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the session outlived its pane program")
		}
		time.Sleep(20 * time.Millisecond)
	}

	f.must("new-session", "-d", "-s", "e", "sleep 30")
}

func TestRun_KillServerEndsEverySessionAndListSessionsThenFinds_NoServer(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("start-server", ";", "new-session", "-d", "-s", "a", "sleep 30", ";", "new-session", "-d", "-s", "b", "sleep 30")
	if got := f.must("list-sessions", "-F", "#{session_name} #{unknown}"); got != "a \nb \n" {
		t.Errorf("list-sessions = %q, want both sessions and an empty unknown variable", got)
	}
	if got := f.must("list-sessions"); got != "a\nb\n" {
		t.Errorf("list-sessions with no format = %q, want the session names", got)
	}

	f.must("kill-server")

	f.refuses("no server running", "list-sessions")
	f.must("kill-server")
}

func TestRun_DisplayMessageNamesTheForegroundProgramOfThePane(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "d", "-y", "3")
	if got := f.must("display-message", "-p", "-t", "=d:", "#{pane_current_command}"); got != "sh\n" {
		t.Errorf("an idle pane runs %q, want the shell sh", got)
	}
	f.must("send-keys", "-t", "=d:", "-l", "exec sleep 30", ";", "send-keys", "-t", "=d:", "C-m")
	deadline := time.Now().Add(10 * time.Second)
	for f.must("display-message", "-p", "-t", "=d:", "#{pane_current_command}") != "sleep\n" {
		if time.Now().After(deadline) {
			t.Fatal("the pane never showed sleep as its foreground program")
		}
		time.Sleep(20 * time.Millisecond)
	}
	activity, err := strconv.ParseInt(strings.TrimSpace(f.must("list-sessions", "-F", "#{window_activity}")), 10, 64)
	if err != nil || activity < time.Now().Add(-time.Hour).Unix() {
		t.Errorf("window_activity = %d (%v), want the time of the last screen write", activity, err)
	}
	f.must("resize-window", "-t", "=d:", "-L", "1")
}

func TestRun_CapturePaneShowsTheVisibleRowsOrTheScrollbackToo(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "c", "-y", "2", "/bin/sh", "-c", "printf 'l1\\r\\nl2\\nl3\\nl4\\r\\n'; sleep 30")
	f.awaitScreen("c", "l4")

	if got := f.must("capture-pane", "-p", "-t", "=c:"); got != "l3\nl4\n" {
		t.Errorf("capture-pane = %q, want the 2 visible rows", got)
	}
	if got := f.must("capture-pane", "-p", "-S", "-1", "-t", "=c:"); got != "l2\nl3\nl4\n" {
		t.Errorf("capture-pane -S -1 = %q, want one scrollback row more", got)
	}
	if got := f.must("capture-pane", "-p", "-S", "-", "-t", "=c:"); got != "l1\nl2\nl3\nl4\n" {
		t.Errorf("capture-pane -S - = %q, want the whole history", got)
	}
}

func TestRun_RefusesWhatTmuxWouldRefuse(t *testing.T) {
	t.Parallel()
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "r", "sleep 30")
	for _, tc := range []struct {
		want string
		args []string
	}{
		{"unsupported global flag", []string{"-V"}},
		{"unknown command: nope", []string{"nope"}},
		{"flag -t needs a value", []string{"has-session", "-t"}},
		{"bad session name", []string{"new-session", "-d", "-s", "a:b"}},
		{"bad height", []string{"new-session", "-d", "-s", "h", "-y", "x"}},
		{"duplicate session: r", []string{"new-session", "-d", "-s", "r"}},
		{"no target session", []string{"has-session"}},
		{"bad session name \"..\"", []string{"kill-session", "-t", "=..:"}},
		{"can't find session: zz", []string{"kill-session", "-t", "=zz:"}},
		{"can't find session: zz", []string{"set-option", "-t", "zz", "a", "b"}},
		{"set-option wants a name and a value", []string{"set-option", "-t", "r", "a"}},
		{"can't find session: zz", []string{"show-options", "-t", "zz", "a"}},
		{"show-options wants one name", []string{"show-options", "-t", "r"}},
		{"can't find session: zz", []string{"send-keys", "-t", "zz", "x"}},
		{"load-buffer wants one file", []string{"load-buffer", "-b", "r"}},
		{"no such file", []string{"load-buffer", "-b", "r", filepath.Join(f.root, "missing")}},
		{"can't find session: zz", []string{"paste-buffer", "-b", "r", "-t", "zz"}},
		{"can't find session: zz", []string{"capture-pane", "-p", "-t", "zz"}},
		{"bad start line", []string{"capture-pane", "-p", "-S", "x", "-t", "r"}},
		{"can't find session: zz", []string{"display-message", "-p", "-t", "zz", "x"}},
		{"display-message wants one format", []string{"display-message", "-p", "-t", "r"}},
		{"can't find session: zz", []string{"list-panes", "-t", "zz"}},
	} {
		f.refuses(tc.want, tc.args...)
	}
}

func TestRun_TheStateRootAndTheSocketNameMustBeUsable(t *testing.T) {
	t.Parallel()
	var errOut bytes.Buffer
	if code := Run("", []string{"start-server"}, &bytes.Buffer{}, &errOut); code != 1 || !strings.Contains(errOut.String(), "no state root") {
		t.Errorf("an empty root = exit %d %q, want a refusal", code, errOut.String())
	}
	for _, socket := range []string{"", "..", "a/b"} {
		errOut.Reset()
		if code := Run(t.TempDir(), []string{"-L", socket, "start-server"}, &bytes.Buffer{}, &errOut); code != 1 || !strings.Contains(errOut.String(), "bad socket name") {
			t.Errorf("socket %q = exit %d %q, want a refusal", socket, code, errOut.String())
		}
	}
	root := t.TempDir()
	if code := Run(root, []string{"-f", os.DevNull, "start-server", `\;`}, &bytes.Buffer{}, &errOut); code != 0 {
		t.Fatalf("start-server on the default socket: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(root, defaultSocket, "sessions")); err != nil {
		t.Errorf("start-server must make the default socket's server: %v", err)
	}
}

func TestParseInvocation_ReadsClustersInlineValuesAndTheEndOfFlags(t *testing.T) {
	t.Parallel()
	inv, err := parseInvocation([]string{"-dp", "-tsess", "-S", "-5", "--", "-x"}, "tS")
	if err != nil {
		t.Fatal(err)
	}
	if inv.flags['t'] != "sess" || inv.flags['S'] != "-5" || len(inv.args) != 1 || inv.args[0] != "-x" {
		t.Errorf("parsed %+v", inv)
	}
	if _, d := inv.flags['d']; !d {
		t.Errorf("the cluster -dp must set -d: %+v", inv)
	}
	if inv, _ := parseInvocation([]string{"-q", "-"}, ""); len(inv.args) != 1 || inv.args[0] != "-" {
		t.Errorf("a lone dash is an argument: %+v", inv)
	}
}

func TestTranslateKeys_MapsKeyNamesAndEchoesOnlyWhatATerminalShows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		keys          []string
		literal       bool
		input, echoed string
	}{
		{[]string{"ls", "Enter"}, false, "ls\n", "ls\n"},
		{[]string{"Escape", "C-c", "Up", "Space"}, false, "\x1b\x03\x1b[A ", " "},
		{[]string{"Enter"}, true, "Enter", "Enter"},
		{[]string{"C-cat", "C-1"}, false, "C-catC-1", "C-catC-1"},
	} {
		input, echoed := translateKeys(tc.keys, tc.literal)
		if input != tc.input || echoed != tc.echoed {
			t.Errorf("translateKeys(%q, %v) = %q, %q; want %q, %q", tc.keys, tc.literal, input, echoed, tc.input, tc.echoed)
		}
	}
}

func TestPaneArgv_RunsOneArgumentThroughTheShellAndManyAsAnArgv(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ args, want []string }{
		{nil, []string{"/bin/sh"}},
		{[]string{"echo a"}, []string{"/bin/sh", "-c", "echo a"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
	} {
		if got := paneArgv(tc.args); strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("paneArgv(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestPaneEnv_DropsTheTerminalAndTheOuterTmuxIdentity(t *testing.T) {
	t.Parallel()
	got := paneEnv([]string{"TMUX=/s,1,0", "TMUX_PANE=%1", "TERM=xterm", "TMUX_TMPDIR=/t", "HOME=/h"})
	if strings.Join(got, " ") != "TMUX_TMPDIR=/t HOME=/h" {
		t.Errorf("paneEnv = %q", got)
	}
}

func TestForeground_FollowsTheNewestChildToTheDeepestProcess(t *testing.T) {
	t.Parallel()
	table := []process{{pid: 10, ppid: 1, comm: "/bin/sh"}, {pid: 11, ppid: 10, comm: "old"}, {pid: 12, ppid: 10, comm: "/usr/bin/agy"}, {pid: 13, ppid: 12, comm: "evolve-fake-cli"}, {pid: 5, ppid: 5, comm: "kernel"}}
	if got := foreground(table, 10); got != "evolve-fake-cli" {
		t.Errorf("foreground = %q, want the deepest newest descendant", got)
	}
	if got := foreground(table, 5); got != "kernel" {
		t.Errorf("a self-parented process must not loop: %q", got)
	}
	if got := foreground(table, 99); got != "" {
		t.Errorf("an unknown pane process = %q, want nothing", got)
	}
}

func TestParseProcesses_ReadsPsAndRefusesAMalformedLine(t *testing.T) {
	t.Parallel()
	table, err := parseProcesses("  1   0 /sbin/launchd\n 20  1 /Applications/A B.app/a\n")
	if err != nil || len(table) != 2 || table[1].comm != "/Applications/A B.app/a" || table[1].ppid != 1 {
		t.Fatalf("parseProcesses = %+v, %v", table, err)
	}
	for _, bad := range []string{"1 0", "x 0 sh", "1 y sh"} {
		if _, err := parseProcesses(bad); err == nil {
			t.Errorf("parseProcesses(%q) must refuse the line", bad)
		}
	}
}

func TestScrollbackOf(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"": 0, "-": -1, "-200": 200, "3": 0} {
		if got, err := scrollbackOf(in); err != nil || got != want {
			t.Errorf("scrollbackOf(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}

func TestTargetName_StripsTheExactMatchAndTheWindow(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"=s:": "s", "s": "s", "=s:0.1": "s", "": ""} {
		if got := targetName(in); got != want {
			t.Errorf("targetName(%q) = %q, want %q", in, got, want)
		}
	}
}

var errInjected = errors.New("injected")

func TestFlag_ADoubledFlagIsRefusedNotReadAsATmuxCommand(t *testing.T) {
	t.Parallel()
	var errOut bytes.Buffer
	if code := Run(t.TempDir(), []string{Flag, "start-server"}, &bytes.Buffer{}, &errOut); code != 1 || !strings.Contains(errOut.String(), `unsupported global flag "`+Flag+`"`) {
		t.Errorf("Run(%q) = exit %d %q; want the doubled flag refused", Flag, code, errOut.String())
	}
}
