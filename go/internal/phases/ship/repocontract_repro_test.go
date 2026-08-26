package ship

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded reproduces
// H3 from cycle 1559: a textual mention of a build constraint is not itself a
// build constraint. The current detector excludes this real failing test,
// allowing the ship to proceed with a red test in its staged diff.
func TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded(t *testing.T) {
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	runGit(t, repo, "add", "go/go.mod")
	runGit(t, repo, "commit", "-qm", "baseline")

	const path = "go/internal/reproduction/literal_test.go"
	mustWrite(t, filepath.Join(repo, path), `package reproduction

import "testing"

const historicalBuildTag = "//go:build requires_tmux"

func TestNewlyAddedRed(t *testing.T) { t.Fatal(historicalBuildTag) }
`)
	runGit(t, repo, "add", path)

	packages, excluded := addedTestPackages(repo)
	if want := []string{"./internal/reproduction"}; !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %v, want %v; a string literal must not exclude the staged failing test", packages, want)
	}
	if len(excluded) != 0 {
		t.Fatalf("excluded = %v, want no exclusions", excluded)
	}
}
