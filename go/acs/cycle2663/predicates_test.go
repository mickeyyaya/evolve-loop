//go:build acs

// Package cycle2663 holds the predicates of evolve checkpoint save --all skipping a worktree that is mid-merge.
package cycle2663

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	mergingName = "merging"
	dirtyName   = "dirty"
	cleanName   = "clean"
	badName     = "bad..name"
)

var signalLine = regexp.MustCompile(`(?m)^\[[a-z-]+\] \S+ \S+.* seq=\d+ origin=\S+ .*`)

var (
	buildOnce sync.Once
	binary    string
	buildErr  string
)

type row struct {
	Worktree string `json:"worktree"`
	Status   string `json:"status"`
	Ref      string `json:"ref"`
	Reason   string `json:"reason"`
	Error    string `json:"error"`
}

type hub struct {
	t       *testing.T
	store   *gittest.Repo
	root    string
	console string
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2663-bin-")
		if err != nil {
			buildErr = err.Error()
			return
		}
		binary = filepath.Join(dir, "evolve")
		_, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", binary, "./cmd/evolve")
		if code != 0 {
			buildErr = "go build exited " + stderr
			if runErr != nil {
				buildErr += " " + runErr.Error()
			}
		}
	})
	if buildErr != "" {
		t.Fatalf("RED: %s", buildErr)
	}
	return binary
}

func newHub(t *testing.T) hub {
	t.Helper()
	tpl := gittest.FromTemplate(t, "cycle2663-hub", func(r *gittest.Repo) error {
		if err := os.WriteFile(filepath.Join(r.Dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
			return err
		}
		r.Git("add", "-A")
		r.Git("commit", "-q", "-m", "base")
		storeDir := filepath.Join(r.Dir, "store.git")
		r.CloneBare(".", storeDir)
		r.Git("-C", storeDir, "remote", "remove", "origin")
		r.Git("-C", storeDir, "update-ref", "refs/remotes/origin/main", "main")
		return os.RemoveAll(filepath.Join(r.Dir, ".git"))
	})
	root := tpl.Dir
	tpl.Dir = filepath.Join(root, "store.git")
	h := hub{t: t, store: tpl, root: root}
	h.console = h.attach("console", "--detach")
	return h
}

func (h hub) attach(rel string, flags ...string) string {
	h.t.Helper()
	dir := filepath.Join(h.root, rel)
	h.store.Git(append(append([]string{"worktree", "add", "-q"}, flags...), dir, "main")...)
	return dir
}

func (h hub) write(dir, name, body string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h hub) enterMerge(dir string) {
	h.t.Helper()
	theirs := h.attach("aux/theirs", "-b", "feat/theirs")
	h.write(theirs, "a.txt", "theirs\n")
	h.store.Git("-C", theirs, "commit", "-qam", "theirs")
	h.write(dir, "a.txt", "ours\n")
	h.store.Git("-C", dir, "commit", "-qam", "ours")
	if _, _, code, _ := acsassert.SubprocessOutputIn(dir, "git", "-C", dir, "merge", "-q", "feat/theirs"); code != 1 {
		h.t.Fatalf("the fixture merge exit %d, want a conflict (1)", code)
	}
	if unmerged := h.store.Git("-C", dir, "ls-files", "-u"); unmerged == "" {
		h.t.Fatal("the fixture worktree has no unmerged index entry")
	}
}

func midMergeHub(t *testing.T) hub {
	t.Helper()
	h := newHub(t)
	dirty := h.attach("dev/"+dirtyName, "-b", "feat/dirty")
	h.attach("dev/"+cleanName, "-b", "feat/clean")
	merging := h.attach("dev/"+mergingName, "-b", "feat/merging")
	h.write(dirty, "a.txt", "work in progress\n")
	h.enterMerge(merging)
	return h
}

func (h hub) saveAll(args ...string) (stdout, stderr string, code int) {
	h.t.Helper()
	argv := append([]string{"checkpoint", "save", "--all", "--project-root", h.console}, args...)
	stdout, stderr, code, _ = acsassert.SubprocessOutput(evolveBinary(h.t), argv...)
	if code == -1 {
		h.t.Fatalf("evolve checkpoint did not launch: %s", stderr)
	}
	return stdout, stderr, code
}

func rowsByName(t *testing.T, stdout string) map[string]row {
	t.Helper()
	var envelope struct {
		Data []row `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil || len(envelope.Data) == 0 {
		t.Fatalf("RED: save --all --json stdout carries no data rows: %q (%v)", stdout, err)
	}
	byName := map[string]row{}
	for _, r := range envelope.Data {
		byName[r.Worktree] = r
	}
	return byName
}

func TestC2663_001_SaveAllSavesTheOtherWorktreesAndExitsZeroWhenOneIsMidMerge(t *testing.T) {
	h := midMergeHub(t)

	stdout, stderr, code := h.saveAll()

	if code != 0 {
		t.Fatalf("RED: save --all exit %d with one mid-merge worktree, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	refs := h.store.Git("for-each-ref", "--format=%(refname)", "refs/checkpoints/")
	if !strings.Contains(refs, "refs/checkpoints/"+dirtyName+"/") {
		t.Errorf("RED: the dirty worktree was not saved: refs %q", refs)
	}
	if strings.Contains(refs, "refs/checkpoints/"+mergingName+"/") {
		t.Errorf("RED: the mid-merge worktree got a checkpoint ref: %q", refs)
	}
}

func TestC2663_002_TextOutputNamesTheSkippedWorktreeAndItsReason(t *testing.T) {
	h := midMergeHub(t)

	stdout, _, _ := h.saveAll()

	var line string
	for _, l := range strings.Split(stdout, "\n") {
		if strings.Contains(l, mergingName) {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("RED: stdout never names the skipped worktree %q: %q", mergingName, stdout)
	}
	if !strings.HasPrefix(line, "skipped") {
		t.Errorf("RED: the line of the mid-merge worktree does not start with skipped: %q", line)
	}
	reason := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "skipped"), " "+mergingName))
	if !regexp.MustCompile(`(?i)merge`).MatchString(reason) {
		t.Errorf("RED: the line gives no merge reason: %q", line)
	}
	if !strings.Contains(stdout, "saved "+dirtyName+" refs/checkpoints/"+dirtyName+"/") {
		t.Errorf("RED: stdout lost the saved line of the dirty worktree: %q", stdout)
	}
}

func TestC2663_003_ASignalOnTheConsoleRecordsTheSkip(t *testing.T) {
	h := midMergeHub(t)

	_, stderr, code := h.saveAll()

	if code != 0 {
		t.Fatalf("RED: save --all exit %d, want 0\nstderr: %s", code, stderr)
	}
	var found bool
	for _, l := range signalLine.FindAllString(stderr, -1) {
		if strings.Contains(l, mergingName) {
			found = true
		}
	}
	if !found {
		t.Errorf("RED: stderr carries no signal line (seq= origin=) that names %q: %q", mergingName, stderr)
	}
	if strings.Contains(stderr, "evolve checkpoint save: "+mergingName+":") {
		t.Errorf("RED: the skip is still reported as a save error: %q", stderr)
	}
}

func TestC2663_004_JSONRowsReportTheSkipAndTheOtherStatuses(t *testing.T) {
	h := midMergeHub(t)

	stdout, stderr, code := h.saveAll("--json")

	if code != 0 {
		t.Fatalf("RED: save --all --json exit %d, want 0\nstderr: %s", code, stderr)
	}
	rows := rowsByName(t, stdout)
	if r := rows[mergingName]; r.Status != "skipped" || r.Reason == "" || r.Ref != "" || r.Error != "" {
		t.Errorf("RED: merging row = %+v, want status skipped with a reason, no ref and no error", r)
	}
	if r := rows[dirtyName]; r.Status != "saved" || r.Ref == "" {
		t.Errorf("RED: dirty row = %+v, want saved with a ref", r)
	}
	if r := rows[cleanName]; r.Status != "clean" {
		t.Errorf("RED: clean row = %+v, want clean", r)
	}
}

func TestC2663_005_ARealSaveFailureStillExitsOneBesideASkip(t *testing.T) {
	h := midMergeHub(t)
	h.attach("dev/"+badName, "-b", "feat/bad")

	stdout, _, code := h.saveAll("--json")

	if code != 1 {
		t.Errorf("RED: save --all exit %d with a worktree that cannot name a ref, want 1", code)
	}
	rows := rowsByName(t, stdout)
	if r := rows[badName]; r.Error == "" {
		t.Errorf("RED: the unnameable worktree row = %+v, want an error", r)
	}
	if r := rows[mergingName]; r.Status != "skipped" {
		t.Errorf("RED: merging row = %+v, want skipped", r)
	}
	if r := rows[dirtyName]; r.Status != "saved" {
		t.Errorf("RED: dirty row = %+v, want saved", r)
	}
}

func TestC2663_006_TheSkipEndsWhenTheMergeIsAborted(t *testing.T) {
	h := midMergeHub(t)
	merging := filepath.Join(h.root, "dev", mergingName)
	h.store.Git("-C", merging, "merge", "--abort")
	h.write(merging, "a.txt", "resumed work\n")

	stdout, stderr, code := h.saveAll("--json")

	if code != 0 {
		t.Fatalf("RED: save --all exit %d, want 0\nstderr: %s", code, stderr)
	}
	if r := rowsByName(t, stdout)[mergingName]; r.Status != "saved" || r.Ref == "" {
		t.Errorf("RED: merging row after the abort = %+v, want saved with a ref", r)
	}
}
