package audit

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestUndeclaredInputs_NamesThePathsAndBoundsTheList(t *testing.T) {
	two := undeclaredInputs([]string{"go/acs/cycle9/predicates_test.go", ".evolve/evals/item.md"}, nil).Error()
	for _, want := range []string{"go/acs/cycle9/predicates_test.go", ".evolve/evals/item.md", "stage"} {
		if !strings.Contains(two, want) {
			t.Errorf("the refusal must name %q: %s", want, two)
		}
	}
	var many []string
	for i := 0; i < undeclaredInputsShown+5; i++ {
		many = append(many, fmt.Sprintf("f%02d.go", i))
	}
	long := undeclaredInputs(many, nil).Error()
	if strings.Contains(long, many[undeclaredInputsShown]) || !strings.Contains(long, "and 5 more") {
		t.Errorf("the list is bounded and counts the rest: %s", long)
	}
	exact := undeclaredInputs(many[:undeclaredInputsShown], nil).Error()
	if strings.Contains(exact, "more") || !strings.Contains(exact, many[undeclaredInputsShown-1]) {
		t.Errorf("a list at the bound is shown whole, with no count: %s", exact)
	}
	if cyclestate.WithoutDetail(two) != cyclestate.WithoutDetail(long) || strings.Contains(cyclestate.WithoutDetail(two), "go/acs") {
		t.Errorf("the paths are detail, never part of the refusal's identity: %q vs %q", cyclestate.WithoutDetail(two), cyclestate.WithoutDetail(long))
	}
	unlisted := undeclaredInputs(nil, errors.New("diff-tree failed")).Error()
	if !strings.Contains(unlisted, "undeclared inputs") || !strings.Contains(unlisted, "diff-tree failed") {
		t.Errorf("a listing failure still refuses and says why the paths are missing: %s", unlisted)
	}
}
