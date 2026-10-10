package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	pmSession1 = "c61dea20-648d-4002-a794-85f794e80ba0"
	pmFixture1 = "../../internal/attemptpostmortem/testdata/cycle1853-build-attempt1.jsonl"
)

type postmortemFixture struct {
	root, home string
	env        postmortemEnv
}

func newPostmortemFixture(t *testing.T) postmortemFixture {
	t.Helper()
	f := postmortemFixture{root: t.TempDir(), home: t.TempDir()}
	if err := os.MkdirAll(paths.EvolveDirOf(f.root), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(pmFixture1)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.home, ".claude", "projects", "-runtime--evolve-worktrees-cycle-cd3ae73e-1853")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, pmSession1+".jsonl"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	f.env = postmortemEnv{home: func() string { return f.home }, resolveRoot: loopStopRoot}
	return f
}

func (f postmortemFixture) run(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := runPostmortemWith(f.env, append(args, "--project-root", f.root), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestPostmortem_CollectThenShowRoundTripsTheCycle1853Backfill(t *testing.T) {
	f := newPostmortemFixture(t)

	code, out, errOut := f.run("collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--session", pmSession1,
		"--cause", "pane_lost", "--exit-code", "81", "--started", "2026-10-09T09:19:20Z", "--ended", "2026-10-09T09:59:52Z")

	if code != 0 {
		t.Fatalf("collect = %d, stderr %s", code, errOut)
	}
	want := attemptpostmortem.Path(paths.RunWorkspace(f.root, 1853), "build", 1)
	if !strings.Contains(out, want) {
		t.Fatalf("collect stdout = %q, want the record path %s", out, want)
	}
	code, out, errOut = f.run("show", "--cycle", "1853", "--phase", "build")
	if code != 0 {
		t.Fatalf("show = %d, stderr %s", code, errOut)
	}
	for _, s := range []string{attemptpostmortem.SectionHeading, "### Attempt 1", "session `" + pmSession1 + "`", "cause `pane_lost`, exit code 81", "tmux kill-server"} {
		if !strings.Contains(out, s) {
			t.Errorf("show lacks %q:\n%s", s, out)
		}
	}
}

func TestPostmortem_CollectWithATranscriptPathAndNoTimesUsesTheTranscriptWindow(t *testing.T) {
	f := newPostmortemFixture(t)

	code, _, errOut := f.run("collect", "--cycle", "1853", "--phase", "build", "--attempt", "2", "--transcript", pmFixture1, "--cause", "pane_lost", "--cli", "claude")

	if code != 0 {
		t.Fatalf("collect = %d, stderr %s", code, errOut)
	}
	records, err := attemptpostmortem.ReadAll(paths.RunWorkspace(f.root, 1853), "build")
	if err != nil || len(records) != 1 {
		t.Fatalf("records = %+v, %v", records, err)
	}
	r := records[0]
	if r.CLI != "claude" || r.Session != "cycle1853-build-attempt1" || r.StartedAt.Format("15:04:05.000") != "09:20:32.418" || r.EndedAt.Format("15:04:05.000") != "09:21:08.277" || r.Suspect == nil {
		t.Fatalf("record = %+v, want the transcript window and the probe", r)
	}
}

func TestPostmortem_ShowWithNoRecordsSaysSo(t *testing.T) {
	f := newPostmortemFixture(t)
	code, out, errOut := f.run("show", "--cycle", "7", "--phase", "build")
	if code != 0 || out != "" || !strings.Contains(errOut, "no attempt postmortem") {
		t.Fatalf("show = %d %q %q, want 0, no section and a note", code, out, errOut)
	}
}

func TestPostmortem_RefusesBadArgumentsWithExit10(t *testing.T) {
	f := newPostmortemFixture(t)
	valid := []string{"--cycle", "1853", "--phase", "build", "--attempt", "1", "--session", pmSession1, "--cause", "pane_lost"}
	cases := map[string][]string{
		"no verb":            {},
		"unknown verb":       {"list"},
		"unknown flag":       append([]string{"show", "--bogus"}, valid[:4]...),
		"operand":            {"show", "--cycle", "1", "--phase", "build", "extra"},
		"show no cycle":      {"show", "--phase", "build"},
		"collect bad cycle":  {"collect", "--cycle", "-3", "--phase", "build", "--attempt", "1", "--session", pmSession1, "--cause", "pane_lost"},
		"bad cycle":          {"show", "--cycle", "x", "--phase", "build"},
		"zero cycle":         {"show", "--cycle", "0", "--phase", "build"},
		"bad phase":          {"show", "--cycle", "1", "--phase", "../build"},
		"no attempt":         {"collect", "--cycle", "1853", "--phase", "build", "--session", pmSession1, "--cause", "pane_lost"},
		"bad attempt":        {"collect", "--cycle", "1853", "--phase", "build", "--attempt", "-1", "--session", pmSession1, "--cause", "pane_lost"},
		"no cause":           {"collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--session", pmSession1},
		"no source":          {"collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--cause", "pane_lost"},
		"both sources":       append(append([]string{"collect"}, valid...), "--transcript", pmFixture1),
		"bad session":        {"collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--session", "../*", "--cause", "pane_lost"},
		"bad exit code":      append(append([]string{"collect"}, valid...), "--exit-code", "x"),
		"bad started":        append(append([]string{"collect"}, valid...), "--started", "yesterday", "--ended", "2026-10-09T09:59:52Z"),
		"bad ended":          append(append([]string{"collect"}, valid...), "--started", "2026-10-09T09:19:20Z", "--ended", "now"),
		"only started":       append(append([]string{"collect"}, valid...), "--started", "2026-10-09T09:19:20Z"),
		"ended before start": append(append([]string{"collect"}, valid...), "--started", "2026-10-09T10:00:00Z", "--ended", "2026-10-09T09:00:00Z"),
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			code, _, errOut := f.run(args...)
			if code != exitUsage || !strings.Contains(errOut, "usage: evolve postmortem") {
				t.Fatalf("%v = %d (%s), want %d with the usage", args, code, errOut, exitUsage)
			}
		})
	}
}

func TestPostmortem_CollectFailuresAreNotUsageErrors(t *testing.T) {
	f := newPostmortemFixture(t)
	base := []string{"collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--cause", "pane_lost"}
	missing := append(append([]string{}, base...), "--session", "00000000-0000-0000-0000-000000000000")
	if code, _, errOut := f.run(missing...); code != exitRefused || !strings.Contains(errOut, "no transcript") {
		t.Fatalf("an unknown session = %d (%s), want %d", code, errOut, exitRefused)
	}
	f.env.home = func() string { return t.TempDir() }
	if code, _, errOut := f.run(withSessionArgs(base, pmSession1)...); code != exitRefused || !strings.Contains(errOut, "no transcript") {
		t.Fatalf("a home with no projects = %d (%s), want %d", code, errOut, exitRefused)
	}
	f = newPostmortemFixture(t)
	unreadable := append(append([]string{}, base...), "--transcript", t.TempDir())
	if code, _, errOut := f.run(unreadable...); code != exitIO || !strings.Contains(errOut, "read transcript") {
		t.Fatalf("an unreadable transcript = %d (%s), want %d", code, errOut, exitIO)
	}
	if err := os.WriteFile(paths.PolicyPath(paths.EvolveDirOf(f.root)), []byte(`{"attempt_postmortem":{"bogus":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	withSession := append(append([]string{}, base...), "--session", pmSession1)
	if code, _, errOut := f.run(withSession...); code != exitIO || !strings.Contains(errOut, "attempt_postmortem") {
		t.Fatalf("a bad policy = %d (%s), want %d", code, errOut, exitIO)
	}
	if code, _, errOut := f.run("show", "--cycle", "1853", "--phase", "build"); code != exitIO {
		t.Fatalf("show with a bad policy = %d (%s), want %d", code, errOut, exitIO)
	}
}

func withSessionArgs(base []string, session string) []string {
	return append(append([]string{}, base...), "--session", session)
}

func TestPostmortem_WriteAndReadFailuresAreIOErrors(t *testing.T) {
	f := newPostmortemFixture(t)
	ws := paths.RunWorkspace(f.root, 1853)
	if err := os.MkdirAll(attemptpostmortem.Path(ws, "build", 1), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := f.run("collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--session", pmSession1, "--cause", "pane_lost")
	if code != exitIO {
		t.Fatalf("a directory at the record path = %d (%s), want %d", code, errOut, exitIO)
	}
	if code, _, _ := f.run("show", "--cycle", "1853", "--phase", "build"); code != exitIO {
		t.Fatalf("show with an unreadable record = %d, want %d", code, exitIO)
	}
}

func TestPostmortem_RootResolutionFailureIsAnIOError(t *testing.T) {
	f := newPostmortemFixture(t)
	f.env.resolveRoot = func(string, io.Writer) (string, error) { return "", errors.New("no working directory") }
	for _, verb := range [][]string{{"show", "--cycle", "1", "--phase", "build"}, {"collect", "--cycle", "1853", "--phase", "build", "--attempt", "1", "--session", pmSession1, "--cause", "pane_lost"}} {
		code, _, errOut := f.run(verb...)
		if code != exitIO || !strings.Contains(errOut, "no working directory") {
			t.Fatalf("%s with no root = %d (%s), want %d", verb[0], code, errOut, exitIO)
		}
	}
}

func TestRunPostmortem_IsRegisteredAndUsesTheProcessHome(t *testing.T) {
	if c := lookupCommand("postmortem"); c == nil {
		t.Fatal("postmortem is not registered")
	}
	t.Setenv("HOME", "/the/home")
	if got := defaultPostmortemEnv().home(); got != "/the/home" {
		t.Fatalf("home() = %q", got)
	}
	var stderr bytes.Buffer
	if code := runPostmortem(nil, nil, &bytes.Buffer{}, &stderr); code != exitUsage {
		t.Fatalf("runPostmortem() = %d, want %d", code, exitUsage)
	}
}
