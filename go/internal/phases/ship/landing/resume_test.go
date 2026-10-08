package landing

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const (
	testCommitTree = "77777777777777777777777777777777abcdef01"
	testAudit      = "a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0"
	testAudited    = "5555555555555555555555555555555555555555"
	testView       = "e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1"
)

func preparedIntent() Intent {
	return Intent{Cycle: 1830, RunID: "run-1830", AuditArtifactSHA256: testAudit, AuditedTree: testAudited, LaneTree: testAudited,
		WorktreeBaseSHA: testOriginRef, CommitSHA: testHead, CommitTree: testCommitTree,
		ConsumedPaths:         []string{".evolve/inbox/item.json", ".evolve/inbox/consumed/item.json"},
		ExplanationViewSHA256: testView, PreMain: testOriginRef, Branch: testBranch, LaneBranch: testCycleBr, Status: IntentPrepared}
}

func witnessOf(in Intent) Witness {
	return Witness{Cycle: in.Cycle, RunID: in.RunID, Journaled: true, AuditPassed: true, AuditArtifactSHA256: in.AuditArtifactSHA256,
		AuditedTree: in.AuditedTree, LaneTip: in.CommitSHA, ExplanationViewSHA256: in.ExplanationViewSHA256}
}

func resumableGit() *fakeGit {
	f := newFakeGit()
	f.on("rev-parse "+testHead+"^{tree}", scripted{stdout: testCommitTree + "\n"})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("merge-base --is-ancestor "+testHead+" "+testOriginRef, scripted{exit: 1})
	return f
}

func resumeFetchFailureFixture(t *testing.T, l *Landing, f *fakeGit) {
	t.Helper()
	f.set("rev-parse "+testHead+"^{tree}", scripted{stdout: testCommitTree + "\n"})
	f.set("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.set("fetch origin "+testBranch, scripted{exit: 128})
	if v := l.Resume(context.Background(), preparedIntent(), witnessOf(preparedIntent())); v != (Verdict{}) {
		t.Fatalf("a failed fetch decides on the last known origin ref: %+v", v)
	}
	f.set("fetch origin "+testBranch, scripted{})
}

func TestResume_AMatchingIntentWhoseOriginIsAnAncestorResumes(t *testing.T) {
	f := resumableGit()
	l, got := newLanding(f)

	v := l.Resume(context.Background(), preparedIntent(), witnessOf(preparedIntent()))

	if v != (Verdict{}) || len(*got) != 0 {
		t.Fatalf("Resume = %+v, events %+v; want a silent resume at the push", v, *got)
	}
	want := []string{"rev-parse " + testHead + "^{tree} [capture]", "fetch origin main [discard]", "rev-parse origin/main [capture]",
		"merge-base --is-ancestor " + testHead + " " + testOriginRef + " [discard]",
		"merge-base --is-ancestor " + testOriginRef + " " + testHead + " [discard]", "merge-base --is-ancestor main " + testHead + " [discard]"}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("argv\n got %q\nwant %q", f.calls, want)
	}
}

func TestResume_DeclinesWhatItCannotProve(t *testing.T) {
	for name, tc := range map[string]struct {
		intent  func(*Intent)
		witness func(*Witness)
		git     func(*fakeGit)
		want    string
	}{
		"a superseding audit": {witness: func(w *Witness) { w.AuditArtifactSHA256 = strings.Repeat("b", 64) },
			want: "a newer audit (" + strings.Repeat("b", 64) + " of tree " + testAudited + ") supersedes the landing's audit"},
		"a re-audit of another tree": {witness: func(w *Witness) { w.AuditedTree = testCommitTree }, want: "supersedes the landing's audit"},
		"a moved lane tip":           {witness: func(w *Witness) { w.LaneTip = testOriginRef }, want: "the lane tip moved to " + testOriginRef + " off the prepared commit " + testHead},
		"another run":                {witness: func(w *Witness) { w.RunID = "run-1831" }, want: `the intent is cycle 1830 run "run-1830", not cycle 1830 run "run-1831"`},
		"another cycle":              {witness: func(w *Witness) { w.Cycle = 1831 }, want: "not cycle 1831"},
		"a re-sealed explanation":    {witness: func(w *Witness) { w.ExplanationViewSHA256 = "" }, want: "the sealed Build explanation changed"},
		"an intent that binds no audit": {intent: func(in *Intent) { in.AuditedTree = "" }, witness: func(w *Witness) { w.AuditedTree = "" },
			want: "the intent binds no audit"},
		"an intent that binds no audit artifact": {intent: func(in *Intent) { in.AuditArtifactSHA256 = "" }, witness: func(w *Witness) { w.AuditArtifactSHA256 = "" },
			want: "the intent binds no audit"},
		"an intent that names no lane tree": {intent: func(in *Intent) { in.LaneTree = "" }, want: "the intent binds no audit"},
		"a commit the journal does not hold": {witness: func(w *Witness) { w.Journaled = false },
			want: "the ship journal does not hold the prepared commit " + testHead},
		"a newest audit that is not a PASS": {witness: func(w *Witness) { w.AuditPassed = false }, want: "the newest audit of the run is not a PASS"},
		"a commit without its tree": {git: func(f *fakeGit) { f.set("rev-parse "+testHead+"^{tree}", scripted{stdout: testAudited + "\n"}) },
			want: "does not hold the tree " + testCommitTree},
		"an unreadable commit": {git: func(f *fakeGit) { f.set("rev-parse "+testHead+"^{tree}", scripted{exit: 128}) }, want: "does not hold the tree"},
		"an unreadable origin": {git: func(f *fakeGit) { f.set("rev-parse origin/"+testBranch, scripted{exit: 128}) }, want: "origin/main is unreadable"},
		"a diverged origin": {git: func(f *fakeGit) {
			f.on("merge-base --is-ancestor "+testOriginRef+" "+testHead, scripted{exit: 1})
		}, want: "origin/main (" + testOriginRef + ") diverged from the prepared commit " + testHead},
		"a main that moved off the commit": {git: func(f *fakeGit) {
			f.on("merge-base --is-ancestor main "+testHead, scripted{err: errors.New("no git")})
		}, want: "main is not an ancestor of the prepared commit " + testHead},
	} {
		t.Run(name, func(t *testing.T) {
			f := resumableGit()
			if tc.git != nil {
				tc.git(f)
			}
			in := preparedIntent()
			if tc.intent != nil {
				tc.intent(&in)
			}
			w := witnessOf(in)
			if tc.witness != nil {
				tc.witness(&w)
			}
			l, _ := newLanding(f)

			v := l.Resume(context.Background(), in, w)

			if v.OnOrigin || !strings.Contains(v.Declined, tc.want) {
				t.Errorf("Resume = %+v, want a decline naming %q", v, tc.want)
			}
		})
	}
}

func TestResume_AWitnessMismatchDeclinesBeforeAnyGitCall(t *testing.T) {
	f := resumableGit()
	l, _ := newLanding(f)
	w := witnessOf(preparedIntent())
	w.LaneTip = testOriginRef

	l.Resume(context.Background(), preparedIntent(), w)

	if len(f.calls) != 0 {
		t.Errorf("argv %q: a stale intent never touches git or origin", f.calls)
	}
}

func TestResume_AFailedFetchWarnsAndDecidesOnTheLastKnownOrigin(t *testing.T) {
	f := resumableGit()
	f.on("fetch origin "+testBranch, scripted{err: errors.New("network down")})
	l, got := newLanding(f)

	v := l.Resume(context.Background(), preparedIntent(), witnessOf(preparedIntent()))

	if v != (Verdict{}) {
		t.Fatalf("Resume = %+v: an unreachable remote is the push's to report", v)
	}
	e := wantOneEvent(t, *got, CodeOriginFetchFailed, "Landing.Resume", map[string]string{"step": "resume", "branch": testBranch, "git_rc": "0", "git_err": "network down"})
	if !strings.Contains(e.Reason, "decides on the last known origin ref") {
		t.Errorf("reason %q", e.Reason)
	}
}

func TestAdmit_ACommitOriginAlreadyHoldsSettlesWithNoPush(t *testing.T) {
	f := resumableGit()
	f.set("merge-base --is-ancestor "+testHead+" "+testOriginRef, scripted{})
	l, _ := newLanding(f)

	v := l.Admit(context.Background(), preparedIntent())

	if v != (Verdict{OnOrigin: true}) {
		t.Fatalf("Admit = %+v: origin holds the prepared commit under a peer, so the landing settles with no push and no unwind", v)
	}
	if last := f.calls[len(f.calls)-1]; last != "merge-base --is-ancestor "+testHead+" "+testOriginRef+" [discard]" {
		t.Errorf("argv %q: the first ancestry answer that origin holds the commit ends the checks", f.calls)
	}
}

func TestAdmit_ChecksOnlyTheAncestryForTheRepairRung(t *testing.T) {
	f := resumableGit()
	l, _ := newLanding(f)
	in := preparedIntent()
	in.AuditArtifactSHA256, in.LaneTree, in.ExplanationViewSHA256 = "", "", ""

	if v := l.Admit(context.Background(), in); v != (Verdict{}) {
		t.Errorf("Admit = %+v: the repair rung has no witness, so only the tree and the ancestry decide", v)
	}
}
