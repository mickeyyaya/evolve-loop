package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func floorListRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo.Dir, "go", ".apicover-enforce"), "./p\n")
	writeFile(t, filepath.Join(repo.Dir, "go", ".cover-strict"), "./p 100\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", ".apicover-enforce"), "./q")
	writeFile(t, filepath.Join(repo.Dir, "go", ".cover-strict"), "./p 10\n./q 100\n")
	return repo, base
}

func TestBaseAndTreeList_KeepsEveryEntryOfTheBaseThatTheBuildDropped(t *testing.T) {
	t.Parallel()
	repo, base := floorListRepo(t)
	in := ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}
	moduleDir := filepath.Join(repo.Dir, "go")

	enforce := string(baseAndTreeList(context.Background(), in, moduleDir, ".apicover-enforce"))
	strict := string(baseAndTreeList(context.Background(), in, moduleDir, ".cover-strict"))

	if lines := strings.Fields(enforce); !reflect.DeepEqual(lines, []string{"./q", "./p"}) {
		t.Errorf("apicover list lines = %q, want the build's ./q and the base ./p on their own lines, even when the tree list has no final newline", lines)
	}
	if !strings.Contains(strict, "./p 100") || !strings.Contains(strict, "./p 10\n") || !strings.Contains(strict, "./q 100") {
		t.Errorf("cover-strict list = %q, want the base floor ./p 100 beside the build's lowered ./p 10", strict)
	}
}

func TestBaseAndTreeList_AListNewInTheBuildIsReadFromTheTree(t *testing.T) {
	t.Parallel()
	repo, base := floorListRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", ".new-list"), "./r\n")

	got := string(baseAndTreeList(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}, filepath.Join(repo.Dir, "go"), ".new-list"))

	if strings.TrimSpace(got) != "./r" {
		t.Errorf("list = %q, want only the tree's ./r: the base has no such list", got)
	}
}

func TestBaseAndTreeList_AnUnreadableBaseWarnsAndKeepsTheTree(t *testing.T) {
	repo, _ := floorListRepo(t)
	var got string
	stderr := captureStderr(t, func() {
		got = string(baseAndTreeList(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: "0000000000000000000000000000000000000000"}, filepath.Join(repo.Dir, "go"), ".apicover-enforce"))
	})

	if strings.TrimSpace(got) != "./q" {
		t.Errorf("list = %q, want the tree's ./q when the base cannot be read", got)
	}
	if !strings.Contains(stderr, "WARN") || !strings.Contains(stderr, ".apicover-enforce") {
		t.Errorf("stderr = %q, want a WARN that names the list", stderr)
	}
}

func TestEnforcedPackages_ABuildCannotDropAPackageFromTheBaseList(t *testing.T) {
	t.Parallel()
	repo, base := floorListRepo(t)
	in := ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}

	got := enforcedPackages(context.Background(), in, filepath.Join(repo.Dir, "go"), []string{"./p", "./q", "./s"})

	if want := map[string]bool{"./p": true, "./q": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("enforced = %v, want %v: the base still enforces ./p", got, want)
	}
}

func TestBaseAndTreeList_AnUnreadableTreeListWarnsAndKeepsTheBase(t *testing.T) {
	repo, base := floorListRepo(t)
	moduleDir := filepath.Join(repo.Dir, "go")
	if err := os.Remove(filepath.Join(moduleDir, ".cover-strict")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(moduleDir, ".cover-strict"), 0o755); err != nil {
		t.Fatal(err)
	}
	var got string
	stderr := captureStderr(t, func() {
		got = string(baseAndTreeList(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}, moduleDir, ".cover-strict"))
	})

	if strings.TrimSpace(got) != "./p 100" || !strings.Contains(stderr, "WARN read .cover-strict") {
		t.Errorf("list = %q, stderr = %q, want the base floor and a WARN for the unreadable tree list", got, stderr)
	}
}

func TestListAtBase_APathOutsideTheWorktreeIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := listAtBase(context.Background(), "", "HEAD", "/abs/go/.cover-strict"); err == nil {
		t.Fatal("a relative worktree against an absolute list path must not resolve")
	}
}

func TestChangedPackageFloorChecks_APackageTheBaseEnforcesStaysEnforcedAfterTheBuildDropsIt(t *testing.T) {
	repo, base := floorListRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a_test.go"), "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { _ = F() }\n")
	var plain []string
	prev := buildSelfCheckRunner
	buildSelfCheckRunner = func(_ context.Context, _, pkg string) (string, bool) {
		plain = append(plain, pkg)
		return "", true
	}
	t.Cleanup(func() { buildSelfCheckRunner = prev })
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: repo.Dir, WorktreeBaseSHA: base}

	out := changedPackageFloorChecks(context.Background(), in, []string{"go/p/a.go", "go/p/a_test.go"})

	if len(plain) != 0 {
		t.Errorf("plain self-check ran %v: ./p is enforced at the base, so the build's edit of the list must not demote it", plain)
	}
	if strings.Contains(strings.Join(out, "\n"), "unit tests FAIL") {
		t.Errorf("floor = %v, want no unit failure for the green package", out)
	}
}

func TestCoverChangedFailures_TheStrictFloorsOfTheBaseStillBindAfterTheBuildLowersThem(t *testing.T) {
	repo, base := floorListRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc F(x int) int {\n\treturn x\n}\n")
	stubCoverChangedProfiler(t, fullyCoveredChangedFProfile, "", coverStatusOK)
	var floors string
	orig := coverStrictRunner
	coverStrictRunner = func(_, _ string, _ []string, data []byte) ([]coverstrict.Result, error) {
		floors = string(data)
		return nil, nil
	}
	t.Cleanup(func() { coverStrictRunner = orig })

	coverChangedFailures(context.Background(), floorInput(t, floorRoot(t, `{"cover_changed":{"stage":"enforce"}}`), repo, base), []string{"go/p/a.go"})

	if !strings.Contains(floors, "./p 100") {
		t.Errorf("strict floors = %q, want the base floor ./p 100 that the build lowered to 10", floors)
	}
}

func TestBaseAndTreeList_AnUnchangedListKeepsEachLineOnce(t *testing.T) {
	t.Parallel()
	repo, base := floorListRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", ".cover-strict"), "./p 100\n")

	got := string(baseAndTreeList(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}, filepath.Join(repo.Dir, "go"), ".cover-strict"))

	if got != "./p 100\n" {
		t.Errorf("list = %q, want the one floor once: a duplicate floor reports each failure twice", got)
	}
}

func TestListAtBase_AListOutsideTheWorktreeIsAnErrorNotAnAbsentList(t *testing.T) {
	t.Parallel()
	repo, base := floorListRepo(t)

	_, err := listAtBase(context.Background(), repo.Dir, base, filepath.Join(filepath.Dir(repo.Dir), "elsewhere", ".cover-strict"))

	if err == nil || !strings.Contains(err.Error(), "outside the worktree") {
		t.Fatalf("err = %v, want a refusal: a path outside the worktree must not read as absent at the base", err)
	}
}
