package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

type chainFacts struct {
	base, first, second, change, store string
}

func (j *trainJourney) twoCommitPR(t *testing.T, n int) chainFacts {
	t.Helper()
	name := "pr-" + strconv.Itoa(n)
	j.work.Git("checkout", "-q", "-b", name, "main")
	j.work.Write("a.txt", "a\n")
	j.work.Git("add", "a.txt")
	j.work.Git("commit", "-q", "-m", "feat: a")
	j.work.Write("b.txt", "b\n")
	j.work.Git("add", "b.txt")
	j.work.Git("rm", "-q", "shared.md")
	j.work.Git("commit", "-q", "-m", "feat: b")
	j.work.Git("push", "-q", "origin", name+":refs/pull/"+strconv.Itoa(n)+"/head")
	j.work.Git("checkout", "-q", "main")
	f := chainFacts{first: strings.TrimSpace(j.work.Git("rev-parse", name+"~1")), second: strings.TrimSpace(j.work.Git("rev-parse", name))}
	f.base, _, f.change, f.store = j.prFacts(t, strconv.Itoa(n))
	return f
}

func (j *trainJourney) reviewChange(t *testing.T, parent, commit, id string) string {
	t.Helper()
	wt := j.worktreeAt(t, parent)
	j.restore(t, wt, commit)
	tokens := []string{j.seat(t, wt, "simp-"+id, "code-simplifier"), j.seat(t, wt, "rev-"+id, "code-reviewer")}
	if rc, logs := j.gate(t, wt, tokens...); rc != 0 {
		t.Fatalf("gate over %s rc=%d\n%s", id, rc, logs)
	}
	j.work.Git("worktree", "remove", "--force", wt)
	return j.changeOf(t, parent, commit)
}

func (j *trainJourney) changeOf(t *testing.T, parent, commit string) string {
	t.Helper()
	return integration.Digest([]byte(j.work.Git("diff", "--no-ext-diff", "--no-textconv", parent, commit) + "\n"))
}

func TestRecordPR_ATwoCommitPRReviewedPerCommitIsRecorded(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	f := j.twoCommitPR(t, 6)
	d1 := j.reviewChange(t, f.base, f.first, "c1")
	d2 := j.reviewChange(t, f.first, f.second, "c2")
	rc, out, errb := j.run("record-pr", "--pr", "6", "--reviewed")
	if rc != 0 || !strings.HasPrefix(out, "recorded PR #6: ") {
		t.Fatalf("record-pr rc=%d out=%q err=%q", rc, out, errb)
	}
	rec, ok := integration.FindPRRecord(j.work.Dir, 6)
	var digests []string
	for _, d := range []string{d1, d2} {
		raw, err := os.ReadFile(filepath.Join(f.store, d+".json"))
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, integration.Digest(raw))
	}
	want := integration.Digest([]byte(strings.Join(digests, "\n")))
	if !ok || rec.Audit.Kind != integration.KindCommitChain || rec.Audit.TreeSHA != f.change || rec.Audit.ArtifactSHA != want || rec.Audit.Class != "" {
		t.Fatalf("record = %+v, %v, want a commit chain over the change %s with the artifact %s", rec.Audit, ok, f.change, want)
	}
	rc, out, errb = j.run("batch", "--dry-run", "6")
	if rc != 0 || !strings.Contains(out, "batch: 1 PRs") {
		t.Errorf("batch dry run rc=%d out=%q err=%q", rc, out, errb)
	}
}

var nextRestore = regexp.MustCompile(`in a worktree at (\w+), run (git restore [^,]*? -- \.) ,`)

func TestRecordPR_AnUnreviewedSecondCommitIsRefusedWithItsNextStep(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	f := j.twoCommitPR(t, 6)
	j.reviewChange(t, f.base, f.first, "c1")
	d2 := j.changeOf(t, f.first, f.second)
	path := filepath.Join(f.store, d2+".json")
	want := j.refusal("6", "commit "+f.second[:7]+" (change "+d2[:7]+") has no passing review record: no review record at "+path+". Next: in a worktree at "+f.first[:7]+", run git restore --source="+f.second[:7]+" --staged --worktree -- . , run the review seats and evolve commit-gate run, then run record-pr again.")
	rc, out, errb := j.run("record-pr", "--pr", "6", "--reviewed")
	if rc != exitRefused || out != "" || errb != want {
		t.Fatalf("rc=%d out=%q\n got %q\nwant %q", rc, out, errb, want)
	}
	if _, ok := integration.FindPRRecord(j.work.Dir, 6); ok {
		t.Error("a refusal left a record")
	}
	m := nextRestore.FindStringSubmatch(errb)
	if m == nil {
		t.Fatalf("no next step in %q", errb)
	}
	wt := j.worktreeAt(t, m[1])
	args := strings.Fields(m[2])
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = wt
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", m[2], err, o)
	}
	tokens := []string{j.seat(t, wt, "simp-next", "code-simplifier"), j.seat(t, wt, "rev-next", "code-reviewer")}
	if rc, logs := j.gate(t, wt, tokens...); rc != 0 {
		t.Fatalf("gate after the next step rc=%d\n%s", rc, logs)
	}
	j.work.Git("worktree", "remove", "--force", wt)
	if rc, _, errb := j.run("record-pr", "--pr", "6", "--reviewed"); rc != 0 {
		t.Fatalf("record-pr after the next step rc=%d %s", rc, errb)
	}
}

func TestRecordPR_AWholeChangeReviewStillBindsAMultiCommitPR(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	f := j.twoCommitPR(t, 6)
	j.reviewChange(t, f.base, f.second, "whole")
	if rc, _, errb := j.run("record-pr", "--pr", "6", "--reviewed"); rc != 0 {
		t.Fatalf("record-pr rc=%d %s", rc, errb)
	}
	rec, ok := integration.FindPRRecord(j.work.Dir, 6)
	if !ok || rec.Audit.Kind != "" || rec.Audit.TreeSHA != f.change {
		t.Errorf("record = %+v, %v, want the whole-change binding with no kind", rec.Audit, ok)
	}
}

func TestRecordPR_ATwoCommitPRWithNoRecordNamesItsFirstCommit(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	f := j.twoCommitPR(t, 6)
	d1 := j.changeOf(t, f.base, f.first)
	rc, _, errb := j.run("record-pr", "--pr", "6", "--reviewed")
	prefix := "commit " + f.first[:7] + " (change " + d1[:7] + ") has no passing review record"
	if rc != exitRefused || !strings.Contains(errb, prefix) || strings.Contains(errb, f.second[:7]) {
		t.Errorf("rc=%d err=%q, want a refusal that names only the first commit %q", rc, errb, prefix)
	}
}

func storeChainRecord(t *testing.T, store, digest string, rec commitgate.ReviewRecord) {
	t.Helper()
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, digest+".json"), recordJSON(t, rec), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChainCheck_ARecordThatCoversAnotherChangeIsRefused(t *testing.T) {
	t.Parallel()
	store := t.TempDir()
	digest := strings.Repeat("e", 64)
	storeChainRecord(t, store, digest, commitgate.ReviewRecord{
		Attestation: commitgate.Attestation{TreeStateSHA: "other", ReviewRuns: []commitgate.ReviewRun{{RunID: "s1"}, {RunID: "r1"}}},
		LaunchRecords: []launchrecord.Record{
			launchFor("s1", "code-simplifier", "other"),
			launchFor("r1", "code-reviewer", "other"),
		},
	})
	_, err := chainCheck(store)(integration.CommitChange{DiffSHA: digest, Paths: []string{"a.go"}})
	want := `integration: the attestation covers the change "other", not the change "` + digest + `" of this PR`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestChainCheck_ADocsOnlyCommitBindsOnADocsReviewRecord(t *testing.T) {
	t.Parallel()
	store := t.TempDir()
	digest := strings.Repeat("d", 64)
	storeChainRecord(t, store, digest, commitgate.ReviewRecord{
		Attestation:   commitgate.Attestation{TreeStateSHA: digest, ReviewRuns: []commitgate.ReviewRun{{RunID: "d1"}}},
		LaunchRecords: []launchrecord.Record{launchFor("d1", "docs-review", digest)},
	})
	artifact, err := chainCheck(store)(integration.CommitChange{DiffSHA: digest, Paths: []string{"docs/a.md"}})
	if err != nil || artifact == "" {
		t.Fatalf("artifact = %q, err = %v, want a bound record", artifact, err)
	}
}
