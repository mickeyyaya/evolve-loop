package ship

// pushonly_test.go — pins for the sanctioned strand completion (inbox
// ship-push-only-recovery; the 2026-08-02 dead end: GIT_PUSH_REJECTED →
// sync-main → "nothing to ship" while ahead>0 with bare git push
// guard-denied). See pushonly.go for the provenance model.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initPushOnlyRepo builds a repo with an origin bare remote, one pushed base
// commit, and returns (projectRoot, run). All git identity is pinned.
func initPushOnlyRepo(t *testing.T) (string, func(dir string, args ...string) string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	remote := filepath.Join(base, "origin.git")
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// -b main matters on CI: without it the bare's HEAD points at the host
	// default (master when init.defaultBranch is unset), so a later clone
	// checks out an unborn branch and `push origin main` finds no ref —
	// locally masked by any global init.defaultBranch=main.
	run(base, "init", "--bare", "-b", "main", remote)
	run(base, "init", "-b", "main", root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	run(root, "commit", "-m", "base")
	run(root, "remote", "add", "origin", remote)
	run(root, "push", "-u", "origin", "main")
	return root, run
}

func pushOnlyRun(t *testing.T, root string) (RunResult, error) {
	t.Helper()
	return Run(context.Background(), Options{
		PushOnly:    true,
		ProjectRoot: root,
		Stdout:      os.Stderr,
		Stderr:      os.Stderr,
	})
}

func TestPushOnly_RefusesUnprovenancedAheadCommit(t *testing.T) {
	root, run := initPushOnlyRepo(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	run(root, "commit", "-m", "hand-made commit, no ship provenance")
	sha := run(root, "rev-parse", "HEAD")

	_, err := pushOnlyRun(t, root)
	if err == nil {
		t.Fatal("push-only pushed a commit with NO ship provenance — it just became the guard bypass")
	}
	if !strings.Contains(err.Error(), sha[:12]) {
		t.Errorf("the refusal must NAME the offending commit: %v", err)
	}
	if remoteHead := run(root, "ls-remote", "origin", "main"); strings.Contains(remoteHead, sha) {
		t.Error("the unprovenanced commit reached origin despite the refusal")
	}
}

func TestPushOnly_PushesJournaledStrand(t *testing.T) {
	root, run := initPushOnlyRepo(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	run(root, "commit", "-m", "ship-minted commit stranded by a rejected push")
	sha := run(root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendShipJournal(root, sha, ClassManual)

	res, err := pushOnlyRun(t, root)
	if err != nil {
		t.Fatalf("push-only refused a journaled strand: %v\nlogs:\n%s", err, strings.Join(res.Logs, "\n"))
	}
	if remoteHead := run(root, "ls-remote", "origin", "main"); !strings.Contains(remoteHead, sha) {
		t.Fatalf("journaled strand not pushed; remote=%s want %s", remoteHead, sha)
	}
}

func TestPushOnly_SyncMainMergeCountsAsProvenance(t *testing.T) {
	root, run := initPushOnlyRepo(t)
	// Origin moves (a console PR merge)…
	clone := filepath.Join(filepath.Dir(root), "clone")
	run(filepath.Dir(root), "clone", filepath.Join(filepath.Dir(root), "origin.git"), clone)
	if err := os.WriteFile(filepath.Join(clone, "b.txt"), []byte("console\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(clone, "add", "b.txt")
	run(clone, "commit", "-m", "console PR")
	run(clone, "push", "origin", "main")
	// …while the plane holds a journaled ship commit, then reconciles via a
	// sync-main-shaped merge (merge-only, never pushed).
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("lane\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	run(root, "commit", "-m", "stranded ship commit")
	sha := run(root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendShipJournal(root, sha, ClassManual)
	run(root, "fetch", "origin", "main")
	run(root, "merge", "--no-edit", "origin/main")

	res, err := pushOnlyRun(t, root)
	if err != nil {
		t.Fatalf("push-only refused the sync-main reconcile shape: %v\nlogs:\n%s", err, strings.Join(res.Logs, "\n"))
	}
	head := run(root, "rev-parse", "HEAD")
	if remoteHead := run(root, "ls-remote", "origin", "main"); !strings.Contains(remoteHead, head) {
		t.Fatalf("reconciled strand not pushed; remote=%s want %s", remoteHead, head)
	}
}

// A merge of two LOCAL unprovenanced branches has no origin-side parent — a
// degenerate isSyncMainMerge accepting any merge would push it (review
// MEDIUM: the negative that separates the structural check from
// "merges are free").
func TestPushOnly_LocalOnlyMergeRefuses(t *testing.T) {
	root, run := initPushOnlyRepo(t)
	run(root, "checkout", "-b", "side")
	if err := os.WriteFile(filepath.Join(root, "side.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "side.txt")
	run(root, "commit", "-m", "local side work")
	run(root, "checkout", "main")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	run(root, "commit", "-m", "local main work")
	run(root, "merge", "--no-edit", "--no-ff", "side")

	_, err := pushOnlyRun(t, root)
	if err == nil {
		t.Fatal("a merge of two LOCAL unprovenanced branches pushed — isSyncMainMerge degenerated to any-merge-counts")
	}
}

func TestPushOnly_StagedChangesRefuseAndNothingToPushIsClean(t *testing.T) {
	root, run := initPushOnlyRepo(t)
	// Nothing ahead: clean success, no mutation.
	if _, err := pushOnlyRun(t, root); err != nil {
		t.Fatalf("nothing-to-push must be a clean exit: %v", err)
	}
	// Staged work: category error.
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	if _, err := pushOnlyRun(t, root); err == nil || !strings.Contains(err.Error(), "staged") {
		t.Fatalf("staged changes must refuse with a named error, got %v", err)
	}
}

// The journal is minted by finalize's success path — the wiring that makes
// every FUTURE strand completable.
func TestFinalize_SuccessAppendsShipJournal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Class: ClassManual}
	res := RunResult{CommitSHA: "cafe0000cafe0000cafe0000cafe0000cafe0000"}
	if _, err := finalize(context.Background(), &opts, &res, nil, "test"); err != nil {
		t.Fatal(err)
	}
	if !journalHasSHA(root, res.CommitSHA) {
		t.Fatal("a successful finalize did not journal the minted commit — future strands would refuse")
	}
	// Dry-run mints nothing.
	dr := Options{ProjectRoot: root, Class: ClassManual, DryRun: true}
	res2 := RunResult{CommitSHA: "beef0000beef0000beef0000beef0000beef0000"}
	if _, err := finalize(context.Background(), &dr, &res2, nil, "test"); err != nil {
		t.Fatal(err)
	}
	if journalHasSHA(root, res2.CommitSHA) {
		t.Fatal("dry-run journaled a commit it never made")
	}
}
