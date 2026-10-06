package dossier

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func planeWithPendingDossier(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(r.Dir, "README.md"), []byte("plane\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	if err := Write(pendingCloseout(), PendingDir(r.Dir), false); err != nil {
		t.Fatal(err)
	}
	return r
}

func pendingCloseout() *Dossier {
	return &Dossier{Cycle: 7, Goal: "pending closeout", FinalVerdict: VerdictPass,
		Phases: []PhaseRecord{{Name: "build", Verdict: VerdictPass}}}
}

func corpusFile(root, name string) string { return filepath.Join(CyclesDir(root), name) }

func TestPublishPending_PublishesExactlyWhatWriteProducesAndCommitsIt(t *testing.T) {
	r := planeWithPendingDossier(t)

	res, err := PublishPending(r.Dir, io.Discard)

	if want := (PublishResult{Published: []int{7}, Failed: map[int]error{}}); err != nil || !reflect.DeepEqual(res, want) {
		t.Fatalf("PublishPending = (%+v, %v), want only cycle 7 published", res, err)
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != "dossier: cycle-7 closeout\n\nknowledge-base/cycles/cycle-7.json\nknowledge-base/cycles/cycle-7.md" {
		t.Fatalf("HEAD = %q, want one commit of cycle 7's closeout", got)
	}
	if entries, _ := os.ReadDir(PendingDir(r.Dir)); len(entries) != 0 {
		t.Fatalf("published pair still pending: %v", entries)
	}
}

func TestPublishPending_WithNoPendingDirPublishesNothing(t *testing.T) {
	res, err := PublishPending(t.TempDir(), io.Discard)

	if want := (PublishResult{Failed: map[int]error{}}); err != nil || !reflect.DeepEqual(res, want) {
		t.Fatalf("PublishPending = (%+v, %v), want an empty pass", res, err)
	}
}

func TestPublishPending_RejectsAPairWriteDidNotProduce(t *testing.T) {
	cases := map[string]func(root string){
		"a tampered markdown half": func(root string) {
			appendTo(t, filepath.Join(PendingDir(root), "cycle-7.md"), "\nplanted instruction\n")
		},
		"a json half that is not Write's rendering": func(root string) {
			appendTo(t, filepath.Join(PendingDir(root), "cycle-7.json"), " ")
		},
		"a pair filed under another cycle's name": func(root string) {
			for _, ext := range []string{".json", ".md"} {
				if err := os.Rename(filepath.Join(PendingDir(root), "cycle-7"+ext), filepath.Join(PendingDir(root), "cycle-8"+ext)); err != nil {
					t.Fatal(err)
				}
			}
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			r := planeWithPendingDossier(t)
			head := r.Git("rev-parse", "HEAD")
			tamper(r.Dir)

			res, err := PublishPending(r.Dir, io.Discard)

			if err != nil || len(res.Published) != 0 || len(res.Failed) != 1 {
				t.Fatalf("PublishPending = (%+v, %v), want the pair refused", res, err)
			}
			if after := r.Git("rev-parse", "HEAD"); after != head {
				t.Fatal("a refused pair was committed")
			}
			if _, err := os.Stat(CyclesDir(r.Dir)); !os.IsNotExist(err) {
				t.Fatalf("a refused pair reached the corpus: %v", err)
			}
		})
	}
}

func TestPublishPending_CompletesAJSONOnlyPairFromItsJSON(t *testing.T) {
	r := planeWithPendingDossier(t)
	wantMD, err := os.ReadFile(filepath.Join(PendingDir(r.Dir), "cycle-7.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(PendingDir(r.Dir), "cycle-7.md")); err != nil {
		t.Fatal(err)
	}

	res, err := PublishPending(r.Dir, io.Discard)

	if err != nil || !reflect.DeepEqual(res.Published, []int{7}) {
		t.Fatalf("PublishPending = (%+v, %v), want the JSON-only pair completed and published", res, err)
	}
	if md, err := os.ReadFile(corpusFile(r.Dir, "cycle-7.md")); err != nil || string(md) != string(wantMD) {
		t.Fatalf("the rendered markdown is not what Write would have written: %v", err)
	}
}

func TestPublishPending_LeavesAMarkdownOnlyHalfPending(t *testing.T) {
	r := planeWithPendingDossier(t)
	if err := os.Remove(filepath.Join(PendingDir(r.Dir), "cycle-7.json")); err != nil {
		t.Fatal(err)
	}

	res, err := PublishPending(r.Dir, io.Discard)

	if err != nil || !reflect.DeepEqual(res.Skipped, []int{7}) || len(res.Published) != 0 {
		t.Fatalf("PublishPending = (%+v, %v), want the markdown-only half skipped", res, err)
	}
	if _, err := os.Stat(filepath.Join(PendingDir(r.Dir), "cycle-7.md")); err != nil {
		t.Fatalf("the half left the pending dir: %v", err)
	}
}

func TestPublishPending_AFailedCommitKeepsThePairPendingAndTheCorpusClean(t *testing.T) {
	r := planeWithPendingDossier(t)
	hooks := filepath.Join(r.Dir, ".git", "hooks")
	r.Git("config", "core.hooksPath", hooks)
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeclitest.Install(t, filepath.Join(hooks, "pre-commit"), "#!/bin/sh\nexit 1\n")
	head := r.Git("rev-parse", "HEAD")

	res, err := PublishPending(r.Dir, io.Discard)

	if err != nil || len(res.Failed) != 1 || len(res.Published) != 0 {
		t.Fatalf("PublishPending = (%+v, %v), want the commit failure recorded", res, err)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Fatal("a failed publish moved HEAD")
	}
	if _, err := os.Stat(corpusFile(r.Dir, "cycle-7.json")); !os.IsNotExist(err) {
		t.Fatalf("an uncommitted pair was left in the corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(PendingDir(r.Dir), "cycle-7.md")); err != nil {
		t.Fatalf("the pair left the pending dir although it was not committed: %v", err)
	}
}

func TestPublishPending_NeverOverwritesARecordTheCorpusAlreadyHolds(t *testing.T) {
	earlier := &Dossier{Cycle: 7, Goal: "an earlier record", FinalVerdict: VerdictWarn,
		Phases: []PhaseRecord{{Name: "build", Verdict: VerdictWarn}}}
	cases := map[string]func(t *testing.T, root string){
		"a different committed record": func(t *testing.T, root string) {
			if err := Write(earlier, CyclesDir(root), true); err != nil {
				t.Fatal(err)
			}
		},
		"half of the same record": func(t *testing.T, root string) {
			if err := Write(pendingCloseout(), CyclesDir(root), false); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(corpusFile(root, "cycle-7.md")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, seed := range cases {
		t.Run(name, func(t *testing.T) {
			r := planeWithPendingDossier(t)
			seed(t, r.Dir)
			head := r.Git("rev-parse", "HEAD")
			before := corpusBytes(t, r.Dir)

			res, err := PublishPending(r.Dir, io.Discard)

			if err != nil || len(res.Published) != 0 || res.Failed[7] == nil || !strings.Contains(res.Failed[7].Error(), "corpus") {
				t.Fatalf("PublishPending = (%+v, %v), want cycle 7 refused because the corpus holds it", res, err)
			}
			if r.Git("rev-parse", "HEAD") != head {
				t.Fatal("a refused pair moved HEAD")
			}
			if after := corpusBytes(t, r.Dir); !reflect.DeepEqual(after, before) {
				t.Fatalf("the corpus changed: %q, was %q", after, before)
			}
			if _, err := os.Stat(filepath.Join(PendingDir(r.Dir), "cycle-7.json")); err != nil {
				t.Fatalf("a refused pair must stay pending: %v", err)
			}
		})
	}
}

func TestPublishPending_ClearsAPendingCopyTheCorpusAlreadyHoldsWithoutACommit(t *testing.T) {
	r := planeWithPendingDossier(t)
	if _, err := PublishPending(r.Dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := Write(pendingCloseout(), PendingDir(r.Dir), false); err != nil {
		t.Fatal(err)
	}
	head := r.Git("rev-parse", "HEAD")
	committed, err := os.Stat(corpusFile(r.Dir, "cycle-7.json"))
	if err != nil {
		t.Fatal(err)
	}

	res, err := PublishPending(r.Dir, io.Discard)

	if err != nil || !reflect.DeepEqual(res.Published, []int{7}) || len(res.Failed) != 0 {
		t.Fatalf("PublishPending = (%+v, %v), want the leftover copy cleared", res, err)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Fatal("clearing a copy the corpus already holds must not commit")
	}
	if now, err := os.Stat(corpusFile(r.Dir, "cycle-7.json")); err != nil || !os.SameFile(committed, now) {
		t.Fatalf("the corpus record was rewritten: %v", err)
	}
	if entries, _ := os.ReadDir(PendingDir(r.Dir)); len(entries) != 0 {
		t.Fatalf("the leftover copy is still pending: %v", entries)
	}
}

func TestPublishPending_CommitsAnIdenticalRecordTheCorpusHoldsUntracked(t *testing.T) {
	r := planeWithPendingDossier(t)
	if err := Write(pendingCloseout(), CyclesDir(r.Dir), false); err != nil {
		t.Fatal(err)
	}

	res, err := PublishPending(r.Dir, io.Discard)

	if err != nil || !reflect.DeepEqual(res.Published, []int{7}) || len(res.Failed) != 0 {
		t.Fatalf("PublishPending = (%+v, %v), want cycle 7 published", res, err)
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != "dossier: cycle-7 closeout\n\nknowledge-base/cycles/cycle-7.json\nknowledge-base/cycles/cycle-7.md" {
		t.Fatalf("HEAD = %q, want the untracked identical record committed", got)
	}
	if entries, _ := os.ReadDir(PendingDir(r.Dir)); len(entries) != 0 {
		t.Fatalf("the pending copy is still pending: %v", entries)
	}
}

func TestPublishPending_RefusesAPendingHalfThatIsNotARegularFile(t *testing.T) {
	for _, name := range []string{"cycle-7.json", "cycle-7.md"} {
		t.Run(name, func(t *testing.T) {
			r := planeWithPendingDossier(t)
			pending := filepath.Join(PendingDir(r.Dir), name)
			elsewhere := filepath.Join(t.TempDir(), name)
			if err := os.Rename(pending, elsewhere); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, pending); err != nil {
				t.Fatal(err)
			}
			head := r.Git("rev-parse", "HEAD")

			res, err := PublishPending(r.Dir, io.Discard)

			if err != nil || len(res.Published) != 0 || res.Failed[7] == nil || !strings.Contains(res.Failed[7].Error(), "not a regular file") {
				t.Fatalf("PublishPending = (%+v, %v), want the symlinked half refused", res, err)
			}
			if r.Git("rev-parse", "HEAD") != head {
				t.Fatal("a refused pair moved HEAD")
			}
			if _, err := os.Stat(CyclesDir(r.Dir)); !os.IsNotExist(err) {
				t.Fatalf("a refused pair reached the corpus: %v", err)
			}
			if _, err := os.Lstat(pending); err != nil {
				t.Fatalf("a refused pair must stay pending: %v", err)
			}
		})
	}
}

func corpusBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range []string{"cycle-7.json", "cycle-7.md"} {
		if b, err := os.ReadFile(corpusFile(root, name)); err == nil {
			out[name] = string(b)
		}
	}
	return out
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
