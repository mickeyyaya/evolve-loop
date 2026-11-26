package ship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
)

func TestIsBookkeepingCommit_AFailedGitReadIsNotProvenance(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		failing string
		want    bool
	}{
		{failing: "", want: true},
		{failing: "git rev-list", want: false},
		{failing: "git log", want: false},
		{failing: "git diff-tree", want: false},
	} {
		t.Run("fail "+tc.failing, func(t *testing.T) {
			t.Parallel()
			r := &scriptedRunner{}
			r.runner()
			for verb, out := range map[string]string{
				"git rev-list":  "c0ffee 1a2b3c\n",
				"git log":       "dossier: cycle-9 closeout\n",
				"git diff-tree": "knowledge-base/cycles/cycle-9.json\n",
			} {
				r.scripts[verb] = struct {
					stdout string
					stderr string
					exit   int
					err    error
				}{stdout: out}
			}
			if tc.failing != "" {
				r.scripts[tc.failing] = struct {
					stdout string
					stderr string
					exit   int
					err    error
				}{err: errors.New(tc.failing + " failed")}
			}
			opts := &Options{ProjectRoot: t.TempDir(), Runner: r.runner()}
			if got := isBookkeepingCommit(context.Background(), opts, "c0ffee"); got != tc.want {
				t.Errorf("isBookkeepingCommit with %q failing = %v, want %v", tc.failing, got, tc.want)
			}
		})
	}
}

func TestIsBookkeepingCommit_AMergeIsNeverBookkeeping(t *testing.T) {
	t.Parallel()
	r := &scriptedRunner{}
	r.runner()
	set := func(verb, out string) {
		r.scripts[verb] = struct {
			stdout string
			stderr string
			exit   int
			err    error
		}{stdout: out}
	}
	set("git rev-list", "c0ffee 1a2b3c 4d5e6f\n")
	set("git log", "dossier: cycle-9 closeout\n")
	set("git diff-tree", "knowledge-base/cycles/cycle-9.json\n")
	opts := &Options{ProjectRoot: t.TempDir(), Runner: r.runner()}
	if isBookkeepingCommit(context.Background(), opts, "c0ffee") {
		t.Error("a merge commit must never count as a bookkeeping commit")
	}
}

func TestPushOnly_RefusesInjectedTaskUnderStampSubjects(t *testing.T) {
	t.Parallel()
	for _, subject := range []string{
		"chore(inbox): the loop's inbox stamps from the plane",
		"chore(inbox): the loop's quarantine moves from the plane",
	} {
		root, run := initPushOnlyRepo(t)
		sha := commitFile(t, root, run, ".evolve/inbox/injected-task.json", subject)
		_, err := pushOnlyRun(t, root)
		assertNothingPushed(t, root, run, sha, err)
	}
}

func commitFile(t *testing.T, root string, run func(string, ...string) string, rel, subject string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(subject+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "-f", rel)
	run(root, "commit", "-m", subject)
	return run(root, "rev-parse", "HEAD")
}

func TestPushOnly_PushesJournaledAndBookkeepingCommits(t *testing.T) {
	t.Parallel()
	root, run := initPushOnlyRepo(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve/inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve/inbox/item.json"), []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "-f", ".evolve/inbox/item.json")
	run(root, "commit", "-m", "seed item")
	run(root, "push", "origin", "main")
	shipped := commitFile(t, root, run, "a.txt", "ship-minted")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendShipJournal(root, shipjournal.Entry{SHA: shipped, Class: string(ClassManual)}); err != nil {
		t.Fatal(err)
	}
	commitFile(t, root, run, "knowledge-base/cycles/cycle-9.json", "dossier: cycle-9 closeout")
	run(root, "rm", "-q", "--cached", ".evolve/inbox/item.json")
	run(root, "commit", "-m", "chore(inbox): the loop's quarantine moves from the plane")
	head := run(root, "rev-parse", "HEAD")

	if _, err := pushOnlyRun(t, root); err != nil {
		t.Fatalf("push-only refused journaled + bookkeeping commits: %v", err)
	}
	if !strings.Contains(run(root, "ls-remote", "origin", "main"), head) {
		t.Error("the bookkeeping tip did not reach origin")
	}
}

func TestPushOnly_RefusesBookkeepingSubjectThatTouchesCode(t *testing.T) {
	t.Parallel()
	root, run := initPushOnlyRepo(t)
	sha := commitFile(t, root, run, "a.txt", "dossier: cycle-9 closeout")
	_, err := pushOnlyRun(t, root)
	assertNothingPushed(t, root, run, sha, err)
}

func TestPushOnly_RefusesUnknownSubjectOnBookkeepingPath(t *testing.T) {
	t.Parallel()
	root, run := initPushOnlyRepo(t)
	sha := commitFile(t, root, run, "knowledge-base/cycles/cycle-9.json", "dossier: cycle-9 whatever")
	_, err := pushOnlyRun(t, root)
	assertNothingPushed(t, root, run, sha, err)
}

func TestPushOnly_RefusesMergeCarryingDossierSubject(t *testing.T) {
	t.Parallel()
	root, run := initPushOnlyRepo(t)
	run(root, "checkout", "-q", "-b", "side")
	commitFile(t, root, run, "b.txt", "side work")
	run(root, "checkout", "-q", "main")
	commitFile(t, root, run, "knowledge-base/cycles/cycle-9.json", "dossier: cycle-9 closeout")
	run(root, "merge", "--no-ff", "-q", "-m", "dossier: cycle-9 closeout", "side")
	sha := run(root, "rev-parse", "HEAD")
	_, err := pushOnlyRun(t, root)
	assertNothingPushed(t, root, run, sha, err)
}

func assertNothingPushed(t *testing.T, root string, run func(string, ...string) string, sha string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("push-only accepted %s, want a refusal", sha[:12])
	}
	if strings.Contains(run(root, "ls-remote", "origin", "main"), sha) {
		t.Errorf("the refused commit %s reached origin", sha[:12])
	}
}
