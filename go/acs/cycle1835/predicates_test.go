//go:build acs

package cycle1835

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const dossierRel = "knowledge-base/cycles"

func putFile(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoWithBaseCommit(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	putFile(t, r.Dir, "README.md")
	r.Git("add", "--", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func TestC1835_001_SameCycleFilesInDifferentDirectoriesAreNeverOnePair(t *testing.T) {
	r := repoWithBaseCommit(t)
	putFile(t, r.Dir, dossierRel+"/cycle-7.json")
	putFile(t, r.Dir, "elsewhere/cycle-7.md")
	headBefore := r.Git("rev-parse", "HEAD")

	res, err := dossier.SweepOrphans(gitexec.Default(r.Dir), io.Discard)
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if len(res.Recommitted) != 0 {
		t.Errorf("Recommitted = %v, want none for files in different directories", res.Recommitted)
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %v: files in different directories were paired into one commit attempt", res.Failed)
	}
	if head := r.Git("rev-parse", "HEAD"); head != headBefore {
		t.Errorf("HEAD moved %s -> %s: a cross-directory pair was committed", headBefore, head)
	}
}

func TestC1835_002_OnlyTheDossierDirectoryIsSwept(t *testing.T) {
	r := repoWithBaseCommit(t)
	putFile(t, r.Dir, "stray/cycle-8.json")
	putFile(t, r.Dir, "stray/cycle-8.md")
	putFile(t, r.Dir, dossierRel+"/cycle-9.json")
	putFile(t, r.Dir, dossierRel+"/cycle-9.md")

	res, err := dossier.SweepOrphans(gitexec.Default(r.Dir), io.Discard)
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if !reflect.DeepEqual(res.Recommitted, []int{9}) {
		t.Errorf("Recommitted = %v, want only the dossier-directory pair [9]", res.Recommitted)
	}
	if tracked := r.Git("ls-files", "--", "stray"); tracked != "" {
		t.Errorf("pair outside the dossier directory was committed: %q", tracked)
	}
	if tracked := r.Git("ls-files", "--", dossierRel); tracked == "" {
		t.Errorf("complete pair under %s was not committed", dossierRel)
	}
}

func TestC1835_003_TrackedModifiedPairIsNotRecommitted(t *testing.T) {
	r := repoWithBaseCommit(t)
	putFile(t, r.Dir, dossierRel+"/cycle-11.json")
	putFile(t, r.Dir, dossierRel+"/cycle-11.md")
	r.Git("add", "--", dossierRel)
	r.Git("commit", "-q", "-m", "tracked dossier")
	headBefore := r.Git("rev-parse", "HEAD")
	for _, name := range []string{"cycle-11.json", "cycle-11.md"} {
		if err := os.WriteFile(filepath.Join(r.Dir, filepath.FromSlash(dossierRel), name), []byte("changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	res, err := dossier.SweepOrphans(gitexec.Default(r.Dir), io.Discard)
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if len(res.Recommitted) != 0 {
		t.Errorf("Recommitted = %v, want none for a tracked modified pair", res.Recommitted)
	}
	if head := r.Git("rev-parse", "HEAD"); head != headBefore {
		t.Errorf("HEAD moved %s -> %s: a tracked modification was swept", headBefore, head)
	}
}

func TestC1835_004_BuildOptsHasNoDeadLedgerPath(t *testing.T) {
	if _, ok := reflect.TypeOf(dossier.BuildOpts{}).FieldByName("LedgerPath"); ok {
		t.Error("BuildOpts.LedgerPath is dead (no caller sets or reads it) and must be removed")
	}
}

func TestC1835_005_WriteNilDossierReturnsErrorWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	if err := dossier.Write(nil, dir, false); err == nil {
		t.Error("Write(nil, ...) returned nil error, want an error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("Write(nil, ...) left %d file(s) in dir", len(entries))
	}
}
