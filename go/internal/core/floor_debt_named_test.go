package core

// floor_debt_named_test.go — pays the cycle-1048 debt: four core exports were
// exercised only under integration tags, reading 0% (false-green) in every
// scoped coverage run and floor-blocking all core-touching lanes. Default-tag
// tests with real assertions.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

func TestDefaultBuildFloorChecks_NonRepoWorktreeIsClean(t *testing.T) {
	// A non-git worktree yields no changed packages — the floor must return
	// no failures rather than erroring (degraded-provisioning shape).
	if fails := DefaultBuildFloorChecks(context.Background(), ReviewInput{Worktree: t.TempDir()}); len(fails) != 0 {
		t.Fatalf("non-repo worktree must produce no floor failures, got %v", fails)
	}
}

func TestWithFailureAdvisorPersona_InjectsAndIgnoresEmpty(t *testing.T) {
	a := &FailureAdvisor{}
	WithFailureAdvisorPersona("PERSONA-BODY")(a)
	if a.identity.Persona != "PERSONA-BODY" {
		t.Fatalf("persona not injected: %q", a.identity.Persona)
	}
	WithFailureAdvisorPersona("")(a)
	if a.identity.Persona != "PERSONA-BODY" {
		t.Fatal("empty body must not clobber an injected persona")
	}
}

type stubKB struct{}

func (stubKB) Lookup(context.Context, research.Query) ([]research.Lesson, error) { return nil, nil }

func TestWithKB_InjectsAndIgnoresNil(t *testing.T) {
	o := &Orchestrator{}
	WithKB(stubKB{})(o)
	if o.kb == nil {
		t.Fatal("KB not injected")
	}
	prev := o.kb
	WithKB(nil)(o)
	if o.kb != prev {
		t.Fatal("nil KB must be ignored (no-recall default preserved)")
	}
}

func TestWithGitDirtyPaths_OverridesSeam(t *testing.T) {
	o := &Orchestrator{}
	WithGitDirtyPaths(func(context.Context, string) ([]string, error) {
		return []string{"seam.go"}, nil
	})(o)
	if o.gitDirtyPaths == nil {
		t.Fatal("seam not injected")
	}
	got, err := o.gitDirtyPaths(context.Background(), "")
	if err != nil || len(got) != 1 || got[0] != "seam.go" {
		t.Fatalf("injected seam must be the one consulted, got %v err %v", got, err)
	}
}
