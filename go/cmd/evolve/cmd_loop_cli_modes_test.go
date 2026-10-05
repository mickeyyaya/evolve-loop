package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetachChildArgs_StripsOnlyDetachFlagsInTheFlagRegion(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		flagEnd int
		want    []string
	}{
		{"log consumes a detach-looking value", []string{"--log", "--detach", "--goal-text", "g"}, 4, []string{"--goal-text", "g"}},
		{"equals forms", []string{"--detach=true", "-log=f", "--cycles", "2"}, 4, []string{"--cycles", "2"}},
		{"positional detach kept", []string{"--detach", "--log", "f", "fix", "--detach"}, 3, []string{"fix", "--detach"}},
		{"terminator kept", []string{"--log", "f", "--", "--log"}, 3, []string{"--", "--log"}},
		{"nothing to strip", []string{"--goal-text", "g"}, 2, []string{"--goal-text", "g"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := append([]string(nil), tc.args...)
			if got := detachChildArgs(in, tc.flagEnd); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("detachChildArgs(%q, %d) = %q want %q", tc.args, tc.flagEnd, got, tc.want)
			}
			if !reflect.DeepEqual(in, tc.args) {
				t.Errorf("detachChildArgs mutated its input: %q", in)
			}
		})
	}
}

func TestReservedGoalVerb_MatchesOnlyAWholeReservedWord(t *testing.T) {
	cases := map[string]string{
		"stop": "evolve loop-stop", " STATUS ": "evolve loop status", "Help": "evolve loop -h",
		"plan": "evolve loop --dry-run", "watch": "evolve dashboard",
		"": "", "status of the fleet": "", "stopper": "",
	}
	for goal, want := range cases {
		verb, reserved := reservedGoalVerb(goal)
		if verb != want || reserved != (want != "") {
			t.Errorf("reservedGoalVerb(%q) = (%q, %v) want %q", goal, verb, reserved, want)
		}
	}
}

func TestIsLoopStatusInvocation_RequiresAFlagOrNothingAfterStatus(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"status"}, true},
		{[]string{"status", "--json"}, true},
		{[]string{"status", "of", "the", "fleet"}, false},
		{[]string{"Status"}, false},
		{[]string{"--goal-text", "status"}, false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := isLoopStatusInvocation(tc.args); got != tc.want {
			t.Errorf("isLoopStatusInvocation(%q) = %v want %v", tc.args, got, tc.want)
		}
	}
}

func TestRunLoopStatus_IdlePlaneReportsNoHeartbeatAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runLoop([]string{"status", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d want 0\nstderr:\n%s", rc, stderr.String())
	}
	want := "loop:    running=false brake=false cycle=0 phase=\nlease:   heartbeat=none\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q want %q", stdout.String(), want)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".evolve"))
	if err != nil || len(entries) != 0 {
		t.Errorf("evolve loop status wrote under .evolve (err=%v): %v", err, entries)
	}
	stdout.Reset()
	if rc := runLoop([]string{"status", "--json", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("--json rc=%d", rc)
	}
	var doc map[string]map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil || doc["loop"]["running"] != false || len(doc) != 1 {
		t.Errorf("--json must be {\"loop\": …} only (err=%v): %s", err, stdout.String())
	}
}

func TestRunLoopStatus_RefusalsUseTheirExitCodes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := runLoop([]string{"status", "--project-root", filepath.Join(t.TempDir(), "absent")}, nil, &stdout, &stderr); rc != 2 || !strings.Contains(stderr.String(), "cannot read the snapshot") {
		t.Errorf("a missing .evolve must exit 2 naming the snapshot: rc=%d\n%s", rc, stderr.String())
	}
	stderr.Reset()
	if rc := runLoop([]string{"status", "--bogus"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("an unknown flag must exit 10, got %d", rc)
	}
}

func TestTailLogSince_ReadsOnlyTheNewestLinesAfterTheOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop.log")
	old := "old line\n"
	if err := os.WriteFile(path, []byte(old+"a\n\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tailLogSince(path, int64(len(old)), 2); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Errorf("tail = %q want [b c]", got)
	}
	if got := tailLogSince(path, 1<<20, 5); len(got) != 0 {
		t.Errorf("an offset beyond EOF must yield no lines, got %q", got)
	}
	if got := tailLogSince(filepath.Join(t.TempDir(), "missing"), 0, 5); len(got) != 1 || !strings.HasPrefix(got[0], "(log unreadable:") {
		t.Errorf("an unreadable log must yield one explanatory line, got %q", got)
	}
}

func TestDetachExitCode_MapsWaitErrors(t *testing.T) {
	if got := detachExitCode(nil); got != 0 {
		t.Errorf("nil = %d want 0", got)
	}
	if got := detachExitCode(errors.New("wait failed")); got != -1 {
		t.Errorf("non-exit error = %d want -1", got)
	}
	err := exec.Command("sh", "-c", "exit 4").Run()
	if got := detachExitCode(err); got != 4 {
		t.Errorf("exit 4 = %d want 4 (err=%v)", got, err)
	}
}

func TestDefaultLoopDetachCommand_ReexecsSelfAsLoop(t *testing.T) {
	cmd, err := defaultLoopDetachCommand([]string{"--goal-text", "g"})
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	if cmd.Path != self || !reflect.DeepEqual(cmd.Args[1:], []string{"loop", "--goal-text", "g"}) || cmd.Dir != "" || cmd.Env != nil {
		t.Errorf("child must be `<self> loop <argv>` inheriting cwd and env: path=%q args=%q dir=%q env=%v", cmd.Path, cmd.Args, cmd.Dir, cmd.Env)
	}
}
