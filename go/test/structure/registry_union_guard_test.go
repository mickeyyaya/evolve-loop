package structure

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var unionRegistries = []string{".cover-strict", ".apicover-enforce"}

func repeatedPackage(name string, data []byte) error {
	seen := map[string]int{}
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key := path.Clean(strings.Fields(line)[0])
		if first, dup := seen[key]; dup {
			return fmt.Errorf("%s line %d: package %q repeats line %d (%q); keep one line for the package and remove the other", name, n+1, key, first, line)
		}
		seen[key] = n + 1
	}
	return nil
}

func unionMerges(t *testing.T, name string, adds []string) []string {
	t.Helper()
	attrs, err := os.ReadFile(filepath.Join("..", "..", "..", ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	list, err := os.ReadFile(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatal(err)
	}
	rel := "go/" + name
	base := strings.TrimRight(string(list), "\n") + "\n"
	repo := gittest.Fixture(t)
	repo.Write(".gitattributes", string(attrs))
	repo.Write(rel, base)
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	branches := []string{"one", "two"}
	for i, b := range branches {
		repo.Git("checkout", "-q", "-b", b, "main")
		repo.Write(rel, base+adds[i]+"\n")
		repo.Git("commit", "-q", "-am", b)
	}
	var merged []string
	for _, order := range [][]string{{"one", "two"}, {"two", "one"}} {
		tree := repo.Git("merge-tree", "--write-tree", order[0], order[1])
		merged = append(merged, repo.Git("show", strings.Fields(tree)[0]+":"+rel)+"\n")
	}
	return merged
}

func TestRepeatedPackageAcceptsDistinctPackagesAndComments(t *testing.T) {
	t.Parallel()
	data := "# note\n./b 100\n\n./a 100\n# ./a 90\n./a/c 90\n"
	if err := repeatedPackage(".cover-strict", []byte(data)); err != nil {
		t.Fatalf("repeatedPackage refused distinct packages: %v", err)
	}
}

func TestRepeatedPackageRefusesSamePackageWithAnotherValueAndNamesLineAndFix(t *testing.T) {
	t.Parallel()
	err := repeatedPackage(".cover-strict", []byte("# h\n./a 100\n\n  ./a 90\n"))
	if err == nil {
		t.Fatal("repeatedPackage accepted one package twice")
	}
	for _, want := range []string{".cover-strict", "line 4", "line 2", `"a"`, "./a 90", "remove"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestRepeatedPackageTreatsDotSlashSpellingAsTheSamePackage(t *testing.T) {
	t.Parallel()
	if err := repeatedPackage(".apicover-enforce", []byte("./a\na\n")); err == nil {
		t.Fatal("repeatedPackage accepted ./a and a")
	}
}

func TestRepeatedPackageSkipsABlankLineAndAnIndentedComment(t *testing.T) {
	t.Parallel()
	if err := repeatedPackage(".cover-strict", []byte("./a 100\n   \n  # ./a 90\n")); err != nil {
		t.Fatalf("repeatedPackage refused a blank line or an indented comment: %v", err)
	}
}

func TestUnionRegistriesMatchGitAttributes(t *testing.T) {
	t.Parallel()
	attrs, err := os.ReadFile(filepath.Join("..", "..", "..", ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, line := range strings.Split(string(attrs), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && slices.Contains(fields[1:], "merge=union") && strings.HasPrefix(fields[0], "go/.") {
			found = append(found, strings.TrimPrefix(fields[0], "go/"))
		}
	}
	want := slices.Sorted(slices.Values(unionRegistries))
	slices.Sort(found)
	if !slices.Equal(found, want) {
		t.Fatalf(".gitattributes union registries %v, guard list %v; keep the two lists equal", found, want)
	}
}

func TestParallelLanesAddingOneIdenticalLineKeepOneLineInBothMergeOrders(t *testing.T) {
	t.Parallel()
	for name, add := range map[string]string{".cover-strict": "./zz/same 100", ".apicover-enforce": "./zz/same"} {
		for i, merged := range unionMerges(t, name, []string{add, add}) {
			if got := strings.Count(merged, add); got != 1 {
				t.Errorf("%s order %d: %d copies of the line, want 1", name, i, got)
			}
			if err := repeatedPackage(name, []byte(merged)); err != nil {
				t.Errorf("%s order %d: %v", name, i, err)
			}
		}
	}
}

func TestUnionMergesTriesBothOrders(t *testing.T) {
	t.Parallel()
	merged := unionMerges(t, ".apicover-enforce", []string{"./zz/one", "./aa/two"})
	if len(merged) != 2 {
		t.Fatalf("unionMerges returned %d results, want one for each merge order", len(merged))
	}
	if merged[0] == merged[1] {
		t.Fatal("the two results are equal, so both ran one merge order")
	}
	for i, m := range merged {
		if !strings.Contains(m, "./zz/one") || !strings.Contains(m, "./aa/two") {
			t.Errorf("order %d lacks a line of a side", i)
		}
	}
}

func TestRegistryFilesHoldEachPackageOnce(t *testing.T) {
	t.Parallel()
	for _, name := range unionRegistries {
		data, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := repeatedPackage(name, data); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestParallelLanesAddingDistinctPackagesComposeInBothMergeOrders(t *testing.T) {
	t.Parallel()
	for _, name := range unionRegistries {
		adds := []string{"./zz/one", "./aa/two"}
		if name == ".cover-strict" {
			adds = []string{"./zz/one 100", "./aa/two 100"}
		}
		for i, merged := range unionMerges(t, name, adds) {
			if err := repeatedPackage(name, []byte(merged)); err != nil {
				t.Errorf("%s order %d: %v", name, i, err)
			}
			if name == ".cover-strict" {
				if _, err := coverstrict.ParseFloors([]byte(merged)); err != nil {
					t.Errorf("order %d: reader refused: %v", i, err)
				}
			}
		}
	}
}

func TestParallelLanesAddingOnePackageFailTheGuardInBothMergeOrders(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		".cover-strict":     {"./zz/same 100", "./zz/same 90"},
		".apicover-enforce": {"./zz/same", "zz/same"},
	}
	for name, adds := range cases {
		for i, merged := range unionMerges(t, name, adds) {
			err := repeatedPackage(name, []byte(merged))
			if err == nil {
				t.Fatalf("%s order %d: guard accepted a repeated package", name, i)
			}
			for _, want := range []string{name, "zz/same", "remove"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%s order %d: error %q lacks %q", name, i, err, want)
				}
			}
		}
	}
}
