package main

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

type trainGh struct {
	mu          sync.Mutex
	ready       string
	creates     int
	comments    map[string][]string
	closed      map[string]string
	onCreate    func()
	failClose   bool
	watchRC     int
	viewRC      int
	conclusion  string
	noRunYet    int
	conclusions []string
}

func (g *trainGh) run(_ context.Context, _ string, args ...string) (string, int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	joined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "pr list --head"):
		return "[]\n", 0, nil
	case strings.HasPrefix(joined, "pr list --state open --base"):
		return g.ready, 0, nil
	case strings.HasPrefix(joined, "pr create"):
		g.creates++
		if g.onCreate != nil {
			hook := g.onCreate
			g.onCreate = nil
			hook()
		}
		return "https://github.com/o/r/pull/99\n", 0, nil
	case strings.HasPrefix(joined, "run list"):
		if g.noRunYet > 0 {
			g.noRunYet--
			return "[]\n", 0, nil
		}
		return `[{"databaseId":123}]` + "\n", 0, nil
	case strings.HasPrefix(joined, "run watch"):
		return "", g.watchRC, nil
	case strings.HasPrefix(joined, "run view"):
		if g.viewRC != 0 {
			return "", g.viewRC, nil
		}
		conclusion := g.conclusion
		if len(g.conclusions) > 0 {
			conclusion, g.conclusions = g.conclusions[0], g.conclusions[1:]
		}
		return `{"conclusion":"` + conclusion + `","status":"completed"}` + "\n", 0, nil
	case strings.HasPrefix(joined, "pr comment"):
		g.comments[args[2]] = append(g.comments[args[2]], args[4])
	case strings.HasPrefix(joined, "pr close"):
		if g.failClose {
			return "", 1, nil
		}
		g.closed[args[2]] = args[4]
	}
	return "", 0, nil
}

type trainJourney struct {
	work   *gittest.Repo
	origin *gittest.Repo
	gh     *trainGh
	env    integrationEnv
}

func newTrainJourney(t *testing.T) *trainJourney {
	t.Helper()
	work := gittest.Fixture(t)
	work.Write("shared.md", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	work.Git("add", ".")
	work.Git("commit", "-q", "-m", "base")
	origin := gittest.Bare(t)
	work.Git("remote", "add", "origin", origin.Dir)
	work.Git("push", "-q", "origin", "main")
	gh := &trainGh{comments: map[string][]string{}, closed: map[string]string{}, conclusion: "success"}
	env := integrationEnv{Gh: gh.run, Git: integration.SysGit(), Root: work.Dir, Wave: "j1"}
	return &trainJourney{work: work, origin: origin, gh: gh, env: env}
}

func (j *trainJourney) pr(t *testing.T, n int, message, file, body string) {
	t.Helper()
	name := "pr-" + strconv.Itoa(n)
	j.work.Git("checkout", "-q", "-b", name, "main")
	j.work.Write(file, body)
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", message)
	j.work.Git("push", "-q", "origin", name+":refs/pull/"+strconv.Itoa(n)+"/head")
	j.work.Git("checkout", "-q", "main")
}

func (j *trainJourney) run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := runIntegrationEnv(j.env, args, &out, &errb)
	return rc, out.String(), errb.String()
}

func (j *trainJourney) originMain() string {
	return strings.TrimSpace(j.origin.Git("rev-parse", "refs/heads/main"))
}

func TestJourneyConsolePRTrainLandsTwoAndBriefsTheConflict(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", "a.md", "a\n")
	j.pr(t, 2, "feat: x", "shared.md", "X\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	j.pr(t, 3, "feat: y", "shared.md", "Y\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	for _, n := range []string{"1", "2", "3"} {
		if rc, _, errb := j.run("record-pr", "--pr", n, "--class", "docs-review"); rc != 0 {
			t.Fatalf("record-pr %s: rc=%d %s", n, rc, errb)
		}
	}
	before := j.originMain()

	rc, out, errb := j.run("batch", "--dry-run", "1", "2", "3")
	if rc != 0 || j.originMain() != before || j.gh.creates != 0 || len(j.gh.closed)+len(j.gh.comments) != 0 {
		t.Fatalf("dry run rc=%d pushed=%v creates=%d err=%s", rc, j.originMain() != before, j.gh.creates, errb)
	}
	for _, want := range []string{"order: 1 2 3", "eject PR #3 conflict", "shared.md", "batch: 2 PRs", "dry run"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run output lacks %q:\n%s", want, out)
		}
	}

	rc, out, errb = j.run("batch", "1", "2", "3")
	if rc != exitRefused || !strings.Contains(errb, "PR #3") {
		t.Fatalf("batch rc=%d out=%s err=%s, want a landing and a refusal that names PR #3", rc, out, errb)
	}
	after := j.originMain()
	if strings.TrimSpace(j.origin.Git("rev-list", "--count", before+".."+after)) != "2" || strings.TrimSpace(j.origin.Git("merge-base", before, after)) != before {
		t.Errorf("main must move by one fast-forward of two commits: %s -> %s", before, after)
	}
	if log := j.origin.Git("log", "--format=%B", before+".."+after); !strings.Contains(log, "Closes-Inbox: item-a") {
		t.Errorf("Closes-Inbox lost:\n%s", log)
	}
	for _, pr := range []string{"1", "2"} {
		if !strings.Contains(j.gh.closed[pr], "#99") {
			t.Errorf("PR %s close comment = %q, want a link to #99", pr, j.gh.closed[pr])
		}
	}
	if _, closed := j.gh.closed["3"]; closed || len(j.gh.comments["3"]) != 1 || !strings.Contains(j.gh.comments["3"][0], "shared.md") {
		t.Errorf("PR 3 must get a brief and stay open: closed=%v comments=%v", j.gh.closed, j.gh.comments)
	}
	if !strings.Contains(out, "landed: 1 2") {
		t.Errorf("output lacks the landed list:\n%s", out)
	}
	if tips := j.origin.Git("for-each-ref", "refs/pull/"); !slices.Contains(strings.Fields(tips), "refs/pull/1/head") {
		t.Errorf("a member branch must stay as it was:\n%s", tips)
	}
}

func TestJourneyConsolePRTrainRefusesAPRWithNoReviewBinding(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.pr(t, 7, "feat: b", "b.md", "b\n")
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatalf("record-pr: %s", errb)
	}
	before := j.originMain()
	rc, _, errb := j.run("batch", "1", "7")
	if rc != exitRefused || !strings.Contains(errb, "PR #7") || j.originMain() != before || j.gh.creates != 0 {
		t.Fatalf("rc=%d err=%q: want a refusal that names PR #7 and no push", rc, errb)
	}
}

func TestJourneyConsolePRTrainTakesTheReadyPRsOfTheHost(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 4, "feat: a", "a.md", "a\n")
	j.gh.ready = `[{"number":4,"headRefName":"feat/a"}]`
	if rc, _, errb := j.run("record-pr", "--pr", "4", "--class", "docs-review"); rc != 0 {
		t.Fatalf("record-pr: %s", errb)
	}
	rc, out, errb := j.run("batch", "--dry-run")
	if rc != 0 || !strings.Contains(out, "order: 4") {
		t.Fatalf("rc=%d out=%s err=%s", rc, out, errb)
	}
}

func TestIntegrationBatchUsageAndFailures(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	usage := [][]string{
		{"batch", "x"},
		{"batch", "--bogus"},
		{"batch", "--help"},
		{"record-pr"},
		{"record-pr", "--pr", "1"},
		{"record-pr", "--pr", "1", "--class", "c", "--attestation", "p"},
		{"record-pr", "--pr", "x", "--class", "c"},
		{"record-pr", "--help"},
		{"record-pr", "--pr", "1", "--class", "c", "extra"},
		{"record-pr", "--bogus"},
	}
	for _, args := range usage {
		rc, _, _ := j.run(args...)
		if args[len(args)-1] == "--help" {
			if rc != 0 {
				t.Errorf("%v: rc=%d, want 0", args, rc)
			}
			continue
		}
		if rc != exitUsage {
			t.Errorf("%v: rc=%d, want %d", args, rc, exitUsage)
		}
	}
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--attestation", filepath.Join(t.TempDir(), "none.json")); rc != exitRefused || errb == "" {
		t.Errorf("a missing attestation: rc=%d %q", rc, errb)
	}
	if rc, _, _ := j.run("record-pr", "--pr", "1", "--class", "Not A Class"); rc != exitRefused {
		t.Errorf("a bad class: rc=%d, want %d", rc, exitRefused)
	}
	if rc, _, _ := j.run("record-pr", "--pr", "99", "--class", "docs-review"); rc != exitIO {
		t.Errorf("a PR with no head: rc=%d, want %d", rc, exitIO)
	}
	j.gh.ready = "x\n"
	if rc, _, _ := j.run("batch"); rc != exitIO {
		t.Errorf("a host failure: rc=%d, want %d", rc, exitIO)
	}
}

func TestIntegrationBatchReportsAFailedLand(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatal(errb)
	}
	failing := j.env
	failing.Gh = func(ctx context.Context, dir string, args ...string) (string, int, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return "", 1, nil
		}
		return j.gh.run(ctx, dir, args...)
	}
	var out, errb bytes.Buffer
	if rc := runIntegrationEnv(failing, []string{"batch", "1"}, &out, &errb); rc != exitIO || !strings.Contains(errb.String(), "pr create") {
		t.Errorf("rc=%d err=%q", rc, errb.String())
	}
}
