package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// --- git fixture helpers (sm* prefix avoids collision with other main tests) ---

func smGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = smFilteredEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (dir=%s): %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// smGitAllowFail runs git without failing the test — some fixture steps
// (e.g. a merge expected to conflict) legitimately exit non-zero.
func smGitAllowFail(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = smFilteredEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func smFilteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func smInitRepo(t *testing.T, dir string) {
	t.Helper()
	smGit(t, dir, "init", "-q")
	smGit(t, dir, "config", "user.email", "ci@example.com")
	smGit(t, dir, "config", "user.name", "ci")
	smGit(t, dir, "config", "commit.gpgsign", "false")
}

// smInitRepoWithRemote creates a local repo with one committed file
// ("base.txt"), a bare "origin" remote, and pushes main so both sides start
// identical. Returns (repoDir, bareRemoteDir).
func smInitRepoWithRemote(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	smInitRepo(t, repo)
	// .evolve/ (cycle-state.json, lease files) must never make the tree
	// "dirty" in the eyes of sync-main's precondition check.
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-A")
	smGit(t, repo, "commit", "-q", "-m", "base")
	smGit(t, repo, "branch", "-M", "main")

	bare := t.TempDir()
	bare = filepath.Join(bare, "origin.git")
	smGit(t, t.TempDir(), "init", "-q", "--bare", bare)
	smGit(t, repo, "remote", "add", "origin", bare)
	smGit(t, repo, "push", "-q", "origin", "main")
	// `init --bare` leaves the bare's HEAD at the host default branch name
	// (master on unconfigured CI runners); a clone of it then checks out an
	// UNBORN branch named by the CLONER's own default, so smRemoteCommit's
	// `push origin main` dies with "src refspec main does not match any".
	// Pin the bare's HEAD so clones are hermetic regardless of git defaults.
	smGit(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	return repo, bare
}

// smRemoteCommit clones bare fresh, commits a new file, and pushes to
// origin/main — simulating a teammate's landed commit the local repo hasn't
// fetched yet.
func smRemoteCommit(t *testing.T, bare, filename, content string) {
	t.Helper()
	clone := t.TempDir()
	smGit(t, t.TempDir(), "clone", "-q", bare, clone)
	// Hermetic identity: does not rely on the host's global git config — a
	// runner without user.email would fail the commit.
	smGit(t, clone, "config", "user.email", "ci@example.com")
	smGit(t, clone, "config", "user.name", "ci")
	if err := os.WriteFile(filepath.Join(clone, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, clone, "add", "-A")
	smGit(t, clone, "commit", "-q", "-m", "remote: "+filename)
	smGit(t, clone, "push", "-q", "origin", "main")
}

func smHead(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(smGit(t, dir, "rev-parse", "HEAD"))
}

func smBareHead(t *testing.T, bare string) string {
	t.Helper()
	return strings.TrimSpace(smGit(t, bare, "rev-parse", "main"))
}

func smPorcelain(t *testing.T, dir string) string {
	t.Helper()
	return smGit(t, dir, "status", "--porcelain")
}

func smMergeHeadExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD"))
	return err == nil
}

func smWriteLiveLease(t *testing.T, repo string) {
	t.Helper()
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wsDir := t.TempDir()
	if err := runlease.Write(wsDir, runlease.Lease{RunID: "sm-live-run"}, time.Now()); err != nil {
		t.Fatalf("runlease.Write: %v", err)
	}
	cs := map[string]any{"cycle_id": 5, "workspace_path": wsDir}
	b, _ := json.Marshal(cs)
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncMain_MergesQuietDivergedTree(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")

	if err := os.WriteFile(filepath.Join(repo, "local-change.txt"), []byte("from local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-A")
	smGit(t, repo, "commit", "-q", "-m", "local change")

	bareHeadBefore := smBareHead(t, bare)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("expected exit 0 on quiet divergence, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}

	for _, f := range []string{"local-change.txt", "remote-change.txt", "base.txt"} {
		if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
			t.Errorf("expected %s present after merge: %v", f, err)
		}
	}

	parents := strings.Fields(strings.TrimSpace(smGit(t, repo, "rev-list", "--parents", "-1", "HEAD")))
	if len(parents) != 3 {
		t.Errorf("expected a merge commit (1 hash + 2 parents), got %v", parents)
	}

	if got := smBareHead(t, bare); got != bareHeadBefore {
		t.Errorf("origin main ref changed — sync-main must never push (before=%s after=%s)", bareHeadBefore, got)
	}
	if p := smPorcelain(t, repo); p != "" {
		t.Errorf("expected clean tree after merge, got:\n%s", p)
	}
}

func TestSyncMain_RefusesOnDirtyIndex(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")

	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nDIRTY\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code == 0 {
		t.Fatalf("expected refusal on dirty index, got exit 0\nstdout=%s", out.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged on refusal: before=%s after=%s", headBefore, got)
	}
	b, err := os.ReadFile(filepath.Join(repo, "base.txt"))
	if err != nil || !strings.Contains(string(b), "DIRTY") {
		t.Errorf("dirty change must survive refusal untouched: %q err=%v", b, err)
	}
	if smMergeHeadExists(repo) {
		t.Error("no merge should have been attempted on a dirty index")
	}
}

func TestSyncMain_RefusesOnLiveLease(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	smWriteLiveLease(t, repo)

	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code == 0 {
		t.Fatalf("expected refusal while a run lease is live, got exit 0\nstdout=%s", out.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged when refused for a live lease: before=%s after=%s", headBefore, got)
	}
}

func TestSyncMain_RefusesCleanlyOnConflict(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "base.txt", "line1\nremote-edit\nline3\n")

	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nlocal-edit\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-A")
	smGit(t, repo, "commit", "-q", "-m", "local conflicting edit")

	headBefore := smHead(t, repo)
	contentBefore, _ := os.ReadFile(filepath.Join(repo, "base.txt"))
	bareHeadBefore := smBareHead(t, bare)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code == 0 {
		t.Fatalf("expected refusal on a genuine conflict, got exit 0\nstdout=%s", out.String())
	}
	if smMergeHeadExists(repo) {
		t.Error("conflict must be cleanly aborted — no lingering MERGE_HEAD")
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged after a clean conflict abort: before=%s after=%s", headBefore, got)
	}
	contentAfter, _ := os.ReadFile(filepath.Join(repo, "base.txt"))
	if string(contentAfter) != string(contentBefore) {
		t.Errorf("working tree must be restored verbatim after conflict abort:\nbefore=%q\nafter=%q", contentBefore, contentAfter)
	}
	if p := smPorcelain(t, repo); p != "" {
		t.Errorf("expected clean tree after conflict abort, got:\n%s", p)
	}
	if got := smBareHead(t, bare); got != bareHeadBefore {
		t.Errorf("origin main ref must never change, even on a conflict abort: before=%s after=%s", bareHeadBefore, got)
	}
}

func TestSyncMain_AnUntrackedFileDoesNotBlockTheSync(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	item := filepath.Join(repo, "operator-inbox-item.json")
	if err := os.WriteFile(item, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb); code != 0 {
		t.Fatalf("an untracked file is not local work git's merge can lose, so it never blocks the sync; exit %d\nstderr=%s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "remote-change.txt")); err != nil {
		t.Errorf("origin's change is merged: %v", err)
	}
	if b, err := os.ReadFile(item); err != nil || string(b) != "{}\n" {
		t.Errorf("the untracked file survives the sync untouched: %q %v", b, err)
	}
	if p := smPorcelain(t, repo); p != "?? operator-inbox-item.json\n" && p != "?? operator-inbox-item.json" {
		t.Errorf("the untracked file stays untracked and the tree is otherwise clean: %q", p)
	}
}

func TestSyncMain_AnUntrackedFileTheMergeWouldOverwriteRefusesCleanly(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "incoming.txt", "from origin\n")
	local := filepath.Join(repo, "incoming.txt")
	if err := os.WriteFile(local, []byte("operator's own copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	if code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb); code == 0 {
		t.Fatalf("a merge that would overwrite an untracked file is refused\nstdout=%s", out.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD is unchanged on refusal: before=%s after=%s", headBefore, got)
	}
	if b, err := os.ReadFile(local); err != nil || string(b) != "operator's own copy\n" {
		t.Errorf("the untracked file is never overwritten: %q %v", b, err)
	}
	if smMergeHeadExists(repo) {
		t.Error("no merge is left in progress")
	}
	if msg := errb.String(); !strings.Contains(msg, "would not merge") || strings.Contains(msg, "abort failed") {
		t.Errorf("the refusal says git would not merge and quotes it, never a failed abort of a merge that never started: %s", msg)
	}
}

func TestSyncMain_ACaseCollidingUntrackedFileIsNeverOverwritten(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "incoming.txt", "from origin\n")
	local := filepath.Join(repo, "INCOMING.txt")
	if err := os.WriteFile(local, []byte("operator's own copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if b, err := os.ReadFile(local); err != nil || string(b) != "operator's own copy\n" {
		t.Fatalf("an untracked file whose name differs only in case from an incoming path keeps its content, whether git refuses the merge (a case-insensitive filesystem) or adds the path beside it (exit %d): %q %v\nstderr=%s", code, b, err, errb.String())
	}
	if smMergeHeadExists(repo) {
		t.Error("no merge is left in progress")
	}
}

func TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItems := func(dir string, items map[string]string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range items {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeItems(inbox, map[string]string{"routed.json": `{"id":"routed","weight":0.5}`, "curated.json": `{"id":"curated","weight":0.5}`, "retired.json": `{"id":"retired","weight":0.5}`})
	smGit(t, repo, "add", "-f", ".evolve/inbox")
	smGit(t, repo, "commit", "-q", "-m", "inbox")
	smGit(t, repo, "push", "-q", "origin", "main")
	console := t.TempDir()
	smGit(t, t.TempDir(), "clone", "-q", bare, console)
	smGit(t, console, "config", "user.email", "ci@example.com")
	smGit(t, console, "config", "user.name", "ci")
	writeItems(filepath.Join(console, ".evolve", "inbox"), map[string]string{"curated.json": `{"id":"curated","weight":0.9}`})
	writeItems(filepath.Join(console, ".evolve", "inbox", "consumed"), nil)
	smGit(t, console, "mv", ".evolve/inbox/retired.json", ".evolve/inbox/consumed/retired.json")
	smGit(t, console, "commit", "-q", "-am", "console curates and retires")
	smGit(t, console, "push", "-q", "origin", "main")
	writeItems(inbox, map[string]string{
		"routed.json":  `{"id":"routed","weight":0.5,"route":"console-manual"}`,
		"curated.json": `{"id":"curated","weight":0.5,"route":"console-manual"}`,
		"retired.json": `{"id":"retired","weight":0.5,"route":"console-manual"}`,
	})

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("sync-main = %d, want the loop's inbox stamps landed instead of refused\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	if got := smPorcelain(t, repo); strings.TrimSpace(got) != "" {
		t.Errorf("tree after sync = %q, want clean", got)
	}
	for name, want := range map[string]string{
		"routed.json":  `{"id":"routed","route":"console-manual","weight":0.5}`,
		"curated.json": `{"id":"curated","route":"console-manual","weight":0.9}`,
	} {
		var got, wanted map[string]any
		b, _ := os.ReadFile(filepath.Join(inbox, name))
		if json.Unmarshal(b, &got) != nil || json.Unmarshal([]byte(want), &wanted) != nil || !reflect.DeepEqual(got, wanted) {
			t.Errorf("%s = %s, want the fields of %s", name, b, want)
		}
	}
	if _, err := os.Stat(filepath.Join(inbox, "retired.json")); !os.IsNotExist(err) {
		t.Errorf("retired.json still in the root (%v), want origin's retirement applied", err)
	}
	for _, said := range []string{
		"committed 1 inbox stamp(s) the loop wrote: .evolve/inbox/routed.json",
		"replayed 1 inbox stamp(s) onto origin's edits: .evolve/inbox/curated.json",
		"dropped 1 inbox stamp(s) on items origin retired: .evolve/inbox/retired.json",
	} {
		if !strings.Contains(out.String(), said) {
			t.Errorf("stdout = %q, want it to say %q", out.String(), said)
		}
	}
}

func TestSyncMain_AnOperatorEditToAnInboxItemStillRefuses(t *testing.T) {
	repo, _ := smInitRepoWithRemote(t)
	item := filepath.Join(repo, ".evolve", "inbox", "x.json")
	if err := os.MkdirAll(filepath.Dir(item), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(item, []byte(`{"id":"x","weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-f", ".evolve/inbox")
	smGit(t, repo, "commit", "-q", "-m", "inbox")
	if err := os.WriteFile(item, []byte(`{"id":"x","weight":0.9}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code == 0 || !strings.Contains(errb.String(), ".evolve/inbox/x.json") {
		t.Errorf("sync-main = %d (stderr %q), want a refusal naming the hand-edited item: a weight change is not the mover's write", code, errb.String())
	}
	if b, _ := os.ReadFile(item); string(b) != `{"id":"x","weight":0.9}` {
		t.Errorf("x.json = %s, want the operator's edit untouched", b)
	}
}

func TestSyncMain_RefusesToMergeWhenTheStampsCannotLand(t *testing.T) {
	repo, bare, kept := smStampedPlane(t)
	replayed := filepath.Join(repo, ".evolve", "inbox", "y.json")
	if err := os.WriteFile(replayed, []byte(`{"id":"y","weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-f", ".evolve/inbox/y.json")
	smGit(t, repo, "commit", "-q", "-m", "a second item")
	smGit(t, repo, "push", "-q", "origin", "main")
	smOriginWrites(t, bare, ".evolve/inbox/y.json", `{"id":"y","weight":0.9}`)
	for _, item := range []string{kept, replayed} {
		stamped := strings.Replace(smRead(t, item), "}", `,"route":"console-manual"}`, 1)
		if err := os.WriteFile(item, []byte(stamped), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code == 0 || !strings.Contains(errb.String(), "landing the loop's inbox stamps failed") {
		t.Errorf("sync-main = %d (stderr %q), want a refusal when the stamp commit fails", code, errb.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD moved from %s to %s: no merge may follow a failed landing", headBefore, got)
	}
	for _, item := range []string{kept, replayed} {
		if got := smRead(t, item); !strings.Contains(got, "console-manual") {
			t.Errorf("%s = %s, want the stamp back in the tree, including the one prepared for a replay", filepath.Base(item), got)
		}
	}
}

func smRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func smStampedPlane(t *testing.T) (repo, bare, item string) {
	t.Helper()
	repo, bare = smInitRepoWithRemote(t)
	item = filepath.Join(repo, ".evolve", "inbox", "x.json")
	if err := os.MkdirAll(filepath.Dir(item), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(item, []byte(`{"id":"x","weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-f", ".evolve/inbox")
	smGit(t, repo, "commit", "-q", "-m", "inbox")
	smGit(t, repo, "push", "-q", "origin", "main")
	return repo, bare, item
}

func smOriginWrites(t *testing.T, bare, rel, content string) {
	t.Helper()
	console := t.TempDir()
	smGit(t, t.TempDir(), "clone", "-q", bare, console)
	smGit(t, console, "config", "user.email", "ci@example.com")
	smGit(t, console, "config", "user.name", "ci")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(console, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(console, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, console, "add", "-f", rel)
	smGit(t, console, "commit", "-q", "-m", "origin writes "+rel)
	smGit(t, console, "push", "-q", "origin", "main")
}

func TestSyncMain_ACompletedSyncWhoseStampIsAlreadyOnOriginSucceeds(t *testing.T) {
	repo, bare, item := smStampedPlane(t)
	stamped := `{"id":"x","weight":0.5,"route":"console-manual"}`
	smOriginWrites(t, bare, ".evolve/inbox/x.json", stamped)
	if err := os.WriteFile(item, []byte(stamped), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code != 0 || !strings.Contains(out.String(), "origin superseded 1 inbox stamp(s), changing every field they set: .evolve/inbox/x.json") {
		t.Errorf("sync-main = %d (stdout %q, stderr %q), want success: origin already holds the loop's stamp", code, out.String(), errb.String())
	}
}

func TestSyncMain_AConflictingMergeRestoresTheStampsItPreparedAndReportsNoneDropped(t *testing.T) {
	repo, bare, item := smStampedPlane(t)
	superseded := filepath.Join(repo, ".evolve", "inbox", "z.json")
	if err := os.WriteFile(superseded, []byte(`{"id":"z","weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-f", ".evolve/inbox/z.json")
	smGit(t, repo, "commit", "-q", "-m", "a second item")
	smGit(t, repo, "push", "-q", "origin", "main")
	smOriginWrites(t, bare, ".evolve/inbox/x.json", `{"id":"x","weight":0.9}`)
	smOriginWrites(t, bare, ".evolve/inbox/z.json", `{"id":"z","weight":0.5,"route":"lane"}`)
	smOriginWrites(t, bare, "base.txt", "line1\nORIGIN\nline3\n")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nPLANE\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "commit", "-q", "-am", "a plane-local commit that conflicts")
	for _, path := range []string{item, superseded} {
		stamped := strings.Replace(smRead(t, path), "}", `,"route":"console-manual"}`, 1)
		if err := os.WriteFile(path, []byte(stamped), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)

	if code == 0 {
		t.Fatalf("sync-main = 0, want the conflicting merge refused\nstdout=%s", out.String())
	}
	for _, path := range []string{item, superseded} {
		if got := smRead(t, path); !strings.Contains(got, "console-manual") {
			t.Errorf("%s = %s, want the loop's stamp restored after the merge was refused", filepath.Base(path), got)
		}
	}
	if strings.Contains(out.String(), "superseded") || strings.Contains(out.String(), "dropped") {
		t.Errorf("stdout = %q, want no stamp reported superseded or dropped: the refused merge restored them", out.String())
	}
}
