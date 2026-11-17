package repocontract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const byNameTestCap = 775

func rowPathProblem(repoRoot, path string) string {
	trimmed := strings.TrimSuffix(path, "/")
	if path == "" || path == WholeRepository || trimmed == "" {
		return ""
	}
	info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(trimmed)))
	switch {
	case err != nil:
		return ""
	case strings.HasSuffix(path, "/") && !info.IsDir():
		return fmt.Sprintf("Path %q ends in / but is a file; remove the slash", path)
	case !strings.HasSuffix(path, "/") && info.IsDir():
		return fmt.Sprintf("Path %q is a directory; end it in /", path)
	}
	return ""
}

func byNameCountProblem(selections []TestSelection, limit int) string {
	count := 0
	for _, selection := range selections {
		count += len(selection.Tests)
	}
	if count <= limit {
		return ""
	}
	return fmt.Sprintf("ByNameTests() names %d tests, over byNameTestCap %d; the pack runs every row at each floor and ship, so revisit D21 of docs/plans/data-file-test-floor-2026-11.md: move a row out of the always set, or move a package whole into Packages()", count, limit)
}

func TestDataFileReads_EveryRowPathMatchesItsKind(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range DataFileReads() {
		if problem := rowPathProblem(root, row.Path); problem != "" {
			t.Error(problem)
		}
	}
}

func TestByNameCountProblem(t *testing.T) {
	t.Parallel()
	selections := []TestSelection{{Package: "./a", Tests: []string{"T1", "T2"}}, {Package: "./b", Tests: []string{"T3"}}}
	if got := byNameCountProblem(selections, 3); got != "" {
		t.Errorf("3 tests under a cap of 3: %q", got)
	}
	got := byNameCountProblem(selections, 2)
	if !strings.Contains(got, "3 tests") || !strings.Contains(got, "2") || !strings.Contains(got, "D21") {
		t.Errorf("3 tests over a cap of 2 must name the count, the cap and the revisit of D5; got %q", got)
	}
}

func TestByNameTests_StaysUnderTheCap(t *testing.T) {
	t.Parallel()
	if problem := byNameCountProblem(ByNameTests(), byNameTestCap); problem != "" {
		t.Error(problem)
	}
}
