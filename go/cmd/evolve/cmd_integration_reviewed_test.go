package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/guardcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/treestate"
)

func (j *trainJourney) prFacts(t *testing.T, n string) (base, head, change, store string) {
	t.Helper()
	base = strings.TrimSpace(j.work.Git("rev-parse", "main"))
	head = strings.TrimSpace(j.work.Git("rev-parse", "pr-"+n))
	diff := j.work.Git("diff", "--no-ext-diff", "--no-textconv", base, head) + "\n"
	store, err := commitgate.ReviewStoreDir(context.Background(), sysexec.DefaultRunner, j.work.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return base, head, integration.Digest([]byte(diff)), store
}

func (j *trainJourney) refusal(pr, reason string) string {
	return "evolve integration: integration: PR #" + pr + " cannot record a review binding: " + reason + "\n"
}

func TestJourneyRecordPRReviewedRefusesAPRWithNoRecordAndNamesTheNextStep(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 5, "feat: a", "a.txt", "a\n")
	base, head, change, store := j.prFacts(t, "5")
	want := j.refusal("5", "no review record for its whole change "+change[:7]+" ("+base[:7]+".."+head[:7]+"), looked in "+store+". Next: in a worktree at "+base[:7]+", run git restore --source="+head[:7]+" --staged --worktree -- . , run the review seats and evolve commit-gate run, then run record-pr again.")
	rc, out, errb := j.run("record-pr", "--pr", "5", "--reviewed")
	if rc != exitRefused || out != "" || errb != want {
		t.Errorf("rc=%d out=%q\n got %q\nwant %q", rc, out, errb, want)
	}
	if _, ok := integration.FindPRRecord(j.work.Dir, 5); ok {
		t.Error("a refusal left a record")
	}
}

func (j *trainJourney) worktreeAt(t *testing.T, ref string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "wt")
	j.work.Git("worktree", "add", "-q", "--detach", dir, ref)
	return dir
}

func (j *trainJourney) seat(t *testing.T, wt, id, profile string) string {
	t.Helper()
	tree, err := treestate.SHA(context.Background(), sysexec.DefaultRunner, wt, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := launchrecord.Record{
		RunID: id, Profile: profile, CLI: "agy-tmux", Family: "agy", Model: "m", Verdict: "PASS",
		TreeBefore: tree, TreeAfter: tree, ArtifactSHA256: "a1", TS: "2026-11-05T09:00:00Z",
	}
	if _, err := launchrecord.Write(filepath.Join(wt, ".evolve"), rec); err != nil {
		t.Fatal(err)
	}
	return "run:" + id
}

func (j *trainJourney) gate(t *testing.T, wt string, tokens ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := guardcmd.RunCommitGate([]string{"run", "--project-root", wt, "--no-install", "--reviewers", strings.Join(tokens, ",")}, nil, &out, &errb)
	return rc, errb.String()
}

func (j *trainJourney) restore(t *testing.T, wt, source string) {
	t.Helper()
	cmd := exec.Command("git", "restore", "--source="+source, "--staged", "--worktree", "--", ".")
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git restore: %v\n%s", err, out)
	}
}

func TestJourneyRecordPRReviewedBindsAPRThatTheRealGateReviewedInAWorktree(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 9, "feat: a", "a.txt", "a\n")
	wt := j.worktreeAt(t, "main")
	j.restore(t, wt, "pr-9")
	tokens := []string{j.seat(t, wt, "simp-9", "code-simplifier"), j.seat(t, wt, "rev-9", "code-reviewer")}
	if rc, logs := j.gate(t, wt, tokens...); rc != 0 {
		t.Fatalf("gate rc=%d\n%s", rc, logs)
	}
	j.work.Git("worktree", "remove", "--force", wt)
	rc, out, errb := j.run("record-pr", "--pr", "9", "--reviewed")
	if rc != 0 || !strings.HasPrefix(out, "recorded PR #9: ") {
		t.Fatalf("record-pr rc=%d out=%q err=%q", rc, out, errb)
	}
	rec, ok := integration.FindPRRecord(j.work.Dir, 9)
	_, _, change, _ := j.prFacts(t, "9")
	_, _, _, store := j.prFacts(t, "9")
	raw, err := os.ReadFile(filepath.Join(store, change+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok || rec.Audit.TreeSHA != change || rec.Audit.ArtifactSHA != integration.Digest(raw) {
		t.Fatalf("record = %+v, %v, want a binding to the whole change %s and the digest of its record file", rec, ok, change)
	}
	rc, out, errb = j.run("batch", "--dry-run", "9")
	if rc != 0 || !strings.Contains(out, "batch: 1 PRs") {
		t.Errorf("batch dry run rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestJourneyRecordPRReviewedRefusesAGateRunThatNamedAPlainReviewer(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 9, "feat: a", "a.txt", "a\n")
	wt := j.worktreeAt(t, "main")
	j.restore(t, wt, "pr-9")
	if rc, logs := j.gate(t, wt, j.seat(t, wt, "simp-9", "code-simplifier"), "code-reviewer"); rc != 0 {
		t.Fatalf("gate rc=%d\n%s", rc, logs)
	}
	rc, out, errb := j.run("record-pr", "--pr", "9", "--reviewed")
	if want := "prove no review (profiles: code-simplifier)"; rc != exitRefused || out != "" || !strings.Contains(errb, want) {
		t.Errorf("rc=%d out=%q err=%q, want a refusal that holds %q", rc, out, errb, want)
	}
}

var nextRestore = regexp.MustCompile(`in a worktree at (\w+), run (git restore [^,]*? -- \.) ,`)

func TestJourneyRecordPRReviewedNCommitsNeedsOneReviewOfTheWholeChange(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 6, "feat: a", "a.txt", "a\n")
	j.work.Git("checkout", "-q", "pr-6")
	j.work.Write("b.txt", "b\n")
	j.work.Git("add", "b.txt")
	j.work.Git("rm", "-q", "shared.md")
	j.work.Git("commit", "-q", "-m", "feat: b")
	j.work.Git("push", "-q", "origin", "pr-6:refs/pull/6/head")
	j.work.Git("checkout", "-q", "main")
	first := strings.TrimSpace(j.work.Git("rev-parse", "pr-6~1"))
	second := strings.TrimSpace(j.work.Git("rev-parse", "pr-6"))
	base, head, change, store := j.prFacts(t, "6")

	for i, step := range [][2]string{{base, first}, {first, second}} {
		wt := j.worktreeAt(t, step[0])
		j.restore(t, wt, step[1])
		tokens := []string{j.seat(t, wt, "simp-c"+strconv.Itoa(i), "code-simplifier"), j.seat(t, wt, "rev-c"+strconv.Itoa(i), "code-reviewer")}
		if rc, logs := j.gate(t, wt, tokens...); rc != 0 {
			t.Fatalf("gate over commit %d rc=%d\n%s", i, rc, logs)
		}
		j.work.Git("worktree", "remove", "--force", wt)
	}

	want := j.refusal("6", "no review record for its whole change "+change[:7]+" ("+base[:7]+".."+head[:7]+"), looked in "+store+". Next: in a worktree at "+base[:7]+", run git restore --source="+head[:7]+" --staged --worktree -- . , run the review seats and evolve commit-gate run, then run record-pr again.")
	rc, _, errb := j.run("record-pr", "--pr", "6", "--reviewed")
	if rc != exitRefused || errb != want {
		t.Fatalf("rc=%d\n got %q\nwant %q", rc, errb, want)
	}

	m := nextRestore.FindStringSubmatch(errb)
	if m == nil {
		t.Fatalf("no next step in %q", errb)
	}
	wt := j.worktreeAt(t, m[1])
	args := strings.Fields(m[2])
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", m[2], err, out)
	}
	tokens := []string{j.seat(t, wt, "simp-whole", "code-simplifier"), j.seat(t, wt, "rev-whole", "code-reviewer")}
	if rc, logs := j.gate(t, wt, tokens...); rc != 0 {
		t.Fatalf("gate over the whole change rc=%d\n%s", rc, logs)
	}
	j.work.Git("worktree", "remove", "--force", wt)
	if rc, _, errb := j.run("record-pr", "--pr", "6", "--reviewed"); rc != 0 {
		t.Fatalf("record-pr after the next step rc=%d %s", rc, errb)
	}
}

var nextReseat = regexp.MustCompile(`Next: (run the review seats and evolve commit-gate run on this change), then run record-pr again\.`)

func TestJourneyRecordPRReviewedCommentOnlyWaiverIsRefusedAndItsNextStepWorks(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.work.Write("c.go", "package c\n\n// note\nvar x = 1\n")
	j.work.Git("add", "c.go")
	j.work.Git("commit", "-q", "-m", "add c")
	j.work.Git("push", "-q", "origin", "main")
	j.pr(t, 8, "refactor: drop the note", "c.go", "package c\n\nvar x = 1\n")
	_, _, change, _ := j.prFacts(t, "8")

	wt := j.worktreeAt(t, "main")
	j.restore(t, wt, "pr-8")
	if rc, logs := j.gate(t, wt); rc != 0 {
		t.Fatalf("waiver gate rc=%d\n%s", rc, logs)
	}
	want := j.refusal("8", `the record of change `+change[:7]+` holds the waiver "comment-only", and a waiver is not a review. Next: run the review seats and evolve commit-gate run on this change, then run record-pr again.`)
	rc, _, errb := j.run("record-pr", "--pr", "8", "--reviewed")
	if rc != exitRefused || errb != want {
		t.Fatalf("rc=%d\n got %q\nwant %q", rc, errb, want)
	}
	if !nextReseat.MatchString(errb) {
		t.Fatalf("no next step in %q", errb)
	}

	tokens := []string{j.seat(t, wt, "simp-8", "code-simplifier"), j.seat(t, wt, "rev-8", "code-reviewer")}
	if rc, logs := j.gate(t, wt, tokens...); rc != 0 {
		t.Fatalf("gate with the review runs rc=%d\n%s", rc, logs)
	}
	j.work.Git("worktree", "remove", "--force", wt)
	if rc, _, errb := j.run("record-pr", "--pr", "8", "--reviewed"); rc != 0 {
		t.Fatalf("record-pr after the next step rc=%d %s", rc, errb)
	}
}

func (j *trainJourney) putRecord(t *testing.T, store, change string, body []byte) string {
	t.Helper()
	path := filepath.Join(store, change+".json")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordJSON(t *testing.T, rec commitgate.ReviewRecord) []byte {
	t.Helper()
	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func launchFor(id, profile, tree string) launchrecord.Record {
	return launchrecord.Record{
		RunID: id, Profile: profile, CLI: "agy-tmux", Family: "agy", Model: "m", Verdict: "PASS",
		TreeBefore: tree, TreeAfter: tree, ArtifactSHA256: "a1", TS: "2026-11-05T09:00:00Z",
	}
}

func TestSignalRecordPRReviewedTextsAndExitCodes(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 5, "feat: a", "a.txt", "a\n")
	_, _, change, store := j.prFacts(t, "5")
	label := change[:7]
	again := "then run record-pr again."
	cases := map[string]struct {
		body func(path string) []byte
		want func(path string) string
	}{
		"does not parse": {
			func(string) []byte { return []byte("{") },
			func(path string) string {
				return "the record of change " + label + " at " + path + " does not parse: unexpected end of JSON input. Next: run the review seats and evolve commit-gate run again, " + again
			},
		},
		"names no review run": {
			func(string) []byte {
				return recordJSON(t, commitgate.ReviewRecord{Attestation: commitgate.Attestation{TreeStateSHA: change, ReviewersRun: []string{"code-reviewer"}}})
			},
			func(string) string {
				return "the record of change " + label + " names no review run. Next: run evolve commit-gate run with --reviewers run:<label>, " + again
			},
		},
		"names a run that is refused": {
			func(string) []byte {
				return recordJSON(t, commitgate.ReviewRecord{Attestation: commitgate.Attestation{TreeStateSHA: change, ReviewRuns: []commitgate.ReviewRun{{RunID: "ghost"}}}})
			},
			func(string) string {
				return "the record of change " + label + " names review run ghost: no launch record of the run is copied in the record. Next: fix the cause, run the review seats and evolve commit-gate run again, " + again
			},
		},
		"a record that covers another change": {
			func(string) []byte {
				return recordJSON(t, commitgate.ReviewRecord{
					Attestation: commitgate.Attestation{TreeStateSHA: "other", ReviewRuns: []commitgate.ReviewRun{{RunID: "s1"}, {RunID: "r1"}}},
					LaunchRecords: []launchrecord.Record{
						launchFor("s1", "code-simplifier", "other"),
						launchFor("r1", "code-reviewer", "other"),
					},
				})
			},
			func(string) string {
				return `integration: the attestation covers the change "other", not the change "` + change + `" of this PR`
			},
		},
		"one simplifier run and a plain reviewer name": {
			func(string) []byte {
				return recordJSON(t, commitgate.ReviewRecord{
					Attestation:   commitgate.Attestation{TreeStateSHA: change, ReviewersRun: []string{"code-simplifier", "go-reviewer"}, ReviewRuns: []commitgate.ReviewRun{{RunID: "s1"}}},
					LaunchRecords: []launchrecord.Record{launchFor("s1", "code-simplifier", change)},
				})
			},
			func(string) string {
				return "the review runs of change " + label + " prove no review (profiles: code-simplifier). Next: run a review seat with that capability and evolve commit-gate run with --reviewers run:<label>, " + again
			},
		},
	}
	for name, c := range cases {
		path := j.putRecord(t, store, change, c.body(""))
		rc, out, errb := j.run("record-pr", "--pr", "5", "--reviewed")
		if want := j.refusal("5", c.want(path)); rc != exitRefused || out != "" || errb != want {
			t.Errorf("%s: rc=%d out=%q\n got %q\nwant %q", name, rc, out, errb, want)
		}
	}
}

func TestSignalRecordPRReviewedReportsAnUnreadableRecordAsAFailureWithItsCause(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 5, "feat: a", "a.txt", "a\n")
	_, _, change, store := j.prFacts(t, "5")
	if err := os.MkdirAll(filepath.Join(store, change+".json"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := j.run("record-pr", "--pr", "5", "--reviewed")
	prefix := j.refusal("5", "read the record of change "+change[:7]+" at "+filepath.Join(store, change+".json")+": ")
	if rc != exitRefused || !strings.HasPrefix(errb, strings.TrimSuffix(prefix, "\n")) {
		t.Errorf("rc=%d err=%q, want a loud refusal that starts %q", rc, errb, prefix)
	}
}

func TestSignalRecordPRReviewedRefusesAPRThatTheTrainCannotCompose(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.work.Git("checkout", "-q", "-b", "side", "main")
	j.work.Write("side.txt", "side\n")
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", "side")
	side := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Git("checkout", "-q", "-b", "pr-12", "main")
	j.work.Write("own.txt", "own\n")
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", "own")
	own := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Git("merge", "-q", "--no-ff", "-m", "merge", "side")
	merge := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Git("push", "-q", "origin", "pr-12:refs/pull/12/head")

	j.work.Git("checkout", "-q", "-b", "pr-13", "main")
	j.work.Git("commit", "-q", "--allow-empty", "-m", "nothing")
	empty := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Git("push", "-q", "origin", "pr-13:refs/pull/13/head")

	j.work.Git("checkout", "-q", "-b", "pr-14", "main")
	j.work.Write("temp.txt", "t\n")
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", "add")
	j.work.Git("rm", "-q", "temp.txt")
	j.work.Git("commit", "-q", "-m", "remove")
	j.work.Git("push", "-q", "origin", "pr-14:refs/pull/14/head")
	j.work.Git("checkout", "-q", "main")

	cases := []struct{ pr, reason string }{
		{"12", "commit " + merge[:7] + " is a merge of " + own[:7] + " and " + side[:7] + ", and the train composes a linear PR. Next: put the commits on a new branch from origin/main, open a PR, and run record-pr on it."},
		{"13", "commit " + empty[:7] + " changes nothing. Next: put the other commits on a new branch from origin/main without it."},
		{"14", "its commits have no net change. Next: close the PR."},
	}
	for _, c := range cases {
		rc, out, errb := j.run("record-pr", "--pr", c.pr, "--reviewed")
		if want := j.refusal(c.pr, c.reason); rc != exitRefused || out != "" || errb != want {
			t.Errorf("PR %s: rc=%d out=%q\n got %q\nwant %q", c.pr, rc, out, errb, want)
		}
	}
}

func TestRecordPRReviewedUsageAndStoreFailure(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	for _, args := range [][]string{
		{"record-pr", "--pr", "1"},
		{"record-pr", "--pr", "1", "--reviewed", "--class", "docs-review"},
		{"record-pr", "--pr", "1", "--attestation", "p"},
	} {
		if rc, _, errb := j.run(args...); rc != exitUsage || !strings.Contains(errb, "--reviewed") {
			t.Errorf("%v: rc=%d err=%q, want usage naming --reviewed", args, rc, errb)
		}
	}
	j.env.Root = t.TempDir()
	rc, _, errb := j.run("record-pr", "--pr", "1", "--reviewed")
	if rc != exitIO || !strings.HasPrefix(errb, "evolve integration: cannot resolve the git common directory of ") {
		t.Errorf("a root with no repository: rc=%d err=%q, want %d and the store failure", rc, errb, exitIO)
	}
}
