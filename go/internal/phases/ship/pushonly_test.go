package ship

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

// Git identity is pinned for deterministic commits.
func initPushOnlyRepo(t *testing.T) (string, func(dir string, args ...string) string) {
	t.Helper()
	remote := gittest.Bare(t).Dir
	root := gittest.Fixture(t).Dir
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
	if err := appendShipJournal(root, shipJournalEntry{SHA: sha, Class: string(ClassManual)}); err != nil {
		t.Fatal(err)
	}

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
	clone := gittest.Clone(t, run(root, "remote", "get-url", "origin")).Dir
	if err := os.WriteFile(filepath.Join(clone, "b.txt"), []byte("console\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(clone, "add", "b.txt")
	run(clone, "commit", "-m", "console PR")
	run(clone, "push", "origin", "main")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("lane\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	run(root, "commit", "-m", "stranded ship commit")
	sha := run(root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendShipJournal(root, shipJournalEntry{SHA: sha, Class: string(ClassManual)}); err != nil {
		t.Fatal(err)
	}
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

// A merge of two LOCAL unprovenanced branches has no origin-side parent; a
// degenerate isSyncMainMerge accepting any merge would push it.
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
	if _, err := pushOnlyRun(t, root); err != nil {
		t.Fatalf("nothing-to-push must be a clean exit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "a.txt")
	if _, err := pushOnlyRun(t, root); err == nil || !strings.Contains(err.Error(), "staged") {
		t.Fatalf("staged changes must refuse with a named error, got %v", err)
	}
}

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
	dr := Options{ProjectRoot: root, Class: ClassManual, DryRun: true}
	res2 := RunResult{CommitSHA: "beef0000beef0000beef0000beef0000beef0000"}
	if _, err := finalize(context.Background(), &dr, &res2, nil, "test"); err != nil {
		t.Fatal(err)
	}
	if journalHasSHA(root, res2.CommitSHA) {
		t.Fatal("dry-run journaled a commit it never made")
	}
}

func TestFinalize_MintedCommitIsJournaledEvenWhenThePushFailed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Class: ClassCycle}
	res := RunResult{CommitSHA: "38d3a41100000000000000000000000000001678"}
	rejected := errors.New("ship: push rejected and origin/main diverged — local commit preserved")
	if _, err := finalize(context.Background(), &opts, &res, rejected, "test"); err == nil {
		t.Fatal("the push failure must still be returned")
	}
	if !journalHasSHA(root, res.CommitSHA) {
		t.Fatal("a minted commit whose push was rejected was not journaled — push-only would refuse the strand it exists to complete")
	}
	none := RunResult{}
	if _, err := finalize(context.Background(), &opts, &none, errors.New("gate red"), "test"); err == nil {
		t.Fatal("expected the error back")
	}
	raw, _ := os.ReadFile(shipJournalPath(root))
	if strings.Contains(string(raw), `"sha":""`) {
		t.Fatalf("an empty SHA was journaled:\n%s", raw)
	}
}
