package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type checkpointHub struct {
	t     *testing.T
	store *gittest.Repo
	root  string
}

func newCheckpointHub(t *testing.T) checkpointHub {
	t.Helper()
	src := gittest.Fixture(t)
	writeCheckpointFile(t, src.Dir, "a.txt", "a\n")
	src.Git("add", "-A")
	src.Git("commit", "-q", "-m", "base")
	store := gittest.Bare(t)
	src.Git("push", "-q", store.Dir, "main")
	store.Git("update-ref", "refs/remotes/origin/main", "main")
	return checkpointHub{t: t, store: store, root: filepath.Dir(store.Dir)}
}

func (h checkpointHub) worktree(rel, branch string) string {
	h.t.Helper()
	dir := filepath.Join(h.root, rel)
	args := []string{"worktree", "add", "-q", "-b", branch, dir, "main"}
	if branch == "" {
		args = []string{"worktree", "add", "-q", "--detach", dir, "main"}
	}
	h.store.Git(args...)
	return dir
}

func (h checkpointHub) refs() string {
	h.t.Helper()
	return h.store.Git("for-each-ref", "--format=%(refname)", "refs/checkpoints/")
}

func writeCheckpointFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkpointGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitexec.Default(dir).Output(context.Background(), args...)
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return out
}

func tickingCheckpointClock(t *testing.T) {
	t.Helper()
	at := time.Date(2026, 10, 6, 6, 0, 0, 0, time.UTC)
	prev := checkpointClock
	checkpointClock = func() time.Time {
		at = at.Add(time.Second)
		return at
	}
	t.Cleanup(func() { checkpointClock = prev })
}

func runCheckpointCLI(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := runCheckpoint(args, nil, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunCheckpoint_SaveAllListAndRestoreRoundTrip(t *testing.T) {
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	console := h.worktree("console", "")
	runtime := h.worktree("runtime", "main-plane")
	dirty := h.worktree("dev/dirty", "feat/dirty")
	h.worktree("dev/clean", "feat/clean")
	writeCheckpointFile(t, dirty, "a.txt", "work in progress\n")
	writeCheckpointFile(t, runtime, "a.txt", "the plane's own state\n")

	code, out, errOut := runCheckpointCLI("save", "--all", "--label", "before lunch", "--project-root", console)
	if code != 0 {
		t.Fatalf("save --all exit %d: %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "saved dirty refs/checkpoints/dirty/") || !strings.Contains(out, "clean clean") {
		t.Errorf("save --all stdout = %q, want a saved line for dirty and a clean line for clean", out)
	}
	if strings.Contains(h.refs(), "runtime") || strings.Contains(out, "runtime") {
		t.Errorf("save --all touched the runtime plane: refs %q, stdout %q", h.refs(), out)
	}

	code, out, errOut = runCheckpointCLI("list", "--json", "--project-root", console)
	var entries []wtcheckpoint.Entry
	if code != 0 || json.Unmarshal([]byte(out), &entries) != nil || len(entries) != 1 {
		t.Fatalf("list --json exit %d stdout %q stderr %q, want one entry", code, out, errOut)
	}
	if e := entries[0]; e.Worktree != "dirty" || e.Label != "before lunch" || e.Files != 1 {
		t.Errorf("entry = %+v, want dirty's labelled one-file checkpoint", e)
	}

	into := filepath.Join(h.root, "dev", "dirty-restored")
	code, out, errOut = runCheckpointCLI("restore", strings.TrimPrefix(entries[0].Ref, "refs/checkpoints/"), "--into", into, "--project-root", console)
	if code != 0 {
		t.Fatalf("restore exit %d: %s%s", code, out, errOut)
	}
	if got, err := os.ReadFile(filepath.Join(into, "a.txt")); err != nil || string(got) != "work in progress\n" {
		t.Errorf("restored a.txt = %q, %v", got, err)
	}
	if !strings.Contains(out, "git switch -c") {
		t.Errorf("restore stdout = %q, want the hint to continue on a branch", out)
	}
}

func TestRunCheckpoint_SaveDefaultsToTheProjectRootAndRefusesANonWorktree(t *testing.T) {
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	dirty := h.worktree("dev/dirty", "feat/dirty")
	writeCheckpointFile(t, dirty, "a.txt", "work\n")

	if code, out, errOut := runCheckpointCLI("save", "--project-root", dirty); code != 0 || !strings.Contains(out, "saved dirty ") {
		t.Errorf("save in a worktree exit %d stdout %q stderr %q", code, out, errOut)
	}
	code, _, errOut := runCheckpointCLI("save", "--worktree", t.TempDir(), "--project-root", dirty)
	if code != 1 || !strings.Contains(errOut, "refused") {
		t.Errorf("save of a plain dir exit %d stderr %q, want 1 and a refusal", code, errOut)
	}
}

func TestRunCheckpoint_RetentionComesFromPolicyJSONAndABrokenPolicyNeverPrunes(t *testing.T) {
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	console := h.worktree("console", "")
	dirty := h.worktree("dev/dirty", "feat/dirty")
	writeCheckpointFile(t, console, ".evolve/policy.json", `{"checkpoint":{"keep_per_worktree":1}}`)
	for _, body := range []string{"one\n", "two\n"} {
		writeCheckpointFile(t, dirty, "a.txt", body)
		if code, out, errOut := runCheckpointCLI("save", "--worktree", dirty, "--project-root", console); code != 0 {
			t.Fatalf("save exit %d: %s%s", code, out, errOut)
		}
	}
	if n := len(strings.Fields(h.refs())); n != 1 {
		t.Fatalf("refs = %q, want keep_per_worktree=1 to leave one", h.refs())
	}

	writeCheckpointFile(t, console, ".evolve/policy.json", `{"checkpoint":{"keep_per_worktee":1}}`)
	writeCheckpointFile(t, dirty, "a.txt", "three\n")
	code, out, errOut := runCheckpointCLI("save", "--worktree", dirty, "--project-root", console)
	if code != 0 || !strings.Contains(out, "saved dirty") || !strings.Contains(errOut, "WARN") || !strings.Contains(errOut, "not pruning") {
		t.Errorf("save under a broken policy exit %d stdout %q stderr %q, want saved with a loud no-prune WARN", code, out, errOut)
	}
	if n := len(strings.Fields(h.refs())); n != 2 {
		t.Errorf("refs = %q, want both kept: a broken policy must never prune", h.refs())
	}
	if code, _, errOut := runCheckpointCLI("prune", "--project-root", console); code != 1 || !strings.Contains(errOut, "keep_per_worktee") {
		t.Errorf("prune under a broken policy exit %d stderr %q, want a refusal naming the bad key", code, errOut)
	}
}

func TestRunCheckpoint_PruneLandedDeletesTheLandedCheckpoint(t *testing.T) {
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	console := h.worktree("console", "")
	dirty := h.worktree("dev/dirty", "feat/dirty")
	writeCheckpointFile(t, dirty, "a.txt", "shipped\n")
	if code, out, errOut := runCheckpointCLI("save", "--worktree", dirty, "--project-root", console); code != 0 {
		t.Fatalf("save exit %d: %s%s", code, out, errOut)
	}
	writeCheckpointFile(t, console, "a.txt", "shipped\n")
	checkpointGit(t, console, "commit", "-qam", "land")
	checkpointGit(t, console, "update-ref", "refs/remotes/origin/main", "HEAD")

	if code, out, _ := runCheckpointCLI("prune", "--project-root", console); code != 0 || strings.TrimSpace(out) != "pruned 0 checkpoint(s)" {
		t.Errorf("prune exit %d stdout %q, want nothing pruned without --landed", code, out)
	}
	code, out, errOut := runCheckpointCLI("prune", "--landed", "--project-root", console)
	if code != 0 || !strings.Contains(out, "pruned refs/checkpoints/dirty/") || !strings.Contains(out, "(landed)") {
		t.Errorf("prune --landed exit %d stdout %q stderr %q", code, out, errOut)
	}
	if h.refs() != "" {
		t.Errorf("refs left = %q, want none", h.refs())
	}
}

func TestRunCheckpoint_UsageErrorsExitTen(t *testing.T) {
	for name, args := range map[string][]string{
		"no subcommand":         nil,
		"unknown subcommand":    {"snapshot"},
		"all and worktree":      {"save", "--all", "--worktree", "x"},
		"save positional":       {"save", "extra"},
		"restore without into":  {"restore", "task/20261006T060000Z"},
		"restore without a ref": {"restore", "--into", "x"},
		"list positional":       {"list", "extra"},
		"prune positional":      {"prune", "extra"},
	} {
		if code, _, _ := runCheckpointCLI(args...); code != 10 {
			t.Errorf("%s: exit %d, want 10", name, code)
		}
	}
	if code, out, _ := runCheckpointCLI("--help"); code != 0 || !strings.Contains(out, "evolve checkpoint save") {
		t.Errorf("--help exit %d stdout %q", code, out)
	}
}

func TestRunCheckpoint_SavePushSendsTheSavedRefAndFailsLoudlyWithoutAnOrigin(t *testing.T) {
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	dirty := h.worktree("dev/dirty", "feat/dirty")
	writeCheckpointFile(t, dirty, "a.txt", "work\n")

	code, out, errOut := runCheckpointCLI("save", "--push", "--project-root", dirty)
	if code != 1 || !strings.Contains(errOut, "git push origin") || !strings.Contains(out, "saved dirty") {
		t.Errorf("save --push with no origin exit %d stdout %q stderr %q, want the save kept, the push failure named and exit 1", code, out, errOut)
	}

	origin := gittest.Bare(t)
	h.store.Git("remote", "add", "origin", origin.Dir)
	h.worktree("dev/clean", "feat/clean")
	writeCheckpointFile(t, dirty, "a.txt", "more work\n")
	code, out, errOut = runCheckpointCLI("save", "--all", "--push", "--json", "--project-root", dirty)
	var rows []checkpointSaveRow
	if code != 0 || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 2 {
		t.Fatalf("save --all --push --json exit %d stdout %q stderr %q, want two rows", code, out, errOut)
	}
	var pushed []string
	for _, r := range rows {
		if r.Pushed != (r.Ref != "") {
			t.Errorf("row %+v: pushed=%v, want pushed exactly when it names a ref", r, r.Pushed)
		}
		if r.Pushed {
			pushed = append(pushed, r.Ref)
		}
	}
	if got := origin.Git("for-each-ref", "--format=%(refname)"); len(pushed) != 1 || got != pushed[0] {
		t.Errorf("origin refs = %q, pushed rows %v; want exactly the one saved ref", got, pushed)
	}
}
