package builddiff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func cycleRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	repo.Write("prior/a.go", "package prior\n\nvar A = 1\n")
	repo.Write("prior/b.go", "package prior\n\nvar B = 2\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "the ship commit of the prior cycle")
	return repo, repo.Git("rev-parse", "HEAD")
}

func derive(t *testing.T, repo *gittest.Repo, base string) Summary {
	t.Helper()
	s, err := Derive(context.Background(), gitexec.Default(repo.Dir), base)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	return s
}

func TestDerive_AnchorsOnTheCycleBaseNotOnTheTipCommit(t *testing.T) {
	cases := []struct {
		name  string
		build func(repo *gittest.Repo)
		want  Summary
	}{
		{"no commit", func(*gittest.Repo) {}, Summary{}},
		{"one commit", func(repo *gittest.Repo) {
			repo.Write("x.go", "package x\n\nvar X = 1\n")
			repo.Git("add", "-A")
			repo.Git("commit", "-q", "-m", "one")
		}, Summary{Files: 1, Added: 3, Changes: []Change{{Action: "ADD", Path: "x.go"}}}},
		{"two commits and a pending change", func(repo *gittest.Repo) {
			repo.Write("x.go", "package x\n")
			repo.Git("add", "-A")
			repo.Git("commit", "-q", "-m", "one")
			repo.Write("seed.txt", "seed\nmore\n")
			repo.Git("commit", "-q", "-am", "two")
			repo.Write("prior/a.go", "package prior\n")
			repo.Write("new.md", "a\nb\n")
		}, Summary{Files: 4, Added: 4, Deleted: 2, Changes: []Change{
			{Action: "ADD", Path: "new.md"},
			{Action: "MODIFY", Path: "prior/a.go"},
			{Action: "MODIFY", Path: "seed.txt"},
			{Action: "ADD", Path: "x.go"},
		}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, base := cycleRepo(t)
			c.build(repo)
			got := derive(t, repo, base)
			if got.String() != c.want.String() {
				t.Fatalf("Derive = %s, want %s", got, c.want)
			}
			if c.name == "no commit" {
				tip := strings.Fields(repo.Git("show", "--numstat", "--format=", "HEAD"))
				if len(tip) == 0 || got.Files != 0 {
					t.Fatalf("a no-op cycle reports %d files; the tip commit (the HEAD^..HEAD shape of cycle 1479) holds %v", got.Files, tip)
				}
			}
		})
	}
}

func TestDerive_ADeletionAndARefusedGitAreReported(t *testing.T) {
	repo, base := cycleRepo(t)
	repo.Git("rm", "-q", "prior/b.go")
	if got := derive(t, repo, base); got.Files != 1 || got.Deleted != 3 || got.Changes[0] != (Change{Action: "DELETE", Path: "prior/b.go"}) {
		t.Errorf("Derive = %s, want one DELETE of 3 lines", got)
	}
	if _, err := Derive(context.Background(), gitexec.Default(repo.Dir), ""); !errors.Is(err, ErrNoBase) {
		t.Errorf("Derive with no base = %v, want ErrNoBase", err)
	}
	if _, err := Derive(context.Background(), gitexec.Default(repo.Dir), "0000000000000000000000000000000000000000"); err == nil {
		t.Error("Derive on an unknown base must fail")
	}
	for _, verb := range []string{"ls-files"} {
		gittest.FaultOn(t, verb)
		if _, err := Derive(context.Background(), gitexec.Default(repo.Dir), base); err == nil {
			t.Errorf("Derive with a failed git %s must fail", verb)
		}
	}
}

func TestDerive_EachFailedStepFailsAndAnUntrackedFileCountsItsLastLine(t *testing.T) {
	repo, base := cycleRepo(t)
	repo.Write("tail.txt", "a\nb")
	repo.Write("with space.txt", "x\n")
	if got := derive(t, repo, base); got.Added != 3 || got.Files != 2 || got.Changes[1].Path != "with space.txt" {
		t.Errorf("Derive = %s, want 3 added lines over 2 files, one of them with a space in its name", got)
	}
	if err := os.Symlink("missing", filepath.Join(repo.Dir, "dangling")); err != nil {
		t.Fatal(err)
	}
	repo.Write("blob.bin", "a\x00b\n")
	got := derive(t, repo, base)
	if got.Files != 4 || got.Added != 3 {
		t.Errorf("Derive = %s, want the dangling link and the binary file listed with no lines", got)
	}
	gittest.InstallFault(t, gittest.GitFault{Verb: "diff", Skip: 1, Code: 128})
	if _, err := Derive(context.Background(), gitexec.Default(repo.Dir), base); err == nil || !strings.Contains(err.Error(), "--name-status") {
		t.Errorf("Derive with a failed name-status = %v", err)
	}
}

func TestDerive_ATrackedPathWithASpaceOrANonASCIICharacterIsNotQuoted(t *testing.T) {
	repo, base := cycleRepo(t)
	repo.Write("caf\u00e9 menu.go", "package x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "one")
	got := derive(t, repo, base)
	if len(got.Changes) != 1 || got.Changes[0].Path != "caf\u00e9 menu.go" {
		t.Errorf("Changes = %v, want the plain path", got.Changes)
	}
}

func TestAddStatus_KeepsAnUnknownCodeAndSkipsALineWithNoPath(t *testing.T) {
	var s Summary
	s.addStatus("U\tconflicted.go\nnot a status line\n")
	if len(s.Changes) != 1 || s.Changes[0] != (Change{Action: "U", Path: "conflicted.go"}) {
		t.Errorf("Changes = %v", s.Changes)
	}
}
