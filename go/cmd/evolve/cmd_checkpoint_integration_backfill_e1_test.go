//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestRunCheckpoint_EveryVerbRefusesAProjectRootOutsideAHub(t *testing.T) {
	t.Parallel()
	plain := t.TempDir()
	for _, args := range [][]string{
		{"save"},
		{"list"},
		{"restore", "dirty/20261006T060001Z", "--into", t.TempDir()},
		{"prune"},
	} {
		code, out, errOut := runCheckpointCLI(append(args, "--project-root", plain)...)
		if code != 1 || out != "" || !strings.HasPrefix(errOut, "evolve checkpoint "+args[0]+": wtcheckpoint: ") {
			t.Errorf("%v: exit %d stdout %q stderr %q, want 1 and the hub error", args, code, out, errOut)
		}
	}
}

func savedCheckpointHub(t *testing.T) (checkpointHub, string, string) {
	t.Helper()
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	console := h.worktree("console", "")
	dirty := h.worktree("dev/dirty", "feat/dirty")
	writeCheckpointFile(t, dirty, "a.txt", "one\n")
	if code, out, errOut := runCheckpointCLI("save", "--worktree", dirty, "--label", "first", "--project-root", console); code != 0 {
		t.Fatalf("save exit %d: %s%s", code, out, errOut)
	}
	writeCheckpointFile(t, dirty, "a.txt", "two\n")
	if code, out, errOut := runCheckpointCLI("save", "--worktree", dirty, "--project-root", console); code != 0 {
		t.Fatalf("save exit %d: %s%s", code, out, errOut)
	}
	return h, console, dirty
}

func TestRunCheckpoint_ListPrintsOneLinePerCheckpointWithItsLabel(t *testing.T) {
	_, console, _ := savedCheckpointHub(t)

	code, out, errOut := runCheckpointCLI("list", "--project-root", console)

	if code != 0 {
		t.Fatalf("list exit %d: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want two lines", out)
	}
	labelled := 0
	for _, l := range lines {
		if !strings.HasPrefix(l, "refs/checkpoints/dirty/") || !strings.Contains(l, "  1 file(s) +1 -1") {
			t.Errorf("line %q, want a dirty checkpoint of one changed file", l)
		}
		if strings.HasSuffix(l, "  label: first") {
			labelled++
		}
	}
	if labelled != 1 {
		t.Errorf("stdout = %q, want exactly one labelled line", out)
	}
}

func TestRunCheckpoint_AMalformedCheckpointRefFailsTheList(t *testing.T) {
	h, console, _ := savedCheckpointHub(t)
	h.store.Git("update-ref", "refs/checkpoints/dirty/not-a-stamp", "main")

	code, out, errOut := runCheckpointCLI("list", "--project-root", console)

	if code != 1 || out != "" || !strings.Contains(errOut, "evolve checkpoint list: wtcheckpoint: refs/checkpoints/dirty/not-a-stamp does not end in a") {
		t.Errorf("exit %d stdout %q stderr %q, want 1 and the stamp error", code, out, errOut)
	}
}

func TestRunCheckpoint_JSONWriteFailuresExit1(t *testing.T) {
	_, console, _ := savedCheckpointHub(t)
	var listed bytes.Buffer
	if code := runCheckpointList([]string{"--json", "--project-root", console}, &listed, &bytes.Buffer{}); code != 0 {
		t.Fatalf("list --json exit %d", code)
	}
	var entries []wtcheckpoint.Entry
	if err := json.Unmarshal(listed.Bytes(), &entries); err != nil || len(entries) != 2 {
		t.Fatalf("entries = %v, %v", entries, err)
	}
	ref := strings.TrimPrefix(entries[0].Ref, "refs/checkpoints/")
	cases := map[string]func(stdout, stderr *bytes.Buffer) int{
		"list": func(_, e *bytes.Buffer) int {
			return runCheckpointList([]string{"--json", "--project-root", console}, &failingWriter{}, e)
		},
		"restore": func(_, e *bytes.Buffer) int {
			return runCheckpointRestore([]string{ref, "--into", t.TempDir() + "/r", "--json", "--project-root", console}, &failingWriter{}, e)
		},
		"prune": func(_, e *bytes.Buffer) int {
			return runCheckpointPrune([]string{"--json", "--project-root", console}, &failingWriter{}, e)
		},
	}
	for verb, run := range cases {
		var errb bytes.Buffer
		if code := run(nil, &errb); code != 1 || !strings.Contains(errb.String(), "evolve checkpoint "+verb+": encode json: broken pipe") {
			t.Errorf("%s: exit %d stderr %q, want 1 and the encode error", verb, code, errb.String())
		}
	}
}

func TestRunCheckpoint_RestoreRefusals(t *testing.T) {
	_, console, _ := savedCheckpointHub(t)
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"a ref and a second operand", []string{"dirty/x", "--into", "d", "--project-root", console, "extra"}, 10, checkpointUsage},
		{"a ref then a stray operand before the flags", []string{"dirty/x", "extra", "--into", "d"}, 10, checkpointUsage},
		{"two operands without a leading ref", []string{"--into", "d", "a", "b"}, 10, checkpointUsage},
		{"an unknown flag", []string{"--bogus"}, 10, checkpointUsage},
		{"an absent checkpoint", []string{"dirty/20990101T000000Z", "--into", t.TempDir() + "/r"}, 1, "evolve checkpoint restore: "},
	}
	for _, tc := range cases {
		args := append([]string{"restore"}, tc.args...)
		if !slices.Contains(args, "--project-root") {
			args = append(args, "--project-root", console)
		}
		code, out, errOut := runCheckpointCLI(args...)
		if code != tc.code || out != "" || !strings.Contains(errOut, tc.want) {
			t.Errorf("%s: exit %d stdout %q stderr %q, want %d and %q", tc.name, code, out, errOut, tc.code, tc.want)
		}
	}
}

func TestRunCheckpoint_RestoreJSONReportsTheResult(t *testing.T) {
	_, console, _ := savedCheckpointHub(t)
	_, listed, _ := runCheckpointCLI("list", "--json", "--project-root", console)
	var entries []wtcheckpoint.Entry
	if err := json.Unmarshal([]byte(listed), &entries); err != nil || len(entries) == 0 {
		t.Fatalf("entries = %v, %v", entries, err)
	}
	into := t.TempDir() + "/restored"

	code, out, errOut := runCheckpointCLI("restore", strings.TrimPrefix(entries[0].Ref, "refs/checkpoints/"), "--into", into, "--json", "--project-root", console)

	var res wtcheckpoint.RestoreResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	if res.Ref != entries[0].Ref || res.Dir != into || !res.Created {
		t.Errorf("result = %+v, want the listed ref restored into a new %s", res, into)
	}
}

func TestRunCheckpoint_PruneJSONAndALandedPruneWithoutOriginMainFails(t *testing.T) {
	h, console, _ := savedCheckpointHub(t)
	writeCheckpointFile(t, console, ".evolve/policy.json", `{"checkpoint":{"keep_per_worktree":1}}`)

	code, out, errOut := runCheckpointCLI("prune", "--json", "--project-root", console)
	var pruned []wtcheckpoint.Pruned
	if code != 0 || json.Unmarshal([]byte(out), &pruned) != nil || len(pruned) != 1 {
		t.Fatalf("prune --json exit %d stdout %q stderr %q, want one pruned entry", code, out, errOut)
	}

	h.store.Git("update-ref", "-d", "refs/remotes/origin/main")
	code, out, errOut = runCheckpointCLI("prune", "--landed", "--project-root", console)
	if code != 1 || out != "pruned 0 checkpoint(s)\n" || !strings.Contains(errOut, "evolve checkpoint prune: wtcheckpoint: --landed needs") {
		t.Errorf("prune --landed exit %d stdout %q stderr %q, want 1, an empty report and the origin/main error", code, out, errOut)
	}
}

func TestRunCheckpoint_SaveAllReportsAWorktreeThatCannotBeSavedAndKeepsTheRest(t *testing.T) {
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	console := h.worktree("console", "")
	bad := h.worktree("dev/bad..name", "feat/bad")
	good := h.worktree("dev/good", "feat/good")
	writeCheckpointFile(t, bad, "a.txt", "x\n")
	writeCheckpointFile(t, good, "a.txt", "y\n")

	code, out, errOut := runCheckpointCLI("save", "--all", "--project-root", console)

	if code != 1 {
		t.Fatalf("exit %d, want 1 (stdout %q stderr %q)", code, out, errOut)
	}
	if !strings.Contains(errOut, "evolve checkpoint save: bad..name: wtcheckpoint: refused: worktree name \"bad..name\"") {
		t.Errorf("stderr = %q, want the refused row named", errOut)
	}
	if !strings.Contains(out, "saved good refs/checkpoints/good/") || strings.Contains(out, "bad..name") {
		t.Errorf("stdout = %q, want only the good row printed", out)
	}
}

func TestRunCheckpoint_SaveJSONEncodeFailureExits1(t *testing.T) {
	_, console, dirty := savedCheckpointHub(t)
	writeCheckpointFile(t, dirty, "a.txt", "three\n")
	var errb bytes.Buffer

	code := runCheckpointSave([]string{"--worktree", dirty, "--json", "--project-root", console}, &failingWriter{}, &errb)

	if code != 1 || !strings.Contains(errb.String(), "evolve checkpoint save: encode json: broken pipe") {
		t.Errorf("exit %d stderr %q, want 1 and the encode error", code, errb.String())
	}
}
