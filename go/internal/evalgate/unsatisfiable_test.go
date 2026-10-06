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

const absenceMessagePredicateSrc = `//go:build acs

package cycle4242

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC4242_DuplicateHelperRemoved(t *testing.T) {
	if !acsassert.FileContains(t, "f.go", "helper") {
		t.Errorf("f.go still holds the duplicated helper")
	}
}
`

const goRunExitCodePredicateSrc = `//go:build acs

package cycle4242

import (
	"os/exec"
	"testing"
)

func TestC4242_UsageErrorExitsThree(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/tool", "bogus")
	_ = cmd.Run()
	if cmd.ProcessState.ExitCode() != 3 {
		t.Errorf("want exit 3 on a usage error")
	}
}
`

func TestUnsatisfiableShapeGate_BlocksOnlyTheProofKinds(t *testing.T) {
	cases := []struct {
		kind, src string
		wantBlock bool
		want      []string
	}{
		{"inverted-idiom", unsatisfiablePredicateSrc, true, []string{"TestC4242_RetiredFileGone", "FileExists", "BLOCKING at enforce"}},
		{"go-run-exit-code", goRunExitCodePredicateSrc, true, []string{"TestC4242_UsageErrorExitsThree", "exit code 3", "go build", "BLOCKING at enforce"}},
		{"absence-message", absenceMessagePredicateSrc, false, []string{"TestC4242_DuplicateHelperRemoved", "acsassert.FileNotContains", "ADVISORY: never blocks"}},
	}
	for _, c := range cases {
		wt := t.TempDir()
		writeCyclePredicates(t, wt, 4242, c.src)
		reason, block := unsatisfiableShapeGate().check(core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt})
		if block != c.wantBlock {
			t.Errorf("%s: block=%v, want %v; reason %q", c.kind, block, c.wantBlock, reason)
		}
		for _, want := range append([]string{"[" + c.kind + "]", "1 finding(s)", "1 linted file(s)"}, c.want...) {
			if !strings.Contains(reason, want) {
				t.Errorf("%s: reason %q missing %q", c.kind, reason, want)
			}
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

func TestNewReviewer_ProofKindRejectsAtEnforceAndIsOnlyLoggedAtShadow(t *testing.T) {
	ws, wt := cycleWorkspace(t, 4242), t.TempDir()
	writeCyclePredicates(t, wt, 4242, unsatisfiablePredicateSrc)
	in := core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: wt, ProjectRoot: t.TempDir()}
	cases := []struct {
		stage       config.Stage
		wantApprove bool
		wantLog     string
	}{
		{config.StageEnforce, false, "stage=enforce, blocking=true"},
		{config.StageShadow, true, "stage=shadow, blocking=false"},
	}
	for _, c := range cases {
		var logged []string
		rv := NewReviewer(c.stage).(*reviewer)
		rv.logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }

		res := rv.Review(context.Background(), in)
		if res.Approve != c.wantApprove {
			t.Errorf("%s: Approve=%v, want %v; Reason=%q", c.stage, res.Approve, c.wantApprove, res.Reason)
		}
		if !c.wantApprove && !strings.Contains(res.Reason, "TestC4242_RetiredFileGone [inverted-idiom]") {
			t.Errorf("%s: the rejection must name the unsatisfiable predicate and its kind; got %q", c.stage, res.Reason)
		}
		var line string
		for _, l := range logged {
			if strings.Contains(l, "unsatisfiable-predicate-shape") {
				line = l
			}
		}
		if !strings.Contains(line, "TestC4242_RetiredFileGone") || !strings.Contains(line, c.wantLog) {
			t.Errorf("%s: the reviewer log must name the predicate with %q; got:\n%s", c.stage, c.wantLog, strings.Join(logged, "\n"))
		}
	}
}
