package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type gcDevHubEnv struct {
	t       *testing.T
	store   *gittest.Repo
	root    string
	runtime string
	evolve  string
}

func newGCDevHub(t *testing.T, policyJSON string) gcDevHubEnv {
	t.Helper()
	origReapers := gcInjectedReapers
	origHost := gcInjectedProcessHost
	t.Cleanup(func() {
		gcInjectedReapers = origReapers
		gcInjectedProcessHost = origHost
	})
	gcInjectedReapers = &gcReapers{
		sessions: func(context.Context) swarm.OrphanReapReport { return swarm.OrphanReapReport{} },
		sockets:  func(context.Context) swarm.OrphanSocketReport { return swarm.OrphanSocketReport{} },
	}
	gcInjectedProcessHost = &gcProcessHost{
		list:   func(context.Context) ([]proctree.Process, error) { return nil, nil },
		signal: func(int, syscall.Signal) error { return nil },
		sleep:  func(time.Duration) {},
		alive:  func(int) bool { return false },
	}
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("GOCACHE", t.TempDir())
	binDir := t.TempDir()
	fakeclitest.Install(t, filepath.Join(binDir, "lsof"), "#!/bin/sh\nprintf 'p1\\nR0\\nn/\\n'\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	tpl := gittest.FromTemplate(t, "cmd-evolve-gc-devhub-store", func(r *gittest.Repo) error {
		r.Write("base.txt", "base\n")
		r.Git("add", "-A")
		r.Git("commit", "-q", "-m", "base")
		bare := filepath.Join(r.Dir, "store.git")
		r.Git("clone", "-q", "--bare", ".", bare)
		r.Git("-C", bare, "update-ref", "refs/remotes/origin/main", "main")
		for _, kv := range gittest.MaintenanceConfig() {
			r.Git("-C", bare, "config", kv[0], kv[1])
		}
		return nil
	})
	root := filepath.Dir(tpl.Dir)
	storeDir := filepath.Join(root, "repo.git")
	if err := os.Rename(filepath.Join(tpl.Dir, "store.git"), storeDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(tpl.Dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(storeDir, tpl.Dir); err != nil {
		t.Fatal(err)
	}
	store := tpl
	runtime := filepath.Join(root, "runtime")
	store.Git("worktree", "add", "-q", "--detach", runtime, "main")
	evolve := filepath.Join(runtime, ".evolve")
	if err := os.MkdirAll(evolve, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolve, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return gcDevHubEnv{t: t, store: store, root: root, runtime: runtime, evolve: evolve}
}

func (h gcDevHubEnv) addDevAt(name string, ageDays int) string {
	h.t.Helper()
	dir := filepath.Join(h.root, "dev", name)
	if ageDays > 0 {
		stamp := strconv.FormatInt(time.Now().Add(-time.Duration(ageDays)*24*time.Hour).Unix(), 10) + " +0000"
		h.t.Setenv("GIT_COMMITTER_DATE", stamp)
		h.t.Setenv("GIT_AUTHOR_DATE", stamp)
		defer os.Unsetenv("GIT_COMMITTER_DATE")
		defer os.Unsetenv("GIT_AUTHOR_DATE")
	}
	h.store.Git("worktree", "add", "-q", "-b", "br-"+name, dir, "main")
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(name+"\n"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	h.store.Git("-C", dir, "add", "-A")
	h.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "work "+name)
	return dir
}

func fakeGhOnPath(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunGC_DryRunListsEachDevWorktreeWithItsReasonAndDaysIdleAndMutatesNothing(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	fakeGhOnPath(t, `echo '[]'`)
	oldDir := h.addDevAt("old-lane", 30)
	newDir := h.addDevAt("new-lane", 0)

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--dry-run", "--project-root", h.runtime}, nil, &stdout, &stderr)
	out := stdout.String()

	if !strings.Contains(out, "evolve gc --dry-run: 1 dev worktree(s) would be archived (1 kept, 0 not judged)") {
		t.Errorf("summary line missing:\n%s", out)
	}
	if !strings.Contains(out, "WOULD-ARCHIVE path="+oldDir) && !strings.Contains(out, "WOULD-ARCHIVE path=/private"+oldDir) {
		t.Errorf("no WOULD-ARCHIVE line for the 30-day-old tree:\n%s", out)
	}
	if !strings.Contains(out, "branch=br-old-lane idle=30.0d (idle 30.0 days (limit 14); branch br-old-lane is not in main and stays)") {
		t.Errorf("the old tree's line lacks the branch, the days idle or the reason:\n%s", out)
	}
	if !strings.Contains(out, "KEEP path=") || !strings.Contains(out, "(active within the 14-day limit)") {
		t.Errorf("no KEEP line with a reason for the new tree:\n%s", out)
	}
	for _, d := range []string{oldDir, newDir} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("--dry-run removed %s: %v", d, err)
		}
	}
	if _, err := os.Stat(gc.DevArchivePath(h.evolve)); !os.IsNotExist(err) {
		t.Errorf("--dry-run wrote the archive manifest (stat err %v)", err)
	}
	if rc != 0 {
		t.Errorf("rc = %d, want 0; stderr=%s", rc, stderr.String())
	}
}

func TestRunGC_ExplicitRunArchivesAnIdleDirtyDevWorktreeAndPrintsHowToRestoreIt(t *testing.T) {
	h := newGCDevHub(t, `{"gc":{"dev_worktrees":{"archive_after_days":20}}}`)
	fakeGhOnPath(t, `echo '[]'`)
	oldDir := h.addDevAt("old-lane", 30)
	keepDir := h.addDevAt("mid-lane", 10)
	if err := os.WriteFile(filepath.Join(oldDir, "wip.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--project-root", h.runtime}, nil, &stdout, &stderr)
	out := stdout.String()

	if rc != 0 {
		t.Fatalf("rc = %d; stdout=%s stderr=%s", rc, out, stderr.String())
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("the 30-day-old tree survived a 20-day limit (stat err %v)", err)
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Errorf("the 10-day-old tree was removed under a 20-day limit: %v", err)
	}
	if strings.TrimSpace(h.store.Git("branch", "--list", "br-old-lane")) == "" {
		t.Error("the archived tree's branch was deleted")
	}
	for _, want := range []string{"ARCHIVE path=", "archived path=", "checkpoint=refs/checkpoints/old-lane/", "restore: git worktree add ", " br-old-lane", "restore: evolve checkpoint restore old-lane/", "dev worktree archive applied; manifest "} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	recs, err := gc.ReadDevArchive(h.evolve)
	if err != nil || len(recs) != 1 || recs[0].Branch != "br-old-lane" {
		t.Fatalf("manifest = %+v (err %v), want the one archived tree", recs, err)
	}
	h.store.Git("worktree", "add", "-q", oldDir, "br-old-lane")
	hub, err := wtcheckpoint.ResolveHub(h.runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wtcheckpoint.Restore(context.Background(), hub, recs[0].Checkpoint, oldDir); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(oldDir, "wip.txt")); err != nil || string(got) != "uncommitted\n" {
		t.Errorf("restored wip.txt = %q (err %v), want the uncommitted work back", got, err)
	}
}

func TestRunGC_AnOpenPullRequestKeepsAnIdleDevWorktree(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	fresh := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	fakeGhOnPath(t, `echo '[{"headRefName":"br-pr-lane","updatedAt":"`+fresh+`"}]'`)
	dir := h.addDevAt("pr-lane", 40)

	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", h.runtime}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d; stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("a tree with an open PR updated a day ago was archived: %v", err)
	}
	if !strings.Contains(stdout.String(), "open pull request updated") {
		t.Errorf("output does not say why the tree is kept:\n%s", stdout.String())
	}
}

func TestRunGC_WithoutGhTheDryRunListsTheTreeWithANoteAndTheRealRunRefusesIt(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	dir := h.addDevAt("nogh-lane", 40)
	pathWithoutGh(t)

	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--dry-run", "--project-root", h.runtime}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("dry run rc = %d; stderr=%s", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "NOTE gh is unavailable: the open pull request check was skipped") || !strings.Contains(out, "WOULD-ARCHIVE path=") {
		t.Errorf("the dry run lacks the note or the candidate:\n%s", out)
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGC([]string{"--project-root", h.runtime}, nil, &stdout, &stderr); rc != 1 {
		t.Fatalf("real run rc = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "must not skip the open pull request rule") {
		t.Errorf("stderr lacks the refusal reason: %s", stderr.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("a real run archived a tree without the pull request rule: %v", err)
	}
}

func pathWithoutGh(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"git", "lsof", "ps", "sh", "env", "go", "tmux", "uname"} {
		if p, err := exec.LookPath(tool); err == nil {
			if err := os.Symlink(p, filepath.Join(bin, tool)); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("PATH", bin)
}

func TestGCRunDevWorktrees_ASweepOutsideAHubIsSkippedWithItsReason(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)
	failed := r.devWorktrees(projectRoot, filepath.Join(projectRoot, ".evolve"), gc.Policy{})
	if failed || !strings.Contains(stdout.String(), "dev worktree sweep skipped: ") {
		t.Errorf("failed=%v stdout=%q, want a skip line and no failure", failed, stdout.String())
	}
}

func TestGCRunDevWorktrees_APlanFailureAndAnUnjudgedTreeAreFailures(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	h.addDevAt("old-lane", 40)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)

	opts := mustDevOptions(t, h.runtime, h.evolve, gc.Policy{})
	opts.Exec = nil
	if !r.devWorktreesWith(opts) || !strings.Contains(stderr.String(), "dev worktree sweep plan failed") {
		t.Errorf("a plan failure: stderr=%q, want it reported and counted as a failure", stderr.String())
	}

	opts = mustDevOptions(t, h.runtime, h.evolve, gc.Policy{})
	opts.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) {
		return nil, errors.New("gh rate limited")
	}
	stdout.Reset()
	if !r.devWorktreesWith(opts) || !strings.Contains(stdout.String(), "NOT-JUDGED path=") || !strings.Contains(stdout.String(), "1 not judged") {
		t.Errorf("an unjudged tree: stdout=%q, want NOT-JUDGED and a failure", stdout.String())
	}
}

func TestGCRunDevWorktrees_AnArchiveRefusalIsReportedAsAFailure(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	dir := h.addDevAt("unsaved-lane", 40)
	if err := os.WriteFile(filepath.Join(dir, "wip.txt"), []byte("w\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	opts := mustDevOptions(t, h.runtime, h.evolve, gc.Policy{})
	opts.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) { return nil, nil }
	opts.Save = func(context.Context, wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
		return wtcheckpoint.SaveResult{}, errors.New("disk full")
	}
	if !r.devWorktreesWith(opts) {
		t.Error("devWorktrees = false, want a failure when the checkpoint save fails")
	}
	if !strings.Contains(stderr.String(), "dev worktree archive partial") || !strings.Contains(stderr.String(), "disk full") {
		t.Errorf("stderr = %q, want the refusal named", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "wip.txt")); err != nil {
		t.Errorf("the work file is gone: %v", err)
	}
}

func TestGCDevItemLine_NamesTheVerbTheBranchAndTheIdleDays(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		it     gc.DevItem
		dryRun bool
		want   string
	}{
		{"would archive", gc.DevItem{Path: "/p", Branch: "b", Action: gc.DevActionArchive, IdleDays: 21.04, Reason: "r"}, true, "WOULD-ARCHIVE path=/p branch=b idle=21.0d (r)"},
		{"archive", gc.DevItem{Path: "/p", Branch: "b", Action: gc.DevActionArchive, Reason: "r"}, false, "ARCHIVE path=/p branch=b idle=0.0d (r)"},
		{"keep without a branch", gc.DevItem{Path: "/p", Action: gc.DevActionKeep, Reason: "detached"}, false, "KEEP path=/p idle=0.0d (detached)"},
		{"not judged", gc.DevItem{Path: "/p", Action: gc.DevActionKeep, Failed: true, Reason: "cannot judge: x"}, true, "NOT-JUDGED path=/p idle=0.0d (cannot judge: x)"},
	}
	for _, c := range cases {
		if got := gcDevItemLine(c.it, c.dryRun); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestGCCheckpointLabel(t *testing.T) {
	t.Parallel()
	if got := gcCheckpointLabel(gc.DevArchiveRecord{}); !strings.Contains(got, "clean") {
		t.Errorf("no checkpoint: %q, want it to say the tree was clean", got)
	}
	if got := gcCheckpointLabel(gc.DevArchiveRecord{Checkpoint: "refs/checkpoints/x/1"}); got != "refs/checkpoints/x/1" {
		t.Errorf("checkpoint: %q, want the ref", got)
	}
}

func TestParseOpenPRUpdates(t *testing.T) {
	t.Parallel()
	got, err := parseOpenPRUpdates([]byte(`[{"headRefName":"a","updatedAt":"2026-10-01T00:00:00Z"},{"headRefName":"a","updatedAt":"2026-10-02T00:00:00Z"},{"headRefName":"b","updatedAt":"2026-10-03T00:00:00Z"}]`))
	if err != nil || len(got["a"]) != 2 || len(got["b"]) != 1 {
		t.Fatalf("got %v, %v; want two updates for a and one for b", got, err)
	}
	if _, err := parseOpenPRUpdates([]byte(`not json`)); err == nil {
		t.Error("garbage: nil error, want it surfaced")
	}
}

func TestGhOpenPRUpdates(t *testing.T) {
	t.Run("missing gh is the unavailable sentinel", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := ghOpenPRUpdates(context.Background(), t.TempDir()); !errors.Is(err, gc.ErrPRLookupUnavailable) {
			t.Errorf("err = %v, want gc.ErrPRLookupUnavailable", err)
		}
	})
	t.Run("a failing gh surfaces its stderr", func(t *testing.T) {
		fakeGhOnPath(t, `echo 'auth required' >&2; exit 4`)
		_, err := ghOpenPRUpdates(context.Background(), t.TempDir())
		if err == nil || errors.Is(err, gc.ErrPRLookupUnavailable) || !strings.Contains(err.Error(), "auth required") {
			t.Errorf("err = %v, want a real failure that carries gh's stderr", err)
		}
	})
	t.Run("a working gh is parsed", func(t *testing.T) {
		fakeGhOnPath(t, `echo '[{"headRefName":"x","updatedAt":"2026-10-01T00:00:00Z"}]'`)
		got, err := ghOpenPRUpdates(context.Background(), t.TempDir())
		if err != nil || len(got["x"]) != 1 {
			t.Errorf("got %v, %v", got, err)
		}
	})
}

func TestGhStderr_OnlyAnExitErrorCarriesIt(t *testing.T) {
	t.Parallel()
	if got := ghStderr(errors.New("plain")); got != "" {
		t.Errorf("ghStderr(plain) = %q, want empty", got)
	}
}

func TestRunDevWorktreeGC_PublishesThePlanAndNeverArchivesEvenUnderEnforce(t *testing.T) {
	h := newGCDevHub(t, `{"gc":{"mode":"enforce"}}`)
	fakeGhOnPath(t, `echo '[]'`)
	dir := h.addDevAt("old-lane", 40)
	workspace := t.TempDir()

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: h.evolve, ProjectRoot: h.runtime}, workspace, &buf)

	raw, err := os.ReadFile(filepath.Join(workspace, "dev-worktree-gc-manifest.json"))
	if err != nil {
		t.Fatalf("no dev worktree manifest published: %v (stderr=%q)", err, buf.String())
	}
	var m gc.DevManifest
	if err := json.Unmarshal(raw, &m); err != nil || len(m.Items) != 1 || m.Items[0].Action != gc.DevActionArchive {
		t.Fatalf("manifest = %s (err %v), want one archive candidate", raw, err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the in-loop hook removed %s although only evolve gc archives: %v", dir, err)
	}
	if !strings.Contains(buf.String(), "[gc] dev worktree shadow: 1 of 1 idle past the archive limit (only evolve gc archives them)") {
		t.Errorf("stderr lacks the shadow line: %q", buf.String())
	}
}

func TestRunDevWorktreeGC_QuietCasesAndFailures(t *testing.T) {
	var buf bytes.Buffer
	runDevWorktreeGC(loopConfig{}, t.TempDir(), gc.Policy{}, &buf)
	if buf.Len() != 0 {
		t.Errorf("an unset ProjectRoot wrote %q, want silence", buf.String())
	}

	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "")
	runDevWorktreeGC(loopConfig{EvolveDir: evolveDir, ProjectRoot: projectRoot}, workspace, gc.Policy{}, &buf)
	if buf.Len() != 0 {
		t.Errorf("a non-hub project wrote %q, want silence", buf.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, "dev-worktree-gc-manifest.json")); !os.IsNotExist(err) {
		t.Errorf("a non-hub project published a manifest (stat err %v)", err)
	}

	h := newGCDevHub(t, `{}`)
	fakeGhOnPath(t, `echo '[]'`)
	h.addDevAt("old-lane", 40)
	if err := os.WriteFile(filepath.Join(h.store.Dir, "HEAD"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDevWorktreeGC(loopConfig{EvolveDir: h.evolve, ProjectRoot: h.runtime}, t.TempDir(), gc.Policy{}, &buf)
	if !strings.Contains(buf.String(), "[gc] WARN: dev worktree plan failed") {
		t.Errorf("a corrupt hub: stderr=%q, want a plan-failure warning", buf.String())
	}
}

func TestRunDevWorktreeGC_AnUnwritableWorkspaceIsReportedByThePublisher(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	fakeGhOnPath(t, `echo '[]'`)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	runDevWorktreeGC(loopConfig{EvolveDir: h.evolve, ProjectRoot: h.runtime}, filepath.Join(blocker, "ws"), gc.Policy{}, &buf)
	if !strings.Contains(buf.String(), "create workspace") || strings.Contains(buf.String(), "dev worktree shadow") {
		t.Errorf("stderr = %q, want the publish failure and no shadow line", buf.String())
	}
}

func TestDevWorktreeGCOptions_CarriesThePolicyTheCallerAndTheRealSeams(t *testing.T) {
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "/the/caller", nil }
	o, err := devWorktreeGCOptions("/root", "/root/.evolve", gc.Policy{DevWorktrees: gc.DevWorktreesPolicy{ArchiveAfterDays: 9}})
	if err != nil {
		t.Fatal(err)
	}
	if o.ProjectRoot != "/root" || o.EvolveDir != "/root/.evolve" || o.Cwd != "/the/caller" || o.Policy.ArchiveAfterDays != 9 || o.Exec == nil || o.OpenPRs == nil {
		t.Errorf("options = %+v, want root, evolve dir, caller, policy, Exec and OpenPRs wired", o)
	}
}

func mustDevOptions(t *testing.T, root, evolve string, pol gc.Policy) gc.DevOptions {
	t.Helper()
	o, err := devWorktreeGCOptions(root, evolve, pol)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestDevWorktreeGCOptions_AnUnreadableCallerDirectoryFailsClosed(t *testing.T) {
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }
	if _, err := devWorktreeGCOptions("/root", "/root/.evolve", gc.Policy{}); err == nil || !strings.Contains(err.Error(), "caller's directory is unreadable") {
		t.Fatalf("err = %v, want the closed refusal", err)
	}

	h := newGCDevHub(t, `{}`)
	fakeGhOnPath(t, `echo '[]'`)
	dir := h.addDevAt("old-lane", 40)
	processGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }

	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	if !r.devWorktrees(h.runtime, h.evolve, gc.Policy{}) || !strings.Contains(stderr.String(), "dev worktree sweep refused") {
		t.Errorf("devWorktrees did not refuse: stderr=%q", stderr.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the tree was touched although the caller's directory is unknown: %v", err)
	}

	var hook bytes.Buffer
	workspace := t.TempDir()
	runDevWorktreeGC(loopConfig{EvolveDir: h.evolve, ProjectRoot: h.runtime}, workspace, gc.Policy{}, &hook)
	if !strings.Contains(hook.String(), "dev worktree sweep skipped") {
		t.Errorf("hook stderr = %q, want the skip warning", hook.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, "dev-worktree-gc-manifest.json")); !os.IsNotExist(err) {
		t.Errorf("the hook published a plan without knowing the caller's directory (stat err %v)", err)
	}
}

func TestRunGC_LogsEachDeletedBranchWithItsHead(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	head := strings.TrimSpace(gcGit(t, projectRoot, "rev-parse", gcOrphanBranch))
	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr); rc == 10 {
		t.Fatalf("rc=10 stderr=%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "gc: deleted branch "+gcOrphanBranch+"@"+head) {
		t.Errorf("stdout lacks the branch@head line:\n%s", stdout.String())
	}
}

func TestRunGCHook_LogsEachDeletedBranchWithItsHeadUnderEnforce(t *testing.T) {
	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "enforce")
	head := strings.TrimSpace(gcGit(t, projectRoot, "rev-parse", gcOrphanBranch))
	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: projectRoot}, workspace, &buf)
	if !strings.Contains(buf.String(), "gc: deleted branch "+gcOrphanBranch+"@"+head) {
		t.Errorf("stderr lacks the branch@head line:\n%s", buf.String())
	}
}

func endableSweep(t *testing.T) func() {
	t.Helper()
	prev := devSweepContext
	t.Cleanup(func() { devSweepContext = prev })
	var mu sync.Mutex
	var end context.CancelFunc = func() {}
	devSweepContext = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
		mu.Lock()
		end = cancel
		mu.Unlock()
		return ctx, cancel
	}
	return func() {
		mu.Lock()
		defer mu.Unlock()
		end()
	}
}

func blockingDevExec(verb, dirPart string, endSweep func()) sysexec.RunFunc {
	return func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == verb && strings.Contains(d, dirPart) {
			endSweep()
			<-ctx.Done()
			return -1, ctx.Err()
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
}

func oneSecondBudget() gc.Policy {
	return gc.Policy{DevWorktrees: gc.DevWorktreesPolicy{SweepBudgetSeconds: 1}}
}

func TestGCRunDevWorktrees_ATreeCutByTheSweepBudgetIsSkippedWithAWarnAndIsNotAFailure(t *testing.T) {
	endSweep := endableSweep(t)
	h := newGCDevHub(t, `{}`)
	h.addDevAt("slow-lane", 40)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)
	opts := mustDevOptions(t, h.runtime, h.evolve, oneSecondBudget())
	opts.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) { return nil, nil }
	opts.Exec = blockingDevExec("symbolic-ref", "slow-lane", endSweep)
	if r.devWorktreesWith(opts) {
		t.Errorf("a budget skip failed gc; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "SKIPPED path=") || strings.Contains(stdout.String(), "NOT-JUDGED") {
		t.Errorf("stdout = %q, want SKIPPED and no NOT-JUDGED", stdout.String())
	}
	if !strings.Contains(stderr.String(), "WARN") || !strings.Contains(stderr.String(), "1s budget") || !strings.Contains(stderr.String(), "1 tree(s) SKIPPED") {
		t.Errorf("stderr = %q, want a WARN naming the budget and the count", stderr.String())
	}
}

func TestGCRunDevWorktrees_ASlowProcessListWarnsArchivesNothingAndIsNotAFailure(t *testing.T) {
	endSweep := endableSweep(t)
	h := newGCDevHub(t, `{}`)
	dir := h.addDevAt("old-lane", 40)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	opts := mustDevOptions(t, h.runtime, h.evolve, oneSecondBudget())
	opts.Processes = func(ctx context.Context) ([]gc.Process, error) {
		endSweep()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if r.devWorktreesWith(opts) {
		t.Errorf("a process-list timeout failed gc; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "WARN: dev worktree sweep over its 1s budget") {
		t.Errorf("stderr = %q, want the over-budget WARN", stderr.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("a tree was archived without a process listing: %v", err)
	}
}

func TestGCRunDevWorktrees_ASpentBudgetDefersTheArchiveOfTheTreesItJudged(t *testing.T) {
	endSweep := endableSweep(t)
	h := newGCDevHub(t, `{}`)
	oldDir := h.addDevAt("old-lane", 40)
	h.addDevAt("slow-lane", 40)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	opts := mustDevOptions(t, h.runtime, h.evolve, oneSecondBudget())
	opts.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) { return nil, nil }
	opts.Exec = blockingDevExec("symbolic-ref", "slow-lane", endSweep)
	if r.devWorktreesWith(opts) {
		t.Errorf("a deferred archive failed gc; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "archive deferred to the next run") {
		t.Errorf("stderr = %q, want the deferral WARN", stderr.String())
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Errorf("the judged tree was removed on an expired budget: %v", err)
	}
}

func TestGCRunDevWorktrees_TheSweepBudgetIsIndependentOfTheReaperDeadline(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	h.addDevAt("old-lane", 40)
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	r := newGCRun(expired, true, &stdout, &stderr)
	opts := mustDevOptions(t, h.runtime, h.evolve, gc.Policy{})
	opts.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) { return nil, nil }
	if r.devWorktreesWith(opts) {
		t.Errorf("an expired reaper context cut the sweep; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "WOULD-ARCHIVE path=") || strings.Contains(stdout.String(), "SKIPPED") {
		t.Errorf("stdout = %q, want the tree judged under the sweep's own budget", stdout.String())
	}
}

func TestGCDevItemLine_ASkippedTreeSaysSkipped(t *testing.T) {
	t.Parallel()
	it := gc.DevItem{Path: "/p", Action: gc.DevActionKeep, Skipped: true, Reason: "not judged: x"}
	if got, want := gcDevItemLine(it, true), "SKIPPED path=/p idle=0.0d (not judged: x)"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}

func TestGCRunDevWorktrees_ABudgetThatEndsMidApplyWarnsNamesTheTreeAndIsNotAFailure(t *testing.T) {
	endSweep := endableSweep(t)
	h := newGCDevHub(t, `{}`)
	dir := h.addDevAt("old-lane", 40)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	opts := mustDevOptions(t, h.runtime, h.evolve, gc.Policy{DevWorktrees: gc.DevWorktreesPolicy{SweepBudgetSeconds: 3}})
	opts.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) { return nil, nil }
	var calls atomic.Int32
	opts.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "symbolic-ref" && strings.Contains(d, "old-lane") && calls.Add(1) > 1 {
			endSweep()
			<-ctx.Done()
			return -1, ctx.Err()
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
	if r.devWorktreesWith(opts) {
		t.Errorf("a mid-apply deferral failed gc; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "old-lane kept: the 3s budget ran out before its archive") {
		t.Errorf("stderr = %q, want a WARN naming the tree and the budget", stderr.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the deferred tree is gone: %v", err)
	}
}
