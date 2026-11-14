package changedpkgs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func scopeFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	writeSource(t, repo.Dir, "go/a/a.go", "a")
	writeSource(t, repo.Dir, "go/b/b.go", "b", "example.com/m/a")
	writeSource(t, repo.Dir, "go/c/c.go", "c")
	writeSource(t, repo.Dir, "go/acs/cycle1/p_test.go", "cycle1", "example.com/m/a")
	if err := os.WriteFile(filepath.Join(repo.Dir, "go", "go.mod"), []byte("module example.com/m\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo.Dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "docs", "x.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	repo.Git("checkout", "-q", "-b", "lane")
	return repo
}

func commitFile(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "lane change")
}

func TestGateScope_OnePackageChangeRunsThatPackageAndItsImporters(t *testing.T) {
	repo := scopeFixture(t)
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	got := GateScope(repo.Dir, "main")

	want := []string{"./a/...", "./b/..."}
	if got.Full || !reflect.DeepEqual(got.Packages, want) {
		t.Fatalf("GateScope = %+v, want Packages %v and Full false: c does not import a and must stay out", got, want)
	}
}

func TestGateScope_UncommittedChangeCountsLikeACommittedOne(t *testing.T) {
	repo := scopeFixture(t)
	writeSource(t, repo.Dir, "go/c/c.go", "c", "example.com/m/a")

	got := GateScope(repo.Dir, "main")

	if got.Full || !reflect.DeepEqual(got.Packages, []string{"./c/..."}) {
		t.Fatalf("GateScope = %+v, want only ./c/... (nothing imports c)", got)
	}
}

func TestGateScope_ModuleRootChangeFallsBackToTheFullSet(t *testing.T) {
	repo := scopeFixture(t)
	commitFile(t, repo, "go/go.mod", "module example.com/m\n\ngo 1.23\n\n// bumped\n")

	got := GateScope(repo.Dir, "main")

	if !got.Full || got.Reason == "" || len(got.Packages) != 0 {
		t.Fatalf("GateScope = %+v, want Full with a reason and no package list for a go.mod change", got)
	}
}

func TestGateScope_ChangeOutsideGoRunsNothing(t *testing.T) {
	repo := scopeFixture(t)
	commitFile(t, repo, "docs/x.md", "changed\n")

	got := GateScope(repo.Dir, "main")

	if got.Full || len(got.Packages) != 0 {
		t.Fatalf("GateScope = %+v, want an empty non-full scope for a docs-only change", got)
	}
}

func TestGateScope_UnderivableChangeFallsBackToTheFullSet(t *testing.T) {
	for name, tc := range map[string]struct{ dir, ref string }{
		"not a git repository": {t.TempDir(), "main"},
		"unknown base ref":     {scopeFixture(t).Dir, "no-such-ref"},
		"empty repo root":      {"", "main"},
	} {
		got := GateScope(tc.dir, tc.ref)
		if !got.Full || got.Reason == "" {
			t.Errorf("%s: GateScope = %+v, want Full with a reason", name, got)
		}
	}
}

func TestGateScope_UnreadableImportGraphFallsBackToTheFullSet(t *testing.T) {
	repo := gittest.Fixture(t)
	writeSource(t, repo.Dir, "go/a/a.go", "a")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline without go.mod")
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	got := GateScope(repo.Dir, "main")

	if !got.Full || got.Reason == "" {
		t.Fatalf("GateScope = %+v, want Full with a reason when go list cannot read the module", got)
	}
}

func TestGateScope_ACSPackagesNeverEnterTheScope(t *testing.T) {
	repo := scopeFixture(t)
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	got := GateScope(repo.Dir, "main")

	for _, p := range got.Packages {
		if strings.Contains(p, "/acs/") || strings.HasPrefix(p, "./acs") {
			t.Fatalf("GateScope.Packages = %v holds %s: acs packages stay out of the test scope", got.Packages, p)
		}
	}
	if len(got.Packages) == 0 {
		t.Fatalf("GateScope = %+v, want the a and b packages so the acs filter is observable", got)
	}
}

func TestGateScope_AcsOnlyChangeRunsNothing(t *testing.T) {
	repo := scopeFixture(t)
	commitFile(t, repo, "go/acs/cycle1/p_test.go", "package cycle1\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) {}\n")

	got := GateScope(repo.Dir, "main")

	if got.Full || len(got.Packages) != 0 {
		t.Fatalf("GateScope = %+v, want an empty non-full scope when only an acs package changed", got)
	}
}

func TestScopeEnforced_KeepsEntriesInsideTheScope(t *testing.T) {
	scope := Scope{Packages: []string{"./a/...", "./b/..."}}

	got := scope.Enforced([]string{"./a", "./b/sub", "./c", "./ab", "./acs/cycle1"})

	if want := []string{"./a", "./b/sub"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Enforced = %v, want %v: ./ab shares a prefix with ./a but is a different package", got, want)
	}
}

func TestScopeEnforced_FullScopeKeepsEveryEntry(t *testing.T) {
	entries := []string{"./a", "./c"}

	got := Scope{Full: true}.Enforced(entries)

	if !reflect.DeepEqual(got, entries) {
		t.Fatalf("Enforced = %v, want %v for a full scope", got, entries)
	}
}

func TestScopeEnforced_EmptyScopeKeepsNothing(t *testing.T) {
	if got := (Scope{}).Enforced([]string{"./a"}); len(got) != 0 {
		t.Fatalf("Enforced = %v, want none for an empty scope", got)
	}
}
