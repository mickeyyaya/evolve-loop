package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const enforceL2PolicyJSON = `{"fleet":{"integration":{"stage":"enforce"}}}`

func writeL2Policy(t *testing.T, root, body string) {
	t.Helper()
	file := filepath.Join(root, ".evolve", "policy.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type l2Commands struct {
	j              *trainJourney
	calls          []string
	createsAtFirst int
	unreachable    error
}

func (c *l2Commands) run(_ context.Context, dir string, argv ...string) (string, int, error) {
	if len(c.calls) == 0 {
		c.createsAtFirst = c.j.gh.creates
	}
	c.calls = append(c.calls, strings.Join(argv, " "))
	if c.unreachable != nil {
		return "", 0, c.unreachable
	}
	tree := filepath.Dir(dir)
	if argv[0] == "gofmt" && l2Both(tree) {
		return "b.md\n", 0, nil
	}
	return "", 0, nil
}

func l2Both(tree string) bool {
	_, errA := os.Stat(filepath.Join(tree, "a.md"))
	_, errB := os.Stat(filepath.Join(tree, "b.md"))
	return errA == nil && errB == nil
}

func (c *l2Commands) ranGofmt() bool {
	return slices.ContainsFunc(c.calls, func(call string) bool { return strings.HasPrefix(call, "gofmt") })
}

func l2Journey(t *testing.T, policyBody string) (*trainJourney, *l2Commands) {
	t.Helper()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.pr(t, 2, "feat: b", "b.md", "b\n")
	for _, n := range []string{"1", "2"} {
		if rc, _, errb := j.run("record-pr", "--pr", n, "--class", "docs-review"); rc != 0 {
			t.Fatal(errb)
		}
	}
	if policyBody != "" {
		writeL2Policy(t, j.work.Dir, policyBody)
	}
	cmds := &l2Commands{j: j}
	j.env.Run = cmds.run
	return j, cmds
}

func (j *trainJourney) mainFiles() []string {
	return strings.Fields(j.origin.Git("ls-tree", "-r", "--name-only", "refs/heads/main"))
}

func TestJourneyBatchRunsTheBatchTestBeforeTheBatchPRAndBriefsTheMemberThatFailsOnlyOnTheCombinedTree(t *testing.T) {
	t.Parallel()
	j, cmds := l2Journey(t, enforceL2PolicyJSON)
	rc, _, errb := j.run("batch", "1", "2")
	if rc != exitRefused {
		t.Fatalf("batch rc=%d %s, want %d: PR 2 was ejected and stays open", rc, errb, exitRefused)
	}
	if !cmds.ranGofmt() || cmds.createsAtFirst != 0 {
		t.Errorf("gofmt ran = %v with %d batch PRs already created at the first L2 command, want it run before the batch PR", cmds.ranGofmt(), cmds.createsAtFirst)
	}
	brief := strings.Join(j.gh.comments["2"], "\n")
	for _, want := range []string{"ejected PR #2 on a batch", "red on [gofmt]", "b.md"} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief of PR 2 lacks %q:\n%s", want, brief)
		}
	}
	if len(j.gh.comments["1"]) != 0 {
		t.Errorf("PR 1 passed and got a comment: %v", j.gh.comments["1"])
	}
	if files := j.mainFiles(); !slices.Contains(files, "a.md") || slices.Contains(files, "b.md") {
		t.Errorf("origin main holds %v, want PR 1 and not PR 2", files)
	}
	if j.gh.creates != 1 {
		t.Errorf("created %d batch PRs, want 1", j.gh.creates)
	}
}

func TestJourneyBatchHostKillOfTheBatchTestEjectsNoMemberAndCreatesNoBatchPR(t *testing.T) {
	t.Parallel()
	j, cmds := l2Journey(t, enforceL2PolicyJSON)
	cmds.unreachable = acsrunner.HostFailure(errors.New("signal: killed"))
	before := j.originMain()
	rc, _, errb := j.run("batch", "1", "2")
	if rc != exitIO {
		t.Errorf("batch rc=%d %s, want %d: a host failure is not a refusal", rc, errb, exitIO)
	}
	if len(j.gh.comments) != 0 || len(j.gh.closed) != 0 || j.gh.creates != 0 || j.originMain() != before {
		t.Errorf("a host kill ejected %v, closed %v, created %d batch PRs", j.gh.comments, j.gh.closed, j.gh.creates)
	}
}

func TestJourneyBatchAtShadowRunsNoBatchTestCommand(t *testing.T) {
	t.Parallel()
	j, cmds := l2Journey(t, "")
	if rc, _, errb := j.run("batch", "1", "2"); rc != 0 {
		t.Fatalf("batch rc=%d %s", rc, errb)
	}
	if len(cmds.calls) != 0 {
		t.Errorf("a shadow stage ran %v, want no batch test command", cmds.calls)
	}
	if files := j.mainFiles(); !slices.Contains(files, "a.md") || !slices.Contains(files, "b.md") {
		t.Errorf("origin main holds %v, want both PRs", files)
	}
}

func TestIntegrationTrainWiresTheBatchTestAtTheStageOfThePolicy(t *testing.T) {
	t.Parallel()
	enforce := t.TempDir()
	writeL2Policy(t, enforce, enforceL2PolicyJSON)
	now := time.Date(2026, 11, 17, 5, 41, 12, 0, time.UTC)
	env := productionIntegrationEnv(enforce, now)
	if env.Run == nil {
		t.Fatal("the production env has no command port for the batch test")
	}
	l2 := trainOf(env, enforce).L2
	if l2.Composer.Stage != policy.FleetIntegrationEnforce || l2.Composer.Gate == nil || l2.Composer.Repo.Dir != enforce {
		t.Errorf("L2 composer = %+v, want the enforce stage of the policy on the root with a gate", l2.Composer)
	}
	if l2.Affected == nil || l2.Test == nil || l2.Predicates == nil || l2.Verdicts == nil {
		t.Errorf("L2 = %+v, want the three adapters, the affected set and the verdict store", l2)
	}
	var names []string
	for _, step := range l2.Pack {
		names = append(names, step.Name)
	}
	if want := []string{"repo-contract", "cover-changed", "cover-strict", "build", "vet", "gofmt"}; !slices.Equal(names, want) {
		t.Errorf("pack = %v, want %v", names, want)
	}
	bare := t.TempDir()
	if stage := trainOf(productionIntegrationEnv(bare, now), bare).L2.Composer.Stage; stage != policy.FleetIntegrationShadow {
		t.Errorf("L2 stage without a policy = %q, want %q", stage, policy.FleetIntegrationShadow)
	}
}
