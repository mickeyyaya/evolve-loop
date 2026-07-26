package core

// build_persona_budget_check_test.go — permanent regression cover for the
// cycle-1101 in-lane persona-budget gate. The cycle-scoped ACS predicates
// (go/acs/cycle1101) exercise the same behaviour against a real go module and
// vanish with the cycle; these keep the CONJUNCTION (persona path changed AND
// internal/prompts red) pinned for every future cycle, hermetically: the
// go-test subprocess is replaced at the buildSelfCheckRunner seam so no test
// here spawns `go test`.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonaDocTouched(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  bool
	}{
		{"persona scout", []string{"agents/evolve-scout.md"}, true},
		{"persona among others", []string{"go/internal/core/x.go", "agents/evolve-auditor.md"}, true},
		{"future persona sibling", []string{"agents/evolve-reflector.md"}, true},
		{"non-persona doc", []string{"docs/notes.md"}, false},
		{"agents but not a persona doc", []string{"agents/README.txt"}, false},
		{"persona-ish prefix, wrong dir", []string{"skills/agents/evolve-scout.md"}, false},
		{"empty diff", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := personaDocTouched(tc.paths); got != tc.want {
				t.Errorf("personaDocTouched(%v) = %v, want %v", tc.paths, got, tc.want)
			}
		})
	}
}

// stubSelfCheckRunner swaps the go-test seam for the duration of a test and
// records which packages it was asked to run.
func stubSelfCheckRunner(t *testing.T, output string, passed bool) *[]string {
	t.Helper()
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	var ran []string
	buildSelfCheckRunner = func(_ context.Context, _, pkg string) (string, bool) {
		ran = append(ran, pkg)
		return output, passed
	}
	return &ran
}

func TestPersonaBudgetFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("persona touched and prompts RED rejects naming the package", func(t *testing.T) {
		ran := stubSelfCheckRunner(t, "--- FAIL: TestPersonaStopCriterionDedupe_CombinedLineCountReduced\n    combined line count = 812, want < 751\n", false)
		got := personaBudgetFailures(ctx, t.TempDir(), []string{"agents/evolve-builder.md"})
		if len(got) != 1 {
			t.Fatalf("want exactly 1 failure for a persona edit with internal/prompts RED, got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], "internal/prompts") {
			t.Errorf("failure must name internal/prompts so the operator knows which gate fired; got %q", got[0])
		}
		if !strings.Contains(got[0], "want < 751") {
			t.Errorf("failure must carry the go-test output so the builder can fix it directly; got %q", got[0])
		}
		if len(*ran) != 1 || (*ran)[0] != personaBudgetPkg {
			t.Errorf("gate must run exactly %q, ran %v", personaBudgetPkg, *ran)
		}
	})

	t.Run("persona touched and prompts GREEN approves", func(t *testing.T) {
		ran := stubSelfCheckRunner(t, "ok\n", true)
		if got := personaBudgetFailures(ctx, t.TempDir(), []string{"agents/evolve-scout.md"}); len(got) != 0 {
			t.Errorf("a persona edit whose budget test is GREEN must approve; got %v", got)
		}
		if len(*ran) != 1 {
			t.Errorf("gate must assert on the TEST OUTCOME, so it must actually run the package once; ran %v", *ran)
		}
	})

	t.Run("no persona path never runs the package", func(t *testing.T) {
		ran := stubSelfCheckRunner(t, "", false) // RED if it were ever run
		if got := personaBudgetFailures(ctx, t.TempDir(), []string{"docs/notes.md", "go/internal/core/x.go"}); len(got) != 0 {
			t.Errorf("a lane that never touches agents/evolve-*.md must see zero behaviour change; got %v", got)
		}
		if len(*ran) != 0 {
			t.Errorf("gate must not run internal/prompts for a non-persona lane; ran %v", *ran)
		}
	})

	t.Run("prompts already in the changed Go packages defers to the selfcheck engine", func(t *testing.T) {
		ran := stubSelfCheckRunner(t, "", false)
		paths := []string{"agents/evolve-scout.md", "go/internal/prompts/prompts.go"}
		if got := personaBudgetFailures(ctx, t.TempDir(), paths); len(got) != 0 {
			t.Errorf("changedPackageFloorChecks already owns this package here — the gate must not double-report; got %v", got)
		}
		if len(*ran) != 0 {
			t.Errorf("one go-test pass per package per handoff; ran %v", *ran)
		}
	})

	t.Run("no worktree fails open", func(t *testing.T) {
		ran := stubSelfCheckRunner(t, "", false)
		if got := personaBudgetFailures(ctx, "", []string{"agents/evolve-scout.md"}); len(got) != 0 {
			t.Errorf("missing worktree is plumbing, not a violation — must fail open; got %v", got)
		}
		if len(*ran) != 0 {
			t.Errorf("no worktree means nothing to run; ran %v", *ran)
		}
	})
}

// TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck is the WIRING proof:
// the check must fire through the engine actually injected at the build phase
// (cmd_cycle.go: NewBuildFloorReviewer(DefaultBuildFloorChecks)), not merely
// exist. A real git fixture supplies the diff because the production path
// derives changed paths from git, never from an injected list.
func TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	wt := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(wt, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	write("agents/evolve-scout.md", "# Evolve Scout\n")
	write("docs/notes.md", "# Notes\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	baseOut, err := exec.Command("git", "-C", wt, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: wt, WorktreeBaseSHA: base}

	// The lane grows the persona doc past budget and COMMITS (builder protocol).
	write("agents/evolve-scout.md", "# Evolve Scout\n\nlines that blow the budget\n")
	git("add", "-A")
	git("commit", "-q", "-m", "persona edit")

	stubSelfCheckRunner(t, "--- FAIL: TestPersonaStopCriterionDedupe_CombinedLineCountReduced\n", false)
	fails := DefaultBuildFloorChecks(context.Background(), in)
	if len(fails) != 1 || !strings.Contains(fails[0], "internal/prompts") {
		t.Fatalf("DefaultBuildFloorChecks did not surface the persona-budget breach — the check is UNWIRED; got %v", fails)
	}
	// And the reviewer converts it into a retryable REJECT, not a WARN.
	res := NewBuildFloorReviewer(DefaultBuildFloorChecks).Review(context.Background(), in)
	if res.Approve || !res.Retry {
		t.Fatalf("persona-budget breach must REJECT with Retry; got Approve=%v Retry=%v", res.Approve, res.Retry)
	}
}
