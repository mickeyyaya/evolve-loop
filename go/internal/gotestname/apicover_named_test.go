package gotestname_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gotestname"
)

func TestExportsAreNamed(t *testing.T) {
	if !gotestname.Is("TestA") || gotestname.Is("Testify") {
		t.Fatal("gotestname.Is does not follow the go test name rule")
	}
	if !gotestname.HasPrefix("FuzzA", "Fuzz") || gotestname.HasPrefix("Fuzzy", "Fuzz") {
		t.Fatal("gotestname.HasPrefix does not follow the go test name rule")
	}
	if !gotestname.IgnoredDir("testdata") || gotestname.IgnoredDir("internal") {
		t.Fatal("gotestname.IgnoredDir does not name the directories the go tool ignores")
	}
	if !gotestname.UnderIgnoredDir("a/testdata/b.go") || gotestname.UnderIgnoredDir("a/b.go") {
		t.Fatal("gotestname.UnderIgnoredDir does not check each directory of the path")
	}
}
