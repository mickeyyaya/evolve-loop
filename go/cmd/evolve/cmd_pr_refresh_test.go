package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/researchindex"
)

const (
	signalDoc = "docs/architecture/signal-codes.md"
	darwinSet = "go/.darwin-set"
)

type fakeGenerators struct {
	root  string
	stale map[string]bool
	fail  string
}

func (f *fakeGenerators) worktree(root string) derived.Worktree {
	f.root = root
	return derived.Worktree{
		Run: f.run,
		Git: func(ctx context.Context, args ...string) (string, int, error) {
			stdout, _, code, err := gitexec.Default(root).Capture(ctx, args...)
			return stdout, code, err
		},
		Base: "HEAD",
	}
}

func (f *fakeGenerators) run(_ context.Context, args ...string) error {
	key := strings.Join(args, " ")
	if key == f.fail {
		return errors.New(key + " failed")
	}
	if slices.Equal(args[:2], []string{"docs", "index"}) {
		var out, errOut bytes.Buffer
		if docsIndexVerb.run(append(args[2:], "--project-root", f.root), &out, &errOut) != 0 {
			return errors.New(errOut.String())
		}
		return nil
	}
	output := map[string]string{"signals codes": signalDoc, "ci darwin-set": darwinSet}[strings.Join(args[:2], " ")]
	if args[len(args)-1] == "check" {
		if f.stale[output] {
			return errors.New(output + " is stale")
		}
		return nil
	}
	f.stale[output] = false
	return os.WriteFile(filepath.Join(f.root, filepath.FromSlash(output)), []byte("regenerated "+output+"\n"), 0o644)
}

func refreshRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(signalDoc, "old signals\n")
	repo.Write(darwinSet, "old set\n")
	repo.Write(researchindex.IndexPath, "# Research Index\n")
	repo.Write(researchindex.EntriesDir+"/2026-10-20-base.md", "## 2026-10-20 — base\n")
	repo.Write("README.md", "readme\n")
	repo.Write("notes.md", "notes\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo
}

func useFakeGenerators(t *testing.T, f *fakeGenerators) {
	t.Helper()
	prev := prRefreshWorktree
	prRefreshWorktree = f.worktree
	t.Cleanup(func() { prRefreshWorktree = prev })
}

func prVerb(args []string, stdout, stderr *bytes.Buffer) int { return runPR(args, nil, stdout, stderr) }

func TestPRRefresh_RegeneratesTheThreeDerivedFilesAndCommitsNothingElse(t *testing.T) {
	repo := refreshRepo(t)
	useFakeGenerators(t, &fakeGenerators{stale: map[string]bool{signalDoc: true, darwinSet: true}})
	repo.Write("README.md", "a dirty edit\n")
	repo.Write("notes.md", "a staged edit\n")
	repo.Git("add", "notes.md")

	code, stdout, stderr := runVerb(prVerb, "refresh", "--project-root", repo.Dir)

	if code != 0 || !strings.Contains(stdout, "committed") {
		t.Fatalf("refresh = %d %q %q", code, stdout, stderr)
	}
	committed := strings.Fields(repo.Git("show", "--name-only", "--format=", "HEAD"))
	want := []string{signalDoc, researchindex.IndexPath, darwinSet}
	slices.Sort(committed)
	slices.Sort(want)
	if !slices.Equal(committed, want) {
		t.Fatalf("the refresh commit holds %v, want exactly %v", committed, want)
	}
	if unstaged, staged := repo.Git("diff", "--name-only"), repo.Git("diff", "--cached", "--name-only"); unstaged != "README.md" || staged != "notes.md" {
		t.Errorf("the other edits must stay uncommitted: unstaged %q, staged %q", unstaged, staged)
	}
	if code, stdout, _ := runVerb(prVerb, "refresh", "--project-root", repo.Dir); code != 0 || !strings.Contains(stdout, "up to date") {
		t.Errorf("a second refresh = %d %q, want up to date", code, stdout)
	}
}

func TestPRRefresh_InAConflictedMergeItRegeneratesTheIndexAndStagesIt(t *testing.T) {
	repo := refreshRepo(t)
	useFakeGenerators(t, &fakeGenerators{stale: map[string]bool{}})
	for _, side := range []string{"a", "b"} {
		repo.Git("checkout", "-q", "-b", side, "main")
		repo.Write(researchindex.EntriesDir+"/2026-10-21-"+side+".md", "## 2026-10-21 — "+side+"\n")
		if _, err := researchindex.Generate(repo.Dir); err != nil {
			t.Fatal(err)
		}
		repo.Git("add", "-A")
		repo.Git("commit", "-q", "-m", side)
		repo.Git("checkout", "-q", "main")
	}
	repo.Git("merge", "-q", "--no-edit", "a")
	if exec.Command("git", "-C", repo.Dir, "merge", "-q", "--no-edit", "b").Run() == nil {
		t.Fatal("the fixture must conflict in the index")
	}

	code, stdout, stderr := runVerb(prVerb, "refresh", "--project-root", repo.Dir)

	if code != 0 || !strings.Contains(stdout, "staged") {
		t.Fatalf("refresh = %d %q %q", code, stdout, stderr)
	}
	if err := researchindex.Check(repo.Dir); err != nil {
		t.Fatalf("Check after refresh = %v", err)
	}
	if status := repo.Git("status", "--porcelain", "--", researchindex.IndexPath); !strings.HasPrefix(status, "M ") {
		t.Errorf("the index must be staged for the merge to continue: %q", status)
	}
}

func TestPRRefresh_AConflictOutsideTheGeneratedRegionIsNeverStaged(t *testing.T) {
	repo := refreshRepo(t)
	useFakeGenerators(t, &fakeGenerators{stale: map[string]bool{}})
	for _, side := range []string{"a", "b"} {
		repo.Git("checkout", "-q", "-b", side, "main")
		repo.Write(researchindex.IndexPath, "# Research Index "+side+"\n")
		if _, err := researchindex.Generate(repo.Dir); err != nil {
			t.Fatal(err)
		}
		repo.Git("commit", "-q", "-am", side)
		repo.Git("checkout", "-q", "main")
	}
	repo.Git("merge", "-q", "--no-edit", "a")
	if exec.Command("git", "-C", repo.Dir, "merge", "-q", "--no-edit", "b").Run() == nil {
		t.Fatal("the fixture must conflict in the hand-written head")
	}

	code, _, stderr := runVerb(prVerb, "refresh", "--project-root", repo.Dir)

	if code != exitIO || !strings.Contains(stderr, "leftover conflict marker") {
		t.Fatalf("refresh = %d %q, want a refusal that names the leftover marker", code, stderr)
	}
	if unmerged := repo.Git("diff", "--name-only", "--diff-filter=U"); unmerged != researchindex.IndexPath {
		t.Errorf("the index must stay unmerged: %q", unmerged)
	}
}

func TestPRRefresh_AFailedGeneratorAUsageErrorAndAGitFaultFailLoudly(t *testing.T) {
	repo := refreshRepo(t)
	useFakeGenerators(t, &fakeGenerators{stale: map[string]bool{darwinSet: true}, fail: "ci darwin-set generate"})
	if code, _, stderr := runVerb(prVerb, "refresh", "--project-root", repo.Dir); code != exitIO || !strings.Contains(stderr, "darwin-set") {
		t.Errorf("refresh with a failed generator = %d %q", code, stderr)
	}
	if code, _, stderr := runVerb(prVerb, "refresh", "extra"); code != exitUsage || !strings.Contains(stderr, prRefreshUsage) {
		t.Errorf("refresh with an operand = %d %q", code, stderr)
	}
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd: gone") }
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	if code, _, stderr := runVerb(prVerb, "refresh"); code != exitIO || !strings.Contains(stderr, "getwd: gone") {
		t.Errorf("refresh with no working directory = %d %q", code, stderr)
	}
}

func TestPRRefresh_AnEntryTheCatalogLacksFailsLoudly(t *testing.T) {
	repo := refreshRepo(t)
	useFakeGenerators(t, &fakeGenerators{stale: map[string]bool{}})
	prev := prRefreshEntries
	t.Cleanup(func() { prRefreshEntries = prev })
	prRefreshEntries = []string{"no-such-entry"}
	if code, _, stderr := runVerb(prVerb, "refresh", "--project-root", repo.Dir); code != exitIO || !strings.Contains(stderr, "no-such-entry") {
		t.Errorf("refresh with an unknown entry = %d %q", code, stderr)
	}
}

func TestPRRefresh_EachFailedGitStepFailsTheRefresh(t *testing.T) {
	for _, verb := range []string{"diff", "status", "add", "rev-parse", "commit"} {
		t.Run(verb, func(t *testing.T) {
			repo := refreshRepo(t)
			useFakeGenerators(t, &fakeGenerators{stale: map[string]bool{darwinSet: true}})
			gittest.InstallFault(t, gittest.GitFault{Verb: verb, Code: 128, Stderr: "fault " + verb})
			if code, _, stderr := runVerb(prVerb, "refresh", "--project-root", repo.Dir); code != exitIO || !strings.Contains(stderr, "git "+verb) {
				t.Errorf("refresh with a failed git %s = %d %q", verb, code, stderr)
			}
		})
	}
}
