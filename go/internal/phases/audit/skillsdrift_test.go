package audit

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestChangesSkillsGenerator(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  bool
	}{
		{"no change", nil, false},
		{"docs and other packages", []string{"docs/a.md", "go/internal/core/x.go", "commands/scout.md"}, false},
		{"the renderer", []string{"docs/a.md", "go/internal/skillcheck/commands.go"}, true},
		{"an embedded template", []string{"go/internal/skillcheck/templates/skill.md.tmpl"}, true},
		{"a test of the generator", []string{"go/internal/skillcheck/commands_test.go"}, true},
		{"a sibling package with the same stem", []string{"go/internal/skillcheckx/a.go"}, false},
		{"the generator name outside go/internal", []string{"docs/go/internal/skillcheck/notes.md"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := changesSkillsGenerator(tc.paths); got != tc.want {
				t.Errorf("changesSkillsGenerator(%q) = %v, want %v", tc.paths, got, tc.want)
			}
		})
	}
}

func TestSkillsCheckOffenders(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   []string
	}{
		{"a clean report", "WARN: user phase x claims writes_source:true\n", nil},
		{"a stale stub", "DRIFT: commands/scout.md is stale or missing (run `evolve skills generate`)\n", []string{"commands/scout.md"}},
		{"a stale facts region", "  DRIFT: skills/build/SKILL.md phase-facts region is stale (run `evolve skills generate`)", []string{"skills/build/SKILL.md"}},
		{"an orphaned stub", "DRIFT: commands/gone.md is an orphaned generated command (run `evolve skills generate`)", []string{"commands/gone.md"}},
		{"an unparseable frontmatter", "DRIFT: skills/x/SKILL.md: unparseable frontmatter: bad", []string{"skills/x/SKILL.md"}},
		{"a manifest problem", "MANIFEST: agents[] lists \"a.md\" but the file is missing", []string{"MANIFEST: agents[] lists \"a.md\" but the file is missing"}},
		{"a go run trailer and a compile error", "x.go:1:2: undefined: y\nexit status 2\nDRIFT:\n", nil},
		{"a drift prefix with only white space after it", "DRIFT:   \nDRIFT: \u00a0\t\n", nil},
		{"a mixed report keeps the order", "WARN: w\nDRIFT: commands/a.md is stale or missing\nMANIFEST: m\nDRIFT: commands/b.md is stale or missing\nexit status 2\n",
			[]string{"commands/a.md", "MANIFEST: m", "commands/b.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillsCheckOffenders(tc.report); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("skillsCheckOffenders(%q) = %q, want %q", tc.report, got, tc.want)
			}
		})
	}
}

func TestWorktreeSkillsDrift_RunsTheWorktreeGeneratorCheck(t *testing.T) {
	root := filepath.Join(t.TempDir(), "lane")
	fake := &fixtures.FakeExec{}
	withFakeRunner(t, fake.Run)

	got, err := worktreeSkillsDrift(root)

	var notice gateWarning
	if got != nil || !errors.As(err, &notice) || !strings.Contains(notice.Error(), "the host generator could not compare") {
		t.Fatalf("a clean worktree check over a tree the host cannot read = (%v, %v), want a could-not-compare warning", got, err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("want one subprocess, got %v", fake.CallKeys())
	}
	call := fake.Calls[0]
	if call.Name != "go" || call.Dir != filepath.Join(root, "go") {
		t.Errorf("ran %q in %q, want go in %q", call.Name, call.Dir, filepath.Join(root, "go"))
	}
	if want := []string{"run", "./cmd/evolve", "skills", "check"}; !reflect.DeepEqual(call.Args, want) {
		t.Errorf("Args = %q, want %q", call.Args, want)
	}
	if n := len(call.Env); n == 0 || call.Env[n-1] != "EVOLVE_WORKTREE_ROOT="+root {
		t.Errorf("the check must read the worktree, Env = %q", call.Env)
	}
}

func TestWorktreeSkillsDrift_Outcomes(t *testing.T) {
	cases := []struct {
		name      string
		resp      fixtures.ExecResponse
		want      []string
		wantErrIn string
	}{
		{"drift lines are offenders", fixtures.ExecResponse{ExitCode: 1, Stderr: "WARN: w\nDRIFT: commands/scout.md is stale or missing\nexit status 2\n"}, []string{"commands/scout.md"}, ""},
		{"a non-zero exit without a report fails with the output", fixtures.ExecResponse{ExitCode: 1, Stderr: "panic: boom\n"},
			[]string{"the worktree generator skills check exited 1 without a drift report: panic: boom"}, ""},
		{"a non-zero exit with no output fails", fixtures.ExecResponse{ExitCode: 2},
			[]string{"the worktree generator skills check exited 2 without a drift report: (no output)"}, ""},
		{"a runner failure cannot grade and says so", fixtures.ExecResponse{Err: errors.New("go: not found")}, nil, "is NOT graded and only CI TestSkills_NoDrift checks it: go: not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withFakeRunner(t, (&fixtures.FakeExec{Default: tc.resp}).Run)

			got, err := worktreeSkillsDrift(t.TempDir())

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("offenders = %q, want %q", got, tc.want)
			}
			if tc.wantErrIn == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErrIn != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrIn)) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErrIn)
			}
		})
	}
}

func TestWorktreeSkillsDrift_BoundsTheRunWithTheGateLimit(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		deadline, hasDeadline = ctx.Deadline()
		return 0, nil
	})
	start := time.Now()

	worktreeSkillsDrift(t.TempDir())

	want := ciparitygate.DefaultTimeouts().GoVet
	if !hasDeadline {
		t.Fatal("the worktree run must carry a deadline")
	}
	if left := deadline.Sub(start); left < want-time.Second || left > want+time.Second {
		t.Errorf("deadline is %v after the start, want the go vet budget %v", left, want)
	}
}

func TestWorktreeSkillsDrift_ATimeoutCannotGradeAndSaysSo(t *testing.T) {
	orig := worktreeSkillsCheckLimit
	worktreeSkillsCheckLimit = 20 * time.Millisecond
	t.Cleanup(func() { worktreeSkillsCheckLimit = orig })
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		<-ctx.Done()
		return -1, nil
	})

	got, err := worktreeSkillsDrift(t.TempDir())

	if got != nil || err == nil || !strings.Contains(err.Error(), "is NOT graded") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a timed-out worktree run = (%v, %v), want a loud could-not-run error that wraps the deadline", got, err)
	}
}

func TestGeneratorDisagreement(t *testing.T) {
	cases := []struct {
		name    string
		host    []string
		hostErr error
		want    string
	}{
		{"the host agrees", nil, nil, ""},
		{"the host sees drift", []string{"commands/a.md", "commands/b.md"}, nil,
			"the lane's generator and the host generator disagree on 2 artifact(s); the lane grades itself, review the skillcheck diff: commands/a.md, commands/b.md"},
		{"the host cannot compare", nil, errors.New("load phase catalog: no registry"),
			"the lane's generator graded the lane clean, but the host generator could not compare (load phase catalog: no registry); the lane grades itself, review the skillcheck diff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := generatorDisagreement(tc.host, tc.hostErr)

			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != tc.want {
				t.Errorf("generatorDisagreement = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRun_SkillsDriftWarningKeepsThePASSAndNamesTheGate(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:           &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:          fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) { return nil, gateWarning("the generators disagree") },
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict = %q, want PASS (a warning never fails the audit)", resp.Verdict)
	}
	want := core.Diagnostic{Severity: "warning", Message: "skills-drift: the generators disagree"}
	if !reflect.DeepEqual(resp.Diagnostics, []core.Diagnostic{want}) {
		t.Errorf("Diagnostics = %+v, want only %+v", resp.Diagnostics, want)
	}
}
