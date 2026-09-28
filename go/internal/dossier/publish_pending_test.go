package dossier

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
	d := &Dossier{Cycle: 7, Goal: "pending closeout", FinalVerdict: VerdictPass,
		Phases: []PhaseRecord{{Name: "build", Verdict: VerdictPass}}}
	if err := Write(d, PendingDir(r.Dir), false); err != nil {
		t.Fatal(err)
	}
	return r
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
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
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
