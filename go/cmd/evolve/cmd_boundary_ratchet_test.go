package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const ratchetListPath = "go/" + sizeratchet.OffendersRelPath

type fakeGH struct {
	list     string
	failVerb string
	lease    string
	calls    [][]string
}

func (f *fakeGH) install(t *testing.T) {
	t.Helper()
	prev := ratchetRun
	t.Cleanup(func() { ratchetRun = prev })
	ratchetRun = f.run
}

func (f *fakeGH) run(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if name == "git" && len(args) > 0 && args[0] == "ls-remote" && f.lease != "" {
		fmt.Fprintf(stdout, "%s\t%s\n", f.lease, args[len(args)-1])
		return 0, nil
	}
	if name != "gh" {
		return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
	f.calls = append(f.calls, args)
	if len(args) > 1 && args[1] == f.failVerb {
		fmt.Fprintln(stderr, "gh "+f.failVerb+" refused")
		return 1, nil
	}
	if len(args) > 1 && args[1] == "list" {
		fmt.Fprint(stdout, listForBase(f.list, flagAfter(args, "--base")))
	}
	return 0, nil
}

func listForBase(list, base string) string {
	var prs []map[string]any
	if base == "" || json.Unmarshal([]byte(list), &prs) != nil {
		return list
	}
	kept := []map[string]any{}
	for _, pr := range prs {
		if got, ok := pr["baseRefName"].(string); !ok || got == base {
			kept = append(kept, pr)
		}
	}
	out, _ := json.Marshal(kept)
	return string(out)
}

func (f *fakeGH) created() []string {
	for _, c := range f.calls {
		if len(c) > 1 && c[1] == "create" {
			return c
		}
	}
	return nil
}

func flagAfter(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

type ratchetOrigin struct {
	bare  *gittest.Repo
	plane *gittest.Repo
}

const rawGitBaselinePath = "go/" + rawgitratchet.BaselineRelPath

func newRatchetOrigin(t *testing.T, funcLines int, offenders string) ratchetOrigin {
	t.Helper()
	return newRatchetOriginWith(t, funcLines, offenders, `{"internal/fixture/raw_test.go": 1}`)
}

func newRatchetOriginWith(t *testing.T, funcLines int, offenders, rawBaseline string) ratchetOrigin {
	t.Helper()
	seed := gittest.Fixture(t)
	seed.Write("go/internal/fixture/raw_test.go", "package fixture\n\nimport \"testing\"\n\nfunc TestRaw(t *testing.T) {\n\t_ = []string{"+strconv.Quote("gi"+"t")+", "+strconv.Quote("in"+"it")+"}\n}\n")
	seed.Write(rawGitBaselinePath, rawBaseline)
	var src strings.Builder
	src.WriteString("package fixture\n\nfunc Big() int {\n\ttotal := 0\n")
	for i := 0; i < funcLines; i++ {
		fmt.Fprintf(&src, "\ttotal += %d\n", i)
	}
	src.WriteString("\treturn total\n}\n")
	seed.Write("go/internal/fixture/big.go", src.String())
	if offenders != "" {
		seed.Write(ratchetListPath, offenders)
	}
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	bare := gittest.Bare(t)
	seed.Git("remote", "add", "origin", bare.Dir)
	seed.Git("push", "-q", "origin", "main")
	return ratchetOrigin{bare: bare, plane: gittest.Clone(t, bare.Dir)}
}

func (o ratchetOrigin) tighten(t *testing.T) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runBoundaryRatchetTighten([]string{"--project-root", o.plane.Dir}, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func (o ratchetOrigin) originHead() string { return o.bare.Git("rev-parse", "main") }

func (o ratchetOrigin) planeIsUntouched(t *testing.T) {
	t.Helper()
	if status := o.plane.Git("status", "--porcelain"); status != "" {
		t.Errorf("the plane tree is dirty after the step: %q", status)
	}
	if list := o.plane.Git("worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("the step left a worktree behind:\n%s", list)
	}
}

func (o ratchetOrigin) assertProposed(t *testing.T, branch, main string) {
	t.Helper()
	if got := o.bare.Git("show", branch+":"+ratchetListPath); got != "{\n  \"internal/fixture.Big\": 64\n}" {
		t.Errorf("%s holds %q, want the tightened list", branch, got)
	}
	if parent := o.bare.Git("rev-parse", branch+"^"); parent != main {
		t.Errorf("%s^ = %s, want origin main %s: one commit on top of main", branch, parent, main)
	}
	if files := o.bare.Git("show", "--name-only", "--format=", branch); files != ratchetListPath {
		t.Errorf("the commit touches %q, want only %s", files, ratchetListPath)
	}
	if got := o.originHead(); got != main {
		t.Errorf("origin main moved to %s: the step never pushes to main", got)
	}
	o.planeIsUntouched(t)
}

func TestBoundaryRatchetTighten_OpensAPullRequestFromADatedBranch(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90, "internal/fixture.Gone": 60}`)
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	main := o.originHead()

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "internal/fixture.Big: 90 -> 64") || !strings.Contains(out, "internal/fixture.Gone: 60 removed") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want each change named and no warning", rc, out, errOut)
	}
	create := gh.created()
	branch := flagAfter(create, "--head")
	if !strings.HasPrefix(branch, ratchetBranchPrefix+time.Now().UTC().Format("2006-01")) || flagAfter(create, "--base") != "main" ||
		flagAfter(create, "--title") != ratchetTightenMessage || !strings.Contains(flagAfter(create, "--body"), "internal/fixture.Big: 90 -> 64") {
		t.Fatalf("gh pr create argv %q, want a PR from a dated %s branch into main that lists the changes", create, ratchetBranchPrefix)
	}
	if !strings.Contains(out, "opened a pull request from "+branch) {
		t.Errorf("stdout %q, want the opened branch named", out)
	}
	o.assertProposed(t, branch, main)
}

func TestBoundaryRatchetTighten_LowersTheRawGitBaselineInTheSameTightenPullRequest(t *testing.T) {
	o := newRatchetOriginWith(t, 60, `{"internal/fixture.Big": 90}`, `{"internal/fixture/raw_test.go": 3, "internal/fixture/gone_test.go": 2}`)
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, out, errOut := o.tighten(t)

	branch := flagAfter(gh.created(), "--head")
	body := flagAfter(gh.created(), "--body")
	if rc != 0 || errOut != "" || !strings.Contains(body, "internal/fixture.Big: 90 -> 64") || !strings.Contains(body, "internal/fixture/raw_test.go: 3 -> 1") || !strings.Contains(out, "internal/fixture/gone_test.go: 2 removed") {
		t.Fatalf("rc=%d stdout=%q stderr=%q body=%q, want both lists tightened in one pull request", rc, out, errOut, body)
	}
	if got := o.bare.Git("show", branch+":"+rawGitBaselinePath); got != "{\n  \"internal/fixture/raw_test.go\": 1\n}" {
		t.Errorf("%s holds the baseline %q, want it lowered", branch, got)
	}
	if files := o.bare.Git("show", "--name-only", "--format=", branch); files != ratchetListPath+"\n"+rawGitBaselinePath && files != rawGitBaselinePath+"\n"+ratchetListPath {
		t.Errorf("the commit touches %q, want both lists in one commit", files)
	}
}

func TestBoundaryRatchetTighten_AListThatCannotBeWrittenWarnsAndPushesNothing(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	prev := ratchetWrite
	t.Cleanup(func() { ratchetWrite = prev })
	ratchetWrite = func(string, []byte, os.FileMode) error { return errors.New("disk full") }
	main := o.originHead()

	rc, _, errOut := o.tighten(t)

	if rc != 0 || !strings.Contains(errOut, "disk full") || gh.created() != nil || o.originHead() != main {
		t.Errorf("rc=%d stderr=%q created=%v, want a warning and no pull request", rc, errOut, gh.created())
	}
}

func TestBoundaryRatchetTighten_OnlyTheRawGitBaselineCanBeLoose(t *testing.T) {
	o := newRatchetOriginWith(t, 60, `{"internal/fixture.Big": 64}`, `{"internal/fixture/raw_test.go": 2}`)
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, _, errOut := o.tighten(t)

	branch := flagAfter(gh.created(), "--head")
	if files := o.bare.Git("show", "--name-only", "--format=", branch); rc != 0 || errOut != "" || files != rawGitBaselinePath {
		t.Errorf("rc=%d stderr=%q, the commit touches %q, want only %s", rc, errOut, files, rawGitBaselinePath)
	}
}

func TestBoundaryRatchetTighten_UpdatesTheOpenTightenPullRequest(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	const branch = ratchetBranchPrefix + "2026-10-01"
	o.plane.Git("checkout", "-q", "-b", "stale")
	o.plane.Git("commit", "-q", "--allow-empty", "-m", "an older tighten")
	o.plane.Git("push", "-q", "origin", "HEAD:refs/heads/"+branch)
	o.plane.Git("checkout", "-q", "main")
	gh := &fakeGH{list: `[{"number":12,"headRefName":"feat/other"},{"number":9,"headRefName":"chore/ratchet-tighten-fork","isCrossRepository":true},{"number":7,"headRefName":"` + branch + `"}]`}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	main := o.originHead()

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "updated pull request #7 ("+branch+")") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want the open PR updated", rc, out, errOut)
	}
	if create := gh.created(); create != nil {
		t.Errorf("gh pr create ran (%q): an open tighten PR must be updated, never duplicated", create)
	}
	o.assertProposed(t, branch, main)
}

func TestBoundaryRatchetTighten_ATightListLandsNothing(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 64}`)
	gh := &fakeGH{list: "[]"}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	before := o.originHead()

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "ratchet tighten: tight") || o.originHead() != before {
		t.Fatalf("rc=%d stdout=%q stderr=%q origin moved=%v, want a tight report and no commit", rc, out, errOut, o.originHead() != before)
	}
	if len(gh.calls) != 0 {
		t.Errorf("gh ran %q for a tight list", gh.calls)
	}
	o.planeIsUntouched(t)
}

func TestBoundaryRatchetTighten_AFailureWarnsAndNeverStopsTheBoundary(t *testing.T) {
	cases := []struct {
		name    string
		prep    func(t *testing.T) ratchetOrigin
		gh      fakeGH
		wantErr string
	}{
		{"gh cannot list the pull requests", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
		}, fakeGH{failVerb: "list"}, "gh list refused"},
		{"gh lists something that is not JSON", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
		}, fakeGH{list: "not json"}, "gh pr list"},
		{"gh cannot open the pull request", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
		}, fakeGH{list: "[]", failVerb: "create"}, "gh create refused"},
		{"a red ratchet on main is not pushed", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, `{"internal/fixture.Big": 60, "internal/fixture.Gone": 60}`)
		}, fakeGH{list: "[]"}, "function-size ratchet violated"},
		{"a module without the list", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, "")
		}, fakeGH{list: "[]"}, "read offender list"},
		{"a refused push", func(t *testing.T) ratchetOrigin {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			hook := filepath.Join(o.bare.Dir, "hooks", "pre-receive")
			if err := os.WriteFile(hook, []byte("#!/bin/sh\necho protected >&2\nexit 1\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			return o
		}, fakeGH{list: "[]"}, "protected"},
		{"a plane without origin", func(t *testing.T) ratchetOrigin {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			o.plane.Git("remote", "remove", "origin")
			return o
		}, fakeGH{list: "[]"}, "fetch"},
		{"a live cycle", func(t *testing.T) ratchetOrigin {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			runDir := filepath.Join(o.plane.Dir, ".evolve", "runs", "cycle-7")
			if err := os.MkdirAll(runDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
				t.Fatal(err)
			}
			return o
		}, fakeGH{list: "[]"}, "cycle-7"},
	}
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := c.prep(t)
			gh := c.gh
			gh.install(t)
			before := o.originHead()

			rc, _, errOut := o.tighten(t)

			if rc != 0 || !strings.Contains(errOut, "WARN") || !strings.Contains(errOut, c.wantErr) {
				t.Errorf("rc=%d stderr=%q, want rc 0 and a WARN naming %q", rc, errOut, c.wantErr)
			}
			if o.originHead() != before {
				t.Errorf("origin main moved after a failed tighten")
			}
			if c.gh.failVerb != "create" {
				if heads := o.bare.Git("for-each-ref", "--format=%(refname)", "refs/heads/"+ratchetBranchPrefix); heads != "" {
					t.Errorf("a tighten branch was pushed after a failure: %s", heads)
				}
			}
		})
	}
}

func TestBoundaryRatchetTighten_ANoTempDirWarns(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))

	rc, _, errOut := o.tighten(t)

	if rc != 0 || !strings.Contains(errOut, "WARN") {
		t.Errorf("rc=%d stderr=%q, want a WARN for an unusable temp dir", rc, errOut)
	}
}

func TestBoundaryRatchetTighten_NeedsTheProjectRoot(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if rc := runBoundaryRatchetTighten(nil, nil, &bytes.Buffer{}, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), "--project-root") {
		t.Errorf("rc=%d stderr=%q, want a usage exit naming --project-root", rc, stderr.String())
	}
}

func TestBoundaryHaltSteps_TightenBeforeSyncMainSoThePlaneMergesIt(t *testing.T) {
	t.Parallel()
	steps := boundaryHaltSteps(boundaryArgs{prs: []string{"12"}}, "/plane")
	var verbs []string
	for _, s := range steps {
		verbs = append(verbs, s.verb)
	}
	if strings.Join(verbs, " ") != "loop-stop pr "+boundaryRatchetVerb+" sync-main inbox quality gc" {
		t.Fatalf("halt steps %v, want the tighten after the PR merges and before sync-main", verbs)
	}
	if s := steps[2]; s.String() != boundaryRatchetDesc || strings.Join(s.args, " ") != "--project-root /plane" {
		t.Errorf("tighten step %q args %v", s.String(), s.args)
	}
	var stderr bytes.Buffer
	if rc := dispatchBoundaryVerb(boundaryRatchetVerb, nil, &bytes.Buffer{}, &stderr); rc != exitUsage {
		t.Errorf("the boundary dispatch has no %s handler: rc=%d stderr=%q", boundaryRatchetVerb, rc, stderr.String())
	}
}

func TestBoundaryRatchetTighten_AMovedTightenBranchIsNotOverwritten(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	const branch = ratchetBranchPrefix + "2026-10-01"
	o.plane.Git("checkout", "-q", "-b", "stale")
	o.plane.Git("commit", "-q", "--allow-empty", "-m", "a commit the step did not read")
	o.plane.Git("push", "-q", "origin", "HEAD:refs/heads/"+branch)
	o.plane.Git("checkout", "-q", "main")
	tip := o.bare.Git("rev-parse", branch)
	gh := &fakeGH{list: `[{"number":7,"headRefName":"` + branch + `"}]`, lease: o.originHead()}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")

	rc, _, errOut := o.tighten(t)

	if rc != 0 || !strings.Contains(errOut, "WARN") || !strings.Contains(errOut, "stale info") {
		t.Errorf("rc=%d stderr=%q, want a WARN for the lease refusal", rc, errOut)
	}
	if got := o.bare.Git("rev-parse", branch); got != tip {
		t.Errorf("%s moved to %s: a branch that moved after ls-remote must not be overwritten", branch, got)
	}
}

func TestBoundaryRatchetTighten_ATightenBranchIntoAnotherBaseIsLeftAlone(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	const other = ratchetBranchPrefix + "release"
	o.plane.Git("checkout", "-q", "-b", "release-work")
	o.plane.Git("commit", "-q", "--allow-empty", "-m", "a tighten aimed at a release branch")
	o.plane.Git("push", "-q", "origin", "HEAD:refs/heads/"+other)
	o.plane.Git("checkout", "-q", "main")
	tip := o.bare.Git("rev-parse", other)
	gh := &fakeGH{list: `[{"number":5,"headRefName":"` + other + `","baseRefName":"release-1"}]`}
	gh.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	main := o.originHead()

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "opened a pull request") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want a new pull request into main", rc, out, errOut)
	}
	if got := o.bare.Git("rev-parse", other); got != tip {
		t.Errorf("%s moved to %s: a tighten PR into another base must not be reused", other, got)
	}
	branch := flagAfter(gh.created(), "--head")
	if branch == other || branch == "" {
		t.Fatalf("gh pr create --head %q, want a new dated branch", branch)
	}
	o.assertProposed(t, branch, main)
}
