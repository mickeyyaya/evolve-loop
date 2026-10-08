package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

func scriptedLanding(t *testing.T, fleet bool) (*Options, *argvRecorder, *worktreeShip, *RunResult) {
	t.Helper()
	opts, _, _ := pinOptions(t, ClassCycle)
	if fleet {
		opts.Env["EVOLVE_FLEET"] = "1"
	}
	opts.RunID, opts.internalAuditBoundTreeSHA, opts.internalAuditArtifactSHA = "run-7", pinTree, "audit-sha"
	opts.internalConsumedPaths = []string{".evolve/inbox/item.json", ".evolve/inbox/consumed/item.json"}
	r := newArgvRecorder(opts)
	wt := filepath.Join(opts.ProjectRoot, "wt")
	scriptGreenLanding(r, wt)
	res := &RunResult{}
	s := newWorktreeShip(context.Background(), opts, res, pinBranch, wt)
	s.cycleBranch = pinCycleBranch
	return opts, r, s, res
}

func landingIntentOf(t *testing.T, opts *Options) landing.Intent {
	t.Helper()
	in, found, err := landing.ReadIntent(intentPath(opts, 7))
	if err != nil || !found {
		t.Fatalf("no landing intent in .evolve/landing: found=%v err=%v", found, err)
	}
	return in
}

func TestWorktreeShipLand_PushesTheLaneCommitBeforeMainMoves(t *testing.T) {
	opts, r, s, res := scriptedLanding(t, false)

	if err := s.land(""); err != nil {
		t.Fatalf("land: %v\n%s", err, strings.Join(res.Logs, "\n"))
	}

	want := []string{
		"-C <root>/wt rev-parse HEAD [capture]",
		"rev-parse origin/main [capture]",
		"merge-base --is-ancestor main " + pinHead + " [discard]",
		"rev-parse " + pinHead + "^{tree} [capture]",
		"rev-parse main [capture]",
		"read-tree " + pinHead + " [other]",
		"ls-tree " + pinHead + "^ -- .evolve/inbox/item.json [other]",
		"update-index --add --cacheinfo 100644," + pinItemBlob + ",.evolve/inbox/item.json [other]",
		"ls-tree " + pinHead + "^ -- .evolve/inbox/consumed/item.json [other]",
		"update-index --force-remove -- .evolve/inbox/consumed/item.json [other]",
		"write-tree [other]",
		"push origin " + pinHead + ":refs/heads/main [streams]",
		"checkout HEAD -- go/evolve [discard]",
		"merge --ff-only " + pinHead + " [streams]",
		"rev-parse " + pinHead + "^{tree} [capture]",
	}
	if strings.Join(r.argvs(), "\n") != strings.Join(want, "\n") {
		t.Errorf("argv\n got %q\nwant %q: the lane commit is pushed by refspec before the plane main moves", r.argvs(), want)
	}
	in := landingIntentOf(t, opts)
	if in.Status != landing.IntentComplete || in.CommitSHA != pinHead || in.PreMain != pinOriginRef || in.AuditArtifactSHA256 != "audit-sha" ||
		len(in.ConsumedPaths) != 2 || in.LaneTree != pinLaneTree || in.AuditedTree != pinTree || in.LaneBranch != pinCycleBranch {
		t.Errorf("intent %+v, want the completed landing of %s prepared on main %s", in, pinHead, pinOriginRef)
	}
	if !journalHasSHA(opts.ProjectRoot, pinHead) || res.CommitSHA != pinHead {
		t.Errorf("journal has %s: %v, result %s", pinHead, journalHasSHA(opts.ProjectRoot, pinHead), res.CommitSHA)
	}
}

func TestWorktreeShipLand_ARejectedPushNeverMovesMainAndKeepsTheIntentPrepared(t *testing.T) {
	opts, r, s, res := scriptedLanding(t, false)
	r.on("push origin "+pinHead+":refs/heads/main", scriptedCall{exit: 1})
	r.on("fetch origin "+pinBranch, scriptedCall{exit: 1})

	err := s.land("")

	wantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "ship: git push of "+pinHead+" to origin/main failed (rc=1); ship did not move main")
	for _, a := range r.argvs() {
		if strings.HasPrefix(a, "merge --ff-only") || strings.HasPrefix(a, "checkout HEAD --") {
			t.Errorf("%s ran after a rejected push: a failed push never moves the plane main", a)
		}
	}
	if in := landingIntentOf(t, opts); in.Status != landing.IntentPrepared || !journalHasSHA(opts.ProjectRoot, pinHead) {
		t.Errorf("intent %+v, journaled=%v: the prepared landing survives the failed push", in, journalHasSHA(opts.ProjectRoot, pinHead))
	}
	if !containsLog(*res, "a retry resumes at the push") {
		t.Errorf("logs %q", res.Logs)
	}
}

func TestWorktreeShipLand_AMainThatMovedPastTheLaneNeverPushesNorPrepares(t *testing.T) {
	for fleet, code := range map[bool]core.ShipErrorCode{true: core.CodeGitFleetRebaseNeeded, false: core.CodeGitFFMergeDiverged} {
		opts, r, s, _ := scriptedLanding(t, fleet)
		r.on("merge-base --is-ancestor main "+pinHead, scriptedCall{exit: 1})

		err := s.land("")

		if se := mustShipErr(t, err); se.Code != code {
			t.Errorf("fleet=%v: code %s, want %s", fleet, se.Code, code)
		}
		for _, a := range r.argvs() {
			if strings.HasPrefix(a, "push ") {
				t.Errorf("fleet=%v: %s ran although main is not an ancestor of the lane commit", fleet, a)
			}
		}
		if _, found, _ := landing.ReadIntent(intentPath(opts, 7)); found || journalHasSHA(opts.ProjectRoot, pinHead) {
			t.Errorf("fleet=%v: a divergence leaves no intent and no journal entry for core's rebase to trip on", fleet)
		}
	}
}
