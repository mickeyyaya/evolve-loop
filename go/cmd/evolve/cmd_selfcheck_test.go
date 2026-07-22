package main

// cmd_selfcheck_test.go — RED contract for ADR-0076 slice B: `evolve selfcheck
// build` is the builder's in-session pre-flight running the EXACT build-floor
// checks, so fixing happens inside the builder's loop and budget instead of
// post-hoc correction windows. DI seam (buildFloorChecksFn) so tests inject a
// stub; a wiring pin holds the seam to core.DefaultBuildFloorChecks.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunSelfcheck_FindingsExitOneAndPrinted(t *testing.T) {
	orig := buildFloorChecksFn
	defer func() { buildFloorChecksFn = orig }()
	buildFloorChecksFn = func(ctx context.Context, in core.ReviewInput) []string {
		return []string{"pkg/x: unit tests FAIL", "apicover naming floor: 1 package"}
	}
	var out, errw strings.Builder
	rc := runSelfcheck([]string{"build", "--worktree", t.TempDir()}, nil, &out, &errw)
	if rc != 1 {
		t.Fatalf("findings must exit 1, got %d", rc)
	}
	if !strings.Contains(out.String(), "unit tests FAIL") || !strings.Contains(out.String(), "apicover naming floor") {
		t.Fatalf("findings must print verbatim for the builder to act on:\n%s", out.String())
	}
}

func TestRunSelfcheck_CleanExitZero(t *testing.T) {
	orig := buildFloorChecksFn
	defer func() { buildFloorChecksFn = orig }()
	buildFloorChecksFn = func(context.Context, core.ReviewInput) []string { return nil }
	var out strings.Builder
	rc := runSelfcheck([]string{"build", "--worktree", t.TempDir()}, nil, &out, &out)
	if rc != 0 {
		t.Fatalf("clean check must exit 0, got %d", rc)
	}
	if !strings.Contains(out.String(), "GREEN") {
		t.Fatalf("clean run must say GREEN explicitly (handoff evidence):\n%s", out.String())
	}
}

func TestRunSelfcheck_UsageOnBadArgs(t *testing.T) {
	var out, errw strings.Builder
	if rc := runSelfcheck([]string{"bogus"}, nil, &out, &errw); rc != 2 {
		t.Fatalf("unknown subcommand must exit 2 with usage, got %d", rc)
	}
	if !strings.Contains(errw.String(), "selfcheck build") {
		t.Fatalf("usage must name the build subcommand:\n%s", errw.String())
	}
}

func TestSelfcheckSeam_DefaultsToBuildFloorChecks(t *testing.T) {
	want := reflect.ValueOf(core.DefaultBuildFloorChecks).Pointer()
	got := reflect.ValueOf(buildFloorChecksFn).Pointer()
	if want != got {
		t.Fatal("seam must default to core.DefaultBuildFloorChecks — the CLI and the floor must run the SAME checks")
	}
}
