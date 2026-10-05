package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunSelfcheck_FindingsExitOneAndPrinted(t *testing.T) {
	orig := buildHandoffFloorFor
	defer func() { buildHandoffFloorFor = orig }()
	buildHandoffFloorFor = func(string) core.BuildHandoffFloor {
		return core.BuildHandoffFloor{{Name: "stub", Run: func(ctx context.Context, in core.ReviewInput) []string {
			return []string{"pkg/x: unit tests FAIL", "apicover naming floor: 1 package"}
		}}}
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
	r := newHandoffReplay(t, 1788, nil)
	r.commitBuild(t, map[string]string{replayLint: replayLintBody})
	r.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Changes\n- `"+replayLint+"`\n")
	orig := buildHandoffFloorFor
	defer func() { buildHandoffFloorFor = orig }()
	buildHandoffFloorFor = func(string) core.BuildHandoffFloor { return nil }
	var out strings.Builder
	rc := runSelfcheck([]string{"build", "--worktree", r.worktree}, nil, &out, &out)
	if rc != 0 || !strings.Contains(out.String(), "GREEN") || !strings.Contains(out.String(), "safe to hand off") {
		t.Fatalf("a bound, well-formed build with a green floor exits 0 and says GREEN (handoff evidence): rc=%d\n%s", rc, out.String())
	}
}

func TestRunSelfcheck_WithoutACycleBindingNeverClaimsSafe(t *testing.T) {
	orig := buildHandoffFloorFor
	defer func() { buildHandoffFloorFor = orig }()
	buildHandoffFloorFor = func(string) core.BuildHandoffFloor { return nil }
	var out, errw strings.Builder
	rc := runSelfcheck([]string{"build", "--worktree", t.TempDir()}, nil, &out, &errw)
	if rc == 0 || strings.Contains(out.String(), "safe to hand off") || !strings.Contains(errw.String(), "no cycle binding") {
		t.Fatalf("an unbound worktree has no build report to hand off and must say it could not bind: rc=%d\nstdout=%s\nstderr=%s", rc, out.String(), errw.String())
	}
	if got := selfcheckGreen("/wt", errors.New("no state")); !strings.Contains(got, "without a cycle binding") || strings.Contains(got, "safe to hand off") {
		t.Fatalf("a GREEN with no cycle binding must not claim it predicts the floor: %q", got)
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
	want := reflect.ValueOf(probeBuildHandoffFloor).Pointer()
	got := reflect.ValueOf(buildHandoffFloorFor).Pointer()
	if want != got {
		t.Fatal("seam must default to probeBuildHandoffFloor — the probes and the floor must run the SAME checks")
	}
}

func TestBuildFloorRoots_ComposeOnlyThroughTheProductionFloor(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(src), "\n") {
			if strings.Contains(line, "core.DefaultBuildFloorChecks") && !strings.HasPrefix(strings.TrimSpace(line), "//") {
				refs = append(refs, name+": "+strings.TrimSpace(line))
			}
		}
	}
	if len(refs) != 1 || !strings.HasPrefix(refs[0], "cmd_cycle_config.go: ") || !strings.Contains(refs[0], "out := append(core.ProtectedSurfaceFloorChecks(guards.IsProtectedSurface)(ctx, in), core.DefaultBuildFloorChecks(ctx, in)...)") {
		t.Fatalf("core.DefaultBuildFloorChecks must be referenced only by the production composition; got %v", refs)
	}
}
