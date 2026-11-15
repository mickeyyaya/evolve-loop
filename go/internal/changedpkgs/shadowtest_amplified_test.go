package changedpkgs

import (
	"context"
	"io"
	"reflect"
	"sort"
	"testing"
)

func TestRunShadowTests_FiltersOutAcsPackages(t *testing.T) {
	repo := shadowFixture(t)
	commitFile(t, repo, "go/acs/cycle9999/pred_test.go", "package cycle9999\n\nimport \"testing\"\n\nfunc TestPred(t *testing.T) { t.Fatal(\"must not run\") }\n")
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/acs/cycle9999/pred_test.go", "package cycle9999\n\nimport \"testing\"\n\nfunc TestPred(t *testing.T) { t.Fatal(\"still must not run\") }\n")

	called := false
	fake := func(_ context.Context, name, _ string, _ []string, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if name == "go" {
			called = true
		}
		return 0, nil
	}

	res, err := RunShadowTests(context.Background(), repo.Dir, "main", fake)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	if called {
		t.Errorf("go test was invoked when only acs package changed")
	}
	if len(res.Selected) != 0 || !res.Pass {
		t.Errorf("got %+v, want empty Selected and Pass true", res)
	}
}

func TestRunShadowTests_ExcludesAcsWhenMixedWithRegularPackageChange(t *testing.T) {
	repo := shadowFixture(t)
	commitFile(t, repo, "go/acs/cycle9999/pred_test.go", "package cycle9999\n\nimport \"testing\"\n\nfunc TestPred(t *testing.T) { t.Fatal(\"must not run\") }\n")
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	commitFile(t, repo, "go/acs/cycle9999/pred_test.go", "package cycle9999\n\nimport \"testing\"\n\nfunc TestPred(t *testing.T) { t.Fatal(\"changed acs\") }\n")

	res, err := RunShadowTests(context.Background(), repo.Dir, "main", nil)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	sort.Strings(res.Selected)
	want := []string{"./a/...", "./b/...", "./d/..."}
	if !reflect.DeepEqual(res.Selected, want) {
		t.Errorf("Selected = %v, want %v", res.Selected, want)
	}
	if !res.Pass {
		t.Errorf("Pass = false, want true; output: %s", res.Output)
	}
}

func TestRunShadowTests_CustomBaseRefAndDiamondDeduplication(t *testing.T) {
	repo := shadowFixture(t)
	commitFile(t, repo, "go/e/e.go", "package e\n\nimport _ \"example.com/m/a\"\n")
	commitFile(t, repo, "go/e/e_test.go", "package e\n\nimport \"testing\"\n\nfunc TestE(t *testing.T) {}\n")
	commitFile(t, repo, "go/d/d.go", "package d\n\nimport (\n\t_ \"example.com/m/b\"\n\t_ \"example.com/m/e\"\n)\n")
	repo.Git("branch", "release-v1")

	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	res, err := RunShadowTests(context.Background(), repo.Dir, "release-v1", nil)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	if res.BaseRef != "release-v1" {
		t.Errorf("BaseRef = %q, want release-v1", res.BaseRef)
	}
	sort.Strings(res.Selected)
	want := []string{"./a/...", "./b/...", "./d/...", "./e/..."}
	if !reflect.DeepEqual(res.Selected, want) {
		t.Errorf("Selected = %v, want %v", res.Selected, want)
	}
	if !res.Pass || res.DurationMS < 0 {
		t.Errorf("got Pass=%v DurationMS=%d, want Pass=true DurationMS>=0", res.Pass, res.DurationMS)
	}
}

func TestRunShadowTests_ZeroCommitsAheadOfBaseSelectsNothing(t *testing.T) {
	repo := shadowFixture(t)
	repo.Git("checkout", "-q", "-b", "lane")

	res, err := RunShadowTests(context.Background(), repo.Dir, "main", nil)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	if len(res.Selected) != 0 || !res.Pass {
		t.Errorf("got %+v, want empty Selected and Pass true", res)
	}
}

func TestRunShadowTests_CancelledContextReturnsError(t *testing.T) {
	repo := shadowFixture(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := RunShadowTests(ctx, repo.Dir, "main", nil); err == nil {
		t.Errorf("expected error for cancelled context")
	}
}
