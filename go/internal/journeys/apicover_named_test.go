package journeys

import (
	"errors"
	"testing"
)

func TestAPI_RegistryModelNamed(t *testing.T) {
	var row Row
	row.ID = "x"
	if row.ID != "x" || RegistryPath != "go/test/journeys/journeys.json" {
		t.Error("Row is a plain record at the spec path")
	}
	var a Assessment
	a.Problem = ProblemMissing
	if !a.Applies && a.Problem != "missing" {
		t.Error("Assessment carries a Problem")
	}
	problems := []Problem{ProblemMissing, ProblemMalformed, ProblemUnknownJourney, ProblemMissingTest, ProblemRegistryUnreadable}
	if len(problems) != 5 || errors.Is(ErrNotFound, ErrTagGated) {
		t.Error("the problems and the two resolve errors are distinct")
	}
	if _, err := Parse([]byte("[")); err == nil {
		t.Error("Parse of broken JSON fails")
	}
	if _, err := Load("absent.json"); err == nil {
		t.Error("Load of an absent file fails")
	}
	if got := Removed([]Row{row}, nil); len(got) != 1 {
		t.Errorf("Removed = %v", got)
	}
	if got := Unresolved(t.TempDir(), []Row{{ID: "x", Test: "p.TestX"}}); len(got) != 1 {
		t.Errorf("Unresolved = %v", got)
	}
	if got := Assess(t.TempDir(), "feat: x"); got.Applies {
		t.Errorf("Assess = %+v", got)
	}
}
