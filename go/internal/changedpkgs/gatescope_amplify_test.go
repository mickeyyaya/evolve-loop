package changedpkgs

import (
	"reflect"
	"testing"
)

func TestGateScopeAmplify_TransitiveImporterIsInScope(t *testing.T) {
	repo := scopeFixture(t)
	writeSource(t, repo.Dir, "go/d/d.go", "d", "example.com/m/b")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "chain d->b->a")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	got := GateScope(repo.Dir, "main")

	want := []string{"./a/...", "./b/...", "./d/..."}
	if got.Full || !reflect.DeepEqual(got.Packages, want) {
		t.Fatalf("GateScope = %+v, want %v", got, want)
	}
}

func TestGateScopeAmplify_TwoChangedPackagesGiveOneSortedUniqueList(t *testing.T) {
	repo := scopeFixture(t)
	writeSource(t, repo.Dir, "go/c/c.go", "c", "example.com/m/a")
	writeSource(t, repo.Dir, "go/a/a.go", "a", "example.com/m/c")
	commitFile(t, repo, "go/a/extra.go", "package a\n\nfunc Extra() {}\n")

	got := GateScope(repo.Dir, "main")

	seen := map[string]bool{}
	prev := ""
	for _, p := range got.Packages {
		if seen[p] || p < prev {
			t.Fatalf("Packages = %v: want sorted and unique", got.Packages)
		}
		seen[p] = true
		prev = p
	}
	if !seen["./a/..."] {
		t.Fatalf("Packages = %v: changed package a missing", got.Packages)
	}
}

func TestGateScopeAmplify_TestFileOnlyChangeKeepsThePackageInScope(t *testing.T) {
	repo := scopeFixture(t)
	commitFile(t, repo, "go/a/a_test.go", "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n")

	got := GateScope(repo.Dir, "main")

	if got.Full || len(got.Packages) == 0 || got.Packages[0] != "./a/..." {
		t.Fatalf("GateScope = %+v, want a in scope for a test-file change", got)
	}
}

func TestGateScopeAmplify_FullScopeHasNoPackagesAndAReason(t *testing.T) {
	got := GateScope("", "")
	if !got.Full || got.Reason == "" || len(got.Packages) != 0 {
		t.Fatalf("GateScope = %+v, want Full with a reason and no packages", got)
	}
}

func TestScopeEnforcedAmplify_EdgeInputs(t *testing.T) {
	scope := Scope{Packages: []string{"./a/..."}}
	cases := map[string]struct {
		scope   Scope
		entries []string
		want    []string
	}{
		"nil entries":            {scope, nil, nil},
		"empty entries":          {scope, []string{}, nil},
		"empty string entry":     {scope, []string{""}, nil},
		"deep child kept":        {scope, []string{"./a/x/y/z"}, []string{"./a/x/y/z"}},
		"sibling prefix dropped": {scope, []string{"./a-b", "./a2"}, nil},
		"exact package kept":     {scope, []string{"./a"}, []string{"./a"}},
		"full nil entries":       {Scope{Full: true}, nil, nil},
		"empty scope nil":        {Scope{}, nil, nil},
	}
	for name, tc := range cases {
		got := tc.scope.Enforced(tc.entries)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: Enforced = %v, want %v", name, got, tc.want)
		}
	}
}

func TestScopeEnforcedAmplify_DoesNotMutateInput(t *testing.T) {
	entries := []string{"./a", "./c", "./b"}
	snapshot := append([]string(nil), entries...)

	Scope{Packages: []string{"./a/..."}}.Enforced(entries)

	if !reflect.DeepEqual(entries, snapshot) {
		t.Fatalf("entries mutated: %v, want %v", entries, snapshot)
	}
}
