//go:build integration

package audit

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	rendererFile  = "commands.go"
	checkerFile   = "skillcheck.go"
	laneMarkerOld = `"Arguments: $ARGUMENTS\n",`
	laneMarkerNew = `"Arguments: $ARGUMENTS\n\nLane marker: the generator changed.\n",`
	selfIDClause  = " (Skill tool id `evo:%s`)"
	selfIDArgsOld = "commandGenMarker, name, name, name,"
	selfIDArgsNew = "commandGenMarker, name, name,"
	driftWordOld  = `"DRIFT: %s is stale or missing`
	driftWordNew  = `"STALE: %s is stale or missing`
)

func generatorLaneRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repoRoot := skillsDriftRepoRoot(t)
	repo := gittest.Fixture(t)
	for _, rel := range []string{
		filepath.Join("docs", "architecture", "phase-registry.json"),
		filepath.Join("go", "go.mod"),
		filepath.Join("go", "go.sum"),
	} {
		skillsDriftCopyFile(t, filepath.Join(repoRoot, rel), filepath.Join(repo.Dir, rel))
	}
	for _, dir := range []string{
		"skills", "commands", "agents", ".claude-plugin", ".codex-plugin",
		filepath.Join(".agents", "plugins"), filepath.Join(".evolve", "profiles"), filepath.Join(".evolve", "phases"),
		filepath.Join("go", "vendor"), filepath.Join("go", "cmd"), filepath.Join("go", "internal"), filepath.Join("go", "pkg"),
	} {
		skillsDriftCopyTree(t, filepath.Join(repoRoot, dir), filepath.Join(repo.Dir, dir))
	}
	if drift, err := skillcheck.Check(repo.Dir); err != nil || len(drift) != 0 {
		t.Fatalf("the base copy must be clean before the lane edits it: drift=%v err=%v", drift, err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "base")
	return repo
}

func generatorLaneWorktree(t *testing.T) string {
	t.Helper()
	return generatorLaneRepo(t).Dir
}

func editGenerator(t *testing.T, root, file string, pairs ...string) {
	t.Helper()
	path := filepath.Join(root, "go", "internal", "skillcheck", file)
	src := fixtures.MustRead(t, path)
	for i := 0; i < len(pairs); i += 2 {
		if strings.Count(src, pairs[i]) != 1 {
			t.Fatalf("generator anchor %q must occur once in %s", pairs[i], path)
		}
		src = strings.Replace(src, pairs[i], pairs[i+1], 1)
	}
	fixtures.MustWrite(t, path, src)
}

func regenerateWithWorktreeGenerator(t *testing.T, root string) {
	t.Helper()
	inv := core.WorktreeEvolveInvocation(root, "skills", "generate")
	var out strings.Builder
	code, err := sysexec.DefaultRunner(context.Background(), "go", inv.Dir, inv.Args, inv.Env, nil, &out, &out)
	if err != nil || code != 0 {
		t.Fatalf("worktree skills generate: exit %d, err %v\n%s", code, err, out.String())
	}
}

func generatedStubs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatalf("read skills: %v", err)
	}
	var stubs []string
	for _, e := range entries {
		if fixtures.FilePresent(filepath.Join(root, "skills", e.Name(), "SKILL.md")) {
			stubs = append(stubs, filepath.Join("commands", e.Name()+".md"))
		}
	}
	sort.Strings(stubs)
	return stubs
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func requireDisagreement(t *testing.T, err error, n int) {
	t.Helper()
	var notice gateWarning
	if !errors.As(err, &notice) {
		t.Fatalf("a passing lane whose generator disagrees with the host must give a gate warning, got %v", err)
	}
	if want := "disagree on " + strconv.Itoa(n) + " artifact(s)"; !strings.Contains(notice.Error(), want) {
		t.Errorf("warning %q must contain %q", notice.Error(), want)
	}
}

func TestSkillsDriftGate_GeneratorChangeWithRegeneratedStubsPasses(t *testing.T) {
	root := generatorLaneWorktree(t)
	editGenerator(t, root, rendererFile, laneMarkerOld, laneMarkerNew)
	regenerateWithWorktreeGenerator(t, root)
	host, _ := skillcheck.Check(root)
	if len(host) == 0 {
		t.Fatal("precondition: the host generator must see the regenerated stubs as drift, or this test proves nothing")
	}

	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: root, ProjectRoot: t.TempDir()})

	if len(got) != 0 {
		t.Fatalf("a generator change with consistent regeneration must pass: got %d offender(s), want 0: %v", len(got), got)
	}
	requireDisagreement(t, err, len(host))
}

func TestSkillsDriftGate_CommittedGeneratorChangeTriggersFromTheCycleBase(t *testing.T) {
	repo := generatorLaneRepo(t)
	base := repo.Git("rev-parse", "HEAD")
	editGenerator(t, repo.Dir, rendererFile, laneMarkerOld, laneMarkerNew)
	regenerateWithWorktreeGenerator(t, repo.Dir)
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "lane")
	stubs := generatedStubs(t, repo.Dir)
	cases := []struct {
		name          string
		base          string
		wantOffenders []string
	}{
		{"with the cycle base the committed change is seen", base, nil},
		{"without a base only HEAD is diffed and the change is missed", "", stubs},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := skillsDriftCheckDefault(core.PhaseRequest{Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: tc.base})

			if !reflect.DeepEqual(sortedCopy(got), sortedCopy(tc.wantOffenders)) {
				t.Errorf("offenders = %v, want %v", got, tc.wantOffenders)
			}
		})
	}
}

func TestSkillsDriftGate_GeneratorChangeThatAgreesWithTheHostIsSilent(t *testing.T) {
	root := generatorLaneWorktree(t)
	fixtures.MustWrite(t, filepath.Join(root, "go", "internal", "skillcheck", "zz_lane.go"), "package skillcheck\n\nconst laneTouch = 1\n")

	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: root, ProjectRoot: t.TempDir()})

	if got != nil || err != nil {
		t.Fatalf("a generator change with identical output = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestSkillsDriftGate_GeneratorChangeWithoutRegenerationFails(t *testing.T) {
	root := generatorLaneWorktree(t)
	editGenerator(t, root, rendererFile, laneMarkerOld, laneMarkerNew)
	if host, _ := skillcheck.Check(root); len(host) != 0 {
		t.Fatalf("precondition: the host generator must see the old stubs as clean, got %v", host)
	}

	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: root, ProjectRoot: t.TempDir()})

	if err != nil {
		t.Fatalf("gate error: %v", err)
	}
	if want := generatedStubs(t, root); !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("a generator change without regeneration must fail on every stub:\ngot  %v\nwant %v", got, want)
	}
}

func TestSkillsDriftGate_ReworkedReportProtocolStillFails(t *testing.T) {
	root := generatorLaneWorktree(t)
	editGenerator(t, root, rendererFile, laneMarkerOld, laneMarkerNew)
	editGenerator(t, root, checkerFile, driftWordOld, driftWordNew)

	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: root, ProjectRoot: t.TempDir()})

	if err != nil {
		t.Fatalf("a red worktree check must FAIL, not fail open: %v", err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "without a drift report") || !strings.Contains(got[0], "STALE: commands/") {
		t.Fatalf("a reworded report must give one offender that carries the output, got %q", got)
	}
}

func TestSkillsDriftGate_LaneWithoutGeneratorChangeIsGradedInProcess(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, root string)
	}{
		{"a clean lane", func(t *testing.T, root string) {
			fixtures.MustWrite(t, filepath.Join(root, "docs", "notes.md"), "a doc change\n")
		}},
		{"a lane with a drifted SKILL.md", func(t *testing.T, root string) {
			path := filepath.Join(root, "skills", "build", "SKILL.md")
			fixtures.MustWrite(t, path, strings.Replace(fixtures.MustRead(t, path), "## Output contract", "## Output contracts", 1))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := generatorLaneWorktree(t)
			tc.edit(t, root)
			fake := &fixtures.FakeExec{}
			withFakeRunner(t, fake.Run)
			req := core.PhaseRequest{Cycle: 7, Worktree: root, ProjectRoot: t.TempDir()}

			gotDiags := skillsGateDiagnostics(t, skillsDriftCheckDefault, req)
			wantDiags := skillsGateDiagnostics(t, func(r core.PhaseRequest) ([]string, error) { return skillcheck.Check(r.Worktree) }, req)

			if !reflect.DeepEqual(gotDiags, wantDiags) {
				t.Errorf("gate output must be byte-identical to the in-process check:\ngot  %q\nwant %q", gotDiags, wantDiags)
			}
			if len(fake.Calls) != 0 {
				t.Errorf("a lane that does not touch the generator must not run a subprocess, got %v", fake.CallKeys())
			}
		})
	}
}

func skillsGateDiagnostics(t *testing.T, check func(core.PhaseRequest) ([]string, error), req core.PhaseRequest) []string {
	t.Helper()
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	req.Workspace = ws
	phase := New(Config{
		Bridge:           &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:          fakePromptsFS("body"),
		CheckSkillsDrift: check,
	})
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var msgs []string
	for _, d := range resp.Diagnostics {
		msgs = append(msgs, string(resp.Verdict)+"|"+d.Severity+"|"+d.Message)
	}
	return msgs
}

func TestSkillsDriftGate_Cycle1840ReplayPasses(t *testing.T) {
	root := generatorLaneWorktree(t)
	editGenerator(t, root, rendererFile, selfIDClause, "", selfIDArgsOld, selfIDArgsNew)
	regenerateWithWorktreeGenerator(t, root)
	if strings.Contains(fixtures.MustRead(t, filepath.Join(root, "commands", "scout.md")), "evo:scout") {
		t.Fatal("precondition: the replayed generator must drop the self-reference from commands/scout.md")
	}
	stubs := generatedStubs(t, root)
	if host, _ := skillcheck.Check(root); !reflect.DeepEqual(sortedCopy(host), stubs) {
		t.Fatalf("precondition: the host generator must report every stub as drift, as in cycle 1840, got %v", host)
	}

	diags := skillsGateDiagnostics(t, skillsDriftCheckDefault, core.PhaseRequest{Cycle: 1840, Worktree: root, ProjectRoot: t.TempDir()})

	want := "PASS|warning|skills-drift: the lane's generator and the host generator disagree on " + strconv.Itoa(len(stubs)) + " artifact(s)"
	if len(diags) != 1 || !strings.HasPrefix(diags[0], want) {
		t.Fatalf("the cycle-1840 lane must keep its PASS with one disagreement warning:\ngot  %q\nwant prefix %q", diags, want)
	}
}

func TestSkillsDriftGate_ARealRunnerTimeoutCannotGradeAndSaysSo(t *testing.T) {
	orig := worktreeSkillsCheckLimit
	worktreeSkillsCheckLimit = 200 * time.Millisecond
	t.Cleanup(func() { worktreeSkillsCheckLimit = orig })
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return sysexec.DefaultRunner(ctx, "sleep", "", []string{"30"}, env, stdin, stdout, stderr)
	})

	got, err := worktreeSkillsDrift(t.TempDir())

	if got != nil || err == nil || !strings.Contains(err.Error(), "is NOT graded") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a real runner killed by the deadline = (%q, %v), want a loud could-not-run error that wraps the deadline", got, err)
	}
}
