//go:build acs

package cycle1845

import (
	"regexp"
	"testing"
	"time"
)

const (
	exitGreen      = 0
	exitRed        = 1
	exitUnseen     = 2
	neverDone      = 1 << 20
	oneSecondCap   = 1
	hungWatchGuard = 40 * time.Second
)

func TestC1845_001_WatchThatOutlivesDeadlineExitsTwoNamingLastStatus(t *testing.T) {
	fx := newCIFixture(t, oneSecondCap, fakeGHState{Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 2001, Conclusion: "success", Pending: neverDone}}})
	res := fx.watch(t, "--sha", fx.head)
	if res.code != exitUnseen {
		t.Fatalf("in-progress run past the deadline must exit %d: %s", exitUnseen, res)
	}
	if !regexp.MustCompile(`last status="in_progress"`).MatchString(res.stderr) {
		t.Errorf("stderr lacks the last observed status: %s", res)
	}
	if regexp.MustCompile(`\bsuccess\b`).MatchString(res.stdout) {
		t.Errorf("stdout reports a green conclusion for a run that never completed: %s", res)
	}
	if items := fx.inboxItems(t); len(items) != 0 {
		t.Errorf("an unfinished run must file no inbox item: %v", items)
	}
}

func TestC1845_002_RunThatNeverAppearsExitsTwoNamingQueued(t *testing.T) {
	fx := newCIFixture(t, oneSecondCap, fakeGHState{})
	res := fx.watch(t, "--sha", fx.head)
	if res.code != exitUnseen {
		t.Fatalf("no visible run past the deadline must exit %d: %s", exitUnseen, res)
	}
	if !regexp.MustCompile(`last status="queued"`).MatchString(res.stderr) {
		t.Errorf("stderr lacks the last observed status: %s", res)
	}
}

func TestC1845_003_TagWatchWithGreenRequiredAndHungReleaseExitsTwo(t *testing.T) {
	fx := newCIFixture(t, oneSecondCap, fakeGHState{Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 2003, Conclusion: "success"},
		{Workflow: releaseWorkflow, ID: 2004, Conclusion: "success", Pending: neverDone},
	}})
	res := fx.watch(t, "--tag", fixtureTag)
	if res.code != exitUnseen {
		t.Fatalf("one hung workflow must make the whole watch exit %d: %s", exitUnseen, res)
	}
	if !regexp.MustCompile(`release\.yml.*last status="in_progress"`).MatchString(res.stderr) {
		t.Errorf("stderr must name the hung workflow and its last status: %s", res)
	}
}

func TestC1845_004_CompletedGreenExitsZeroAndRedExitsOne(t *testing.T) {
	green := newCIFixture(t, 30, fakeGHState{Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 2005, Conclusion: "success"}}})
	if res := green.watch(t, "--sha", green.head); res.code != exitGreen {
		t.Errorf("completed green run must exit %d: %s", exitGreen, res)
	}
	red := newCIFixture(t, 30, fakeGHState{Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 2006, Conclusion: "failure"}}})
	if res := red.watch(t, "--sha", red.head); res.code != exitRed {
		t.Errorf("completed red run must exit %d: %s", exitRed, res)
	}
}

func TestC1845_005_RedWorkflowIsNotMaskedByLaterHungWorkflow(t *testing.T) {
	fx := newCIFixture(t, oneSecondCap, fakeGHState{Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 2007, Conclusion: "failure"},
		{Workflow: releaseWorkflow, ID: 2008, Conclusion: "success", Pending: neverDone},
	}})
	res := fx.watch(t, "--tag", fixtureTag)
	if res.code == exitGreen {
		t.Fatalf("red plus hung must never exit 0: %s", res)
	}
	if !regexp.MustCompile(`required\.yml`).MatchString(res.combined()) {
		t.Errorf("the red workflow vanished from the output: %s", res)
	}
}

func TestC1845_006_HungGHCallStillEndsTheWatchAtItsDeadlineWithExitTwo(t *testing.T) {
	fx := newCIFixture(t, oneSecondCap*2, fakeGHState{Hang: true, Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 2009, Conclusion: "success"}}})
	res := fx.watchWithin(t, hungWatchGuard, "--sha", fx.head)
	if res.code != exitUnseen {
		t.Fatalf("a gh call that never returns must end the watch at its deadline with exit %d: %s", exitUnseen, res)
	}
	if !regexp.MustCompile(`(?i)tim(ed|e)\s?out|deadline`).MatchString(res.stderr) {
		t.Errorf("stderr does not say the watch ran out of time: %s", res)
	}
}
