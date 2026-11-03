package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	tightRatchetList = "{\n  \"internal/fixture.Big\": 64\n}\n"
	ratchetPRBranch  = ratchetBranchPrefix + "2026-10-01"
)

var (
	ciGreen   = ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}
	ciPending = ciwatch.RunStatus{Status: "in_progress"}
)

type fakeRatchetMerge struct {
	ci        ciwatch.RunStatus
	mergeOID  string
	failMerge bool
	onMerge   func()
	calls     []string
}

func (f *fakeRatchetMerge) install(t *testing.T) {
	t.Helper()
	prev := ratchetPRMerger
	t.Cleanup(func() { ratchetPRMerger = prev })
	ratchetPRMerger = func(string, io.Writer) (prMerger, error) {
		return prMerger{
			method: "merge", poll: time.Second, gh: f.gh, fetch: f.fetch,
			liveRuns: func() ([]runlease.LiveRun, error) { return nil, nil },
			now:      time.Now, sleep: func(time.Duration) {}, stdout: io.Discard, stderr: io.Discard,
		}, nil
	}
}

func (f *fakeRatchetMerge) fetch(_ context.Context, sha string) (ciwatch.RunStatus, error) {
	f.calls = append(f.calls, "ci "+sha)
	return f.ci, nil
}

func (f *fakeRatchetMerge) gh(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	if len(args) > 1 && args[1] == "merge" {
		if f.failMerge {
			return nil, errors.New("gh: the merge was refused")
		}
		if f.onMerge != nil {
			f.onMerge()
		}
		return nil, nil
	}
	return []byte(`{"state":"MERGED","mergeCommit":{"oid":"` + f.mergeOID + `"}}`), nil
}

func (f *fakeRatchetMerge) mergeCalls() []string {
	var merges []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "pr merge") {
			merges = append(merges, c)
		}
	}
	return merges
}

type ratchetPRFixture struct {
	ratchetOrigin
	tip   string
	gh    *fakeGH
	merge *fakeRatchetMerge
}

func newRatchetPRFixture(t *testing.T, prList, stage string) ratchetPRFixture {
	t.Helper()
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	return pushRatchetPR(t, o, "main", prList, stage)
}

func pushRatchetPR(t *testing.T, o ratchetOrigin, from, prList, stage string) ratchetPRFixture {
	t.Helper()
	o.plane.Git("checkout", "-q", "-b", "tighten", from)
	o.plane.Write(ratchetListPath, prList)
	o.plane.Git("commit", "-q", "-am", "an older tighten")
	o.plane.Git("push", "-q", "origin", "HEAD:refs/heads/"+ratchetPRBranch)
	o.plane.Git("checkout", "-q", "main")
	if stage != "" {
		writeRatchetPolicy(t, o.plane.Dir, `{"boundary":{"ratchet_merge":{"stage":"`+stage+`"}}}`)
	}
	tip := o.bare.Git("rev-parse", ratchetPRBranch)
	gh := &fakeGH{list: `[{"number":7,"headRefName":"` + ratchetPRBranch + `","headRefOid":"` + tip + `"}]`}
	gh.install(t)
	merge := &fakeRatchetMerge{ci: ciGreen, mergeOID: tip}
	merge.install(t)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	return ratchetPRFixture{ratchetOrigin: o, tip: tip, gh: gh, merge: merge}
}

func writeRatchetPolicy(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f ratchetPRFixture) mergeOnOrigin() {
	f.bare.Git("update-ref", "refs/heads/main", f.tip)
	f.gh.list = "[]"
}

func (f ratchetPRFixture) branchTip() string { return f.bare.Git("rev-parse", ratchetPRBranch) }

func TestBoundaryRatchetMerge_GreenGatesMergeThenRegenerate(t *testing.T) {
	f := newRatchetPRFixture(t, tightRatchetList, "enforce")
	f.merge.onMerge = f.mergeOnOrigin

	rc, out, errOut := f.tighten(t)

	merges := f.merge.mergeCalls()
	if rc != 0 || errOut != "" || len(merges) != 1 || merges[0] != "pr merge 7 --merge --match-head-commit "+f.tip {
		t.Fatalf("rc=%d stdout=%q stderr=%q merges=%q, want one head-bound merge of #7", rc, out, errOut, merges)
	}
	if !strings.Contains(f.merge.calls[0], "ci "+f.tip) {
		t.Errorf("calls %q, want the CI check on the head before the merge", f.merge.calls)
	}
	merged, tight := strings.Index(out, "ratchet merge: merged #7 "+f.tip), strings.Index(out, "ratchet tighten: tight")
	if merged < 0 || tight < merged {
		t.Errorf("stdout %q, want the merge line and then the regenerated tight report", out)
	}
	if f.originHead() != f.tip || f.gh.created() != nil {
		t.Errorf("origin main %s, create %q: want main at the merged tip and no new pull request", f.originHead(), f.gh.created())
	}
	f.planeIsUntouched(t)
}

func TestBoundaryRatchetMerge_ARedScanHoldsTheMergeWarnsAndReplacesTheTip(t *testing.T) {
	f := newRatchetPRFixture(t, "{\n  \"internal/fixture.Big\": 60\n}\n", "enforce")
	main := f.originHead()

	rc, out, errOut := f.tighten(t)

	if rc != 0 || len(f.merge.mergeCalls()) != 0 || !strings.Contains(errOut, "WARN: ratchet merge: #7") || !strings.Contains(errOut, "function-size ratchet violated") {
		t.Fatalf("rc=%d stdout=%q stderr=%q merges=%q, want a WARN for the red scan and no merge", rc, out, errOut, f.merge.mergeCalls())
	}
	if f.branchTip() == f.tip || !strings.Contains(out, "updated pull request #7") {
		t.Errorf("stdout %q, want the tip of #7 replaced", out)
	}
	f.assertProposed(t, ratchetPRBranch, main)
}

func TestBoundaryRatchetMerge_CINotGreenHoldsTheMerge(t *testing.T) {
	for name, ci := range map[string]ciwatch.RunStatus{
		"pending": ciPending,
		"failed":  {Status: ciwatch.StatusCompleted, Conclusion: "failure", FailingJobs: []string{"go-test"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRatchetPRFixture(t, tightRatchetList, "enforce")
			f.merge.ci = ci
			main := f.originHead()

			rc, out, errOut := f.tighten(t)

			if rc != 0 || errOut != "" || len(f.merge.mergeCalls()) != 0 || !strings.Contains(out, "ratchet merge: #7 held: required CI") {
				t.Errorf("rc=%d stdout=%q stderr=%q merges=%q, want a held line and no merge", rc, out, errOut, f.merge.mergeCalls())
			}
			if f.originHead() != main {
				t.Errorf("origin main moved without a merge")
			}
		})
	}
}

func TestBoundaryRatchetMerge_ShadowLogsAWouldMergeLineAndNeverMerges(t *testing.T) {
	for name, stage := range map[string]string{"no policy key": "", "an explicit shadow": "shadow"} {
		t.Run(name, func(t *testing.T) {
			f := newRatchetPRFixture(t, tightRatchetList, stage)
			main := f.originHead()

			rc, out, errOut := f.tighten(t)

			if rc != 0 || errOut != "" || !strings.Contains(out, "ratchet merge: would merge #7 at "+shortSHA(f.tip)) {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want a would-merge line", rc, out, errOut)
			}
			if merges := f.merge.mergeCalls(); len(merges) != 0 || f.originHead() != main {
				t.Errorf("merges %q, want no merge call in shadow", merges)
			}
		})
	}
}

func TestBoundaryRatchetMerge_OffEvaluatesNoGate(t *testing.T) {
	f := newRatchetPRFixture(t, tightRatchetList, "off")

	rc, out, errOut := f.tighten(t)

	if rc != 0 || errOut != "" || len(f.merge.calls) != 0 || strings.Contains(out, "ratchet merge") {
		t.Errorf("rc=%d stdout=%q stderr=%q calls=%q, want no gate at stage off", rc, out, errOut, f.merge.calls)
	}
}

func TestBoundaryRatchetMerge_AForkPullRequestIsNeverMerged(t *testing.T) {
	f := newRatchetPRFixture(t, tightRatchetList, "enforce")
	f.gh.list = `[{"number":7,"headRefName":"` + ratchetPRBranch + `","headRefOid":"` + f.tip + `","isCrossRepository":true}]`

	rc, out, errOut := f.tighten(t)

	if rc != 0 || errOut != "" || len(f.merge.calls) != 0 || f.originHead() == f.tip {
		t.Fatalf("rc=%d stdout=%q stderr=%q calls=%q, want the fork pull request never merged", rc, out, errOut, f.merge.calls)
	}
	if branch := flagAfter(f.gh.created(), "--head"); branch == "" || branch == ratchetPRBranch {
		t.Errorf("gh pr create --head %q, want a new pull request from the step's own branch", branch)
	}
}

func TestBoundaryRatchetMerge_AnUnchangedListIsNotForcePushed(t *testing.T) {
	f := newRatchetPRFixture(t, tightRatchetList, "enforce")
	f.merge.ci = ciPending

	rc, out, errOut := f.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "ratchet tighten: #7 already holds this list") || strings.Contains(out, "updated pull request") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want the unchanged list reported and no update", rc, out, errOut)
	}
	if f.branchTip() != f.tip {
		t.Errorf("%s moved to %s: an unchanged list must not restart CI with a force-push", ratchetPRBranch, f.branchTip())
	}
	f.planeIsUntouched(t)
}

func TestBoundaryRatchetMerge_AGateFailureWarnsAndHoldsTheMerge(t *testing.T) {
	cases := []struct {
		name    string
		prep    func(t *testing.T) ratchetPRFixture
		wantErr string
	}{
		{"a head that moved", func(t *testing.T) ratchetPRFixture {
			f := newRatchetPRFixture(t, tightRatchetList, "enforce")
			f.gh.list = `[{"number":7,"headRefName":"` + ratchetPRBranch + `","headRefOid":"` + strings.Repeat("0", 40) + `"}]`
			return f
		}, "moved"},
		{"a branch that is gone", func(t *testing.T) ratchetPRFixture {
			f := newRatchetPRFixture(t, tightRatchetList, "enforce")
			f.gh.list = `[{"number":7,"headRefName":"` + ratchetBranchPrefix + `gone","headRefOid":"` + f.tip + `"}]`
			return f
		}, "fetch"},
		{"a base off origin main", func(t *testing.T) ratchetPRFixture {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			o.plane.Git("checkout", "-q", "-b", "side")
			o.plane.Git("commit", "-q", "--allow-empty", "-m", "a commit that never landed")
			o.plane.Git("checkout", "-q", "main")
			return pushRatchetPR(t, o, "side", tightRatchetList, "enforce")
		}, "is not on origin/main"},
		{"a list overlay that cannot be checked out", func(t *testing.T) ratchetPRFixture {
			f := newRatchetPRFixture(t, tightRatchetList, "enforce")
			f.gh.failGit = "checkout"
			return f
		}, "git checkout refused"},
		{"a refused merge", func(t *testing.T) ratchetPRFixture {
			f := newRatchetPRFixture(t, tightRatchetList, "enforce")
			f.merge.failMerge = true
			return f
		}, "the merge was refused"},
		{"an unknown stage", func(t *testing.T) ratchetPRFixture {
			return newRatchetPRFixture(t, tightRatchetList, "sometimes")
		}, "boundary.ratchet_merge.stage"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := c.prep(t)
			main := f.originHead()

			rc, _, errOut := f.tighten(t)

			if rc != 0 || !strings.Contains(errOut, "WARN: ratchet merge: #7") || !strings.Contains(errOut, c.wantErr) {
				t.Errorf("rc=%d stderr=%q, want a ratchet merge WARN naming %q", rc, errOut, c.wantErr)
			}
			if f.originHead() != main {
				t.Errorf("origin main moved after a held merge")
			}
		})
	}
}

func TestBoundaryRatchetMerge_AFailedRefreshAfterTheMergeWarns(t *testing.T) {
	for name, after := range map[string]func(f ratchetPRFixture){
		"no origin to fetch": func(f ratchetPRFixture) { f.plane.Git("remote", "remove", "origin") },
		"gh cannot relist":   func(f ratchetPRFixture) { f.gh.failVerb = "list" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newRatchetPRFixture(t, tightRatchetList, "enforce")
			f.merge.onMerge = func() { f.mergeOnOrigin(); after(f) }

			rc, out, errOut := f.tighten(t)

			if rc != 0 || !strings.Contains(out, "ratchet merge: merged #7") || !strings.Contains(errOut, "WARN: ratchet tighten") {
				t.Errorf("rc=%d stdout=%q stderr=%q, want the merge reported and a tighten WARN", rc, out, errOut)
			}
		})
	}
}

func TestRatchetMergeGateFor_ReadsTheStageAndBuildsTheMergerOfPRMerge(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		body      string
		want      config.Stage
		wantErr   string
		wantMerge bool
	}{
		{"no policy", "", config.StageShadow, "", true},
		{"enforce", `{"boundary":{"ratchet_merge":{"stage":"enforce"}}}`, config.StageEnforce, "", true},
		{"off builds no merger", `{"boundary":{"ratchet_merge":{"stage":"off"}}}`, config.StageOff, "", false},
		{"a bad merge method", `{"pr":{"merge_method":"fast"}}`, config.StageShadow, "pr.merge_method", false},
		{"a policy that does not parse", `{"boundary":{"ratchet_merge":{"stag":"on"}}}`, config.StageOff, "boundary.ratchet_merge.stag", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755); err != nil {
				t.Fatal(err)
			}
			if c.body != "" {
				writeRatchetPolicy(t, root, c.body)
			}

			g, err := ratchetMergeGateFor(root, io.Discard)

			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err %v, want one naming %s", err, c.wantErr)
				}
				return
			}
			if err != nil || g.stage != c.want || (g.merger.gh != nil) != c.wantMerge || (g.merger.method == "merge") != c.wantMerge {
				t.Errorf("gate %+v, err %v, want stage %v and a pr merge merger %v", g, err, c.want, c.wantMerge)
			}
		})
	}
}

func TestPRMerger_MergeHeadIsTheMergeStepOfPRMerge(t *testing.T) {
	t.Parallel()
	gh := &fakePRGitHub{merged: map[string]string{"7": `{"state":"MERGED","mergeCommit":{"oid":"` + mergeSHA + `"}}`}}
	m, _, _ := greenMerger(gh, nil, nil)
	sha, err := m.mergeHead(context.Background(), "7", "h7")
	if err != nil || sha != mergeSHA || !strings.Contains(strings.Join(gh.calls, "\n"), "pr merge 7 --squash --match-head-commit h7") {
		t.Fatalf("mergeHead = %q, %v, calls %q", sha, err, gh.calls)
	}
	live, _, _ := greenMerger(gh, nil, []runlease.LiveRun{{Dir: "/runs/cycle-9", Lease: runlease.Lease{RunID: "cycle-9"}}})
	var refusal prRefusal
	if _, err := live.mergeHead(context.Background(), "7", "h7"); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "cycle-9") {
		t.Errorf("mergeHead under a live run = %v, want a refusal naming the run", err)
	}
}

func TestBoundaryRatchetMerge_AMergeWithNoMergeCommitReadRelistsAndNeverPushesTheMergedBranch(t *testing.T) {
	f := newRatchetPRFixture(t, "{\n  \"internal/fixture.Big\": 70\n}\n", "enforce")
	f.merge.mergeOID = "unknown"
	f.merge.onMerge = func() {
		f.bare.Git("update-ref", "refs/heads/main", f.tip)
		f.gh.list = "[]"
	}

	rc, out, errOut := f.tighten(t)

	if rc != 0 || !strings.Contains(errOut, "WARN: ratchet merge: #7") || !strings.Contains(out, "opened a pull request") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want a WARN and a new pull request after the relist", rc, out, errOut)
	}
	if f.branchTip() != f.tip {
		t.Errorf("%s moved to %s: the step must not push onto the branch of a merged pull request", ratchetPRBranch, f.branchTip())
	}
}
