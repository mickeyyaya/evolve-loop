package evalgate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const unsatisfiablePredicateSrc = `//go:build acs

package cycle4242

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC4242_RetiredFileGone(t *testing.T) {
	if acsassert.FileExists(t, "legacy.md") {
		t.Errorf("legacy.md still exists")
	}
}
`

const satisfiablePredicateSrc = `//go:build acs

package cycle4242

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC4242_RetiredFlagGone(t *testing.T) {
	if !acsassert.FileNotContains(t, "f.go", "retiredFlag") {
		t.Errorf("f.go still holds retiredFlag")
	}
}
`

func TestUnsatisfiableShapeGate_FlagsUnsatisfiablePredicateAdvisory(t *testing.T) {
	wt := t.TempDir()
	writeCyclePredicates(t, wt, 4242, unsatisfiablePredicateSrc)

	reason, block := unsatisfiableShapeGate().check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt,
	})
	if block {
		t.Error("the unsatisfiable lint must never block")
	}
	for _, want := range []string{"TestC4242_RetiredFileGone", "inverted-idiom", "FileExists", "1 finding(s)", "1 linted file(s)", "ADVISORY"} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason %q missing %q", reason, want)
		}
	}
}

func TestUnsatisfiableShapeGate_EveryOutcomeIsObservable(t *testing.T) {
	clean, empty, noacs := t.TempDir(), t.TempDir(), t.TempDir()
	writeCyclePredicates(t, clean, 4242, satisfiablePredicateSrc)
	writeCyclePredicates(t, empty, 4242, "")
	cases := []struct {
		name string
		in   core.ReviewInput
		want []string
	}{
		{"clean", core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: clean}, []string{"CLEAN", "linted 1 file(s)", "predicates_test.go", "0 findings"}},
		{"unparseable", core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: empty}, []string{"stood down", "NO predicate was inspected"}},
		{"no acs package", core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: noacs}, []string{"no Go ACS predicate package"}},
		{"no worktree", core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242)}, []string{"stood down", "cannot locate"}},
		{"no cycle", core.ReviewInput{Phase: "tdd", Workspace: t.TempDir(), Worktree: clean}, []string{"stood down", "cycle=0"}},
	}
	for _, c := range cases {
		reason, block := unsatisfiableShapeGate().check(c.in)
		if block {
			t.Errorf("%s: blocked; the lint is advisory", c.name)
		}
		for _, want := range c.want {
			if !strings.Contains(reason, want) {
				t.Errorf("%s: reason %q missing %q", c.name, reason, want)
			}
		}
	}
}

func TestUnsatisfiableShapeGate_OnlyAtTDD(t *testing.T) {
	g := unsatisfiableShapeGate()
	if !g.appliesTo(string(core.PhaseTDD)) {
		t.Error("the lint must run at the end of tdd, where predicates first exist")
	}
	for _, p := range []string{"scout", "triage", "build", "audit", "ship"} {
		if g.appliesTo(p) {
			t.Errorf("the lint must not run at %s", p)
		}
	}
}

func TestUnsatisfiableShapeGate_WiredIntoReviewer(t *testing.T) {
	for _, g := range newGatesForTest() {
		if g.name() == "unsatisfiable-predicate-shape" {
			return
		}
	}
	t.Fatal("unsatisfiableShapeGate is not wired into NewReviewer's gate list — the lint would run only when an agent hand-runs quality-check")
}

func TestNewReviewer_UnsatisfiablePredicateSurfacesButNeverBlocksAtEnforce(t *testing.T) {
	ws, wt := cycleWorkspace(t, 4242), t.TempDir()
	writeCyclePredicates(t, wt, 4242, unsatisfiablePredicateSrc)

	var logged []string
	rv := NewReviewer(config.StageEnforce).(*reviewer)
	rv.logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }

	res := rv.Review(context.Background(), core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: wt, ProjectRoot: t.TempDir()})
	if !res.Approve {
		t.Fatalf("an unsatisfiable SHAPE must never reject a deliverable at enforce; got Reason=%q", res.Reason)
	}
	var line string
	for _, l := range logged {
		if strings.Contains(l, "unsatisfiable-predicate-shape") {
			line = l
		}
	}
	if !strings.Contains(line, "TestC4242_RetiredFileGone") || !strings.Contains(line, "blocking=false") {
		t.Fatalf("the reviewer log must name the unsatisfiable predicate with blocking=false; got:\n%s", strings.Join(logged, "\n"))
	}
}
