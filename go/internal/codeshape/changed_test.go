package codeshape

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const smallFunc = "package a\n\nfunc Small(x int) int {\n\treturn x + 1\n}\n"

const branchy = `
func Branchy(x int) int {
	if x == 1 {
		return 1
	}
	if x == 2 {
		return 2
	}
	if x == 3 {
		return 3
	}
	if x == 4 {
		return 4
	}
	if x == 5 {
		return 5
	}
	if x == 6 || x == 7 {
		return 6
	}
	if x == 8 && x > 0 {
		return 8
	}
	if x == 9 {
		return 9
	}
	return 0
}
`

func commit(t *testing.T, r *gittest.Repo, msg string) string {
	t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", msg)
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

func TestChangedFuncs_ACyclomaticElevenFunctionLowersTheChangedReading(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("go/internal/a/a.go", smallFunc)
	first := commit(t, repo, "small")
	repo.Write("go/internal/a/a.go", smallFunc+branchy)
	second := commit(t, repo, "branchy")

	before, err := ChangedFuncs(repo.Dir, first)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ChangedFuncs(repo.Dir, second)
	if err != nil {
		t.Fatal(err)
	}

	if k, n := Conforming(before); k != 1 || n != 1 {
		t.Errorf("first ship = %d/%d, want 1/1 (Small conforms)", k, n)
	}
	if k, n := Conforming(after); k != 0 || n != 1 || after[0].Name != "Branchy" || after[0].Cyclo != 11 {
		t.Errorf("second ship = %d/%d %+v, want 0/1: Branchy has cyclomatic 11", k, n, after)
	}
}
