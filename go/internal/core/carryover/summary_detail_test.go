package carryover

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestSummary_ALearningRecordKeepsTheFailureClassNotItsDetail(t *testing.T) {
	refusal := func(paths string) error {
		return errors.New(cyclestate.WithDetail("predicate execution tree includes undeclared inputs", paths) + "; acs-verdict.json: missing")
	}
	a := Summary(1694, "audit", refusal("go/acs/cycle1694/predicates_test.go"))
	b := Summary(1705, "audit", refusal("docs/explain/builds/cycle-1705-run.md, "+strings.Repeat("x/", 300)))
	if fingerprint(a) != fingerprint(b) {
		t.Fatalf("two refusals that differ only in their detail must dedupe:\n%s\n%s", a, b)
	}
	if !strings.Contains(b, "acs-verdict.json: missing") || strings.Contains(b, "x/x/") {
		t.Fatalf("the reason after the detail survives and the detail does not: %s", b)
	}
	other := Summary(1705, "audit", errors.New(cyclestate.WithDetail("predicate execution tree includes undeclared inputs", "a.go")+"; gofmt: 1 file"))
	if fingerprint(other) == fingerprint(a) {
		t.Fatal("a different trailing reason is a different failure")
	}
}
