package mainsync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainsync"
)

type fixture struct {
	plane, peer, origin *gittest.Repo
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{plane: gittest.Fixture(t), origin: gittest.Bare(t)}
	f.plane.Write("base.txt", "base\n")
	f.plane.Git("add", "-A")
	f.plane.Git("commit", "-q", "-m", "base")
	f.plane.Git("remote", "add", "origin", f.origin.Dir)
	f.plane.Git("push", "-q", "origin", "main")
	f.peer = gittest.Clone(t, f.origin.Dir)
	return f
}

func (f fixture) peerCommits(rel, body string) string {
	f.peer.Write(rel, body)
	f.peer.Git("add", "-A")
	f.peer.Git("commit", "-q", "-m", "origin moves "+rel)
	f.peer.Git("push", "-q", "origin", "main")
	return f.peer.Git("rev-parse", "HEAD")
}

func (f fixture) run(p mainsync.Policy) mainsync.Result {
	return mainsync.Run(context.Background(), gitexec.Default(f.plane.Dir), p)
}

func TestRun_ABehindMainIsFastForwardedAndSettledAsAdvanced(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tip := f.peerCommits("peer.txt", "from origin\n")
	var settled []mainsync.Outcome

	res := f.run(mainsync.Policy{Around: func(context.Context, gitexec.Git) func(mainsync.Result) {
		return func(r mainsync.Result) { settled = append(settled, r.Outcome) }
	}})

	if res.Outcome != mainsync.OutcomeFastForwarded || f.plane.Git("rev-parse", "HEAD") != tip {
		t.Fatalf("outcome=%s head=%s want %s", res.Outcome, f.plane.Git("rev-parse", "HEAD"), tip)
	}
	if len(settled) != 1 || settled[0] != mainsync.OutcomeFastForwarded {
		t.Fatalf("settled=%v", settled)
	}
}

func TestRun_AFetchConfigThatOmitsMainStillRefreshesOriginMain(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.plane.Git("config", "--unset-all", "remote.origin.fetch")
	tip := f.peerCommits("peer.txt", "from origin\n")

	res := f.run(mainsync.Policy{})

	if res.Outcome != mainsync.OutcomeFastForwarded || res.Relation.Remote != tip {
		t.Fatalf("outcome=%s remote=%s want %s", res.Outcome, res.Relation.Remote, tip)
	}
}

func TestRun_CurrentAheadAndDivergedMoveNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if res := f.run(mainsync.Policy{}); res.Outcome != mainsync.OutcomeCurrent {
		t.Fatalf("outcome=%s", res.Outcome)
	}
	f.plane.Write("local.txt", "local\n")
	f.plane.Git("add", "-A")
	f.plane.Git("commit", "-q", "-m", "local")
	before := f.plane.Git("rev-parse", "HEAD")
	if res := f.run(mainsync.Policy{}); res.Outcome != mainsync.OutcomeAhead {
		t.Fatalf("outcome=%s", res.Outcome)
	}
	f.peerCommits("peer.txt", "from origin\n")
	if res := f.run(mainsync.Policy{}); res.Outcome != mainsync.OutcomeDiverged || f.plane.Git("rev-parse", "HEAD") != before {
		t.Fatalf("outcome=%s head moved", res.Outcome)
	}
}

func TestRun_AFailedFetchIsReportedAndNothingIsTrusted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.plane.Git("remote", "set-url", "origin", f.origin.Dir+"-missing")
	res := f.run(mainsync.Policy{})
	if res.Outcome != mainsync.OutcomeFetchFailed || res.Code == 0 || res.Stderr == "" {
		t.Fatalf("%+v", res)
	}
}

func TestRun_AnUnreadableRelationIsReported(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.plane.Git("update-ref", "-d", "refs/heads/main")
	res := f.run(mainsync.Policy{})
	if res.Outcome != mainsync.OutcomeRelationFailed || res.Err == nil {
		t.Fatalf("%+v", res)
	}
}

func TestRun_ATrackedChangeBlocksTheFastForwardAndIsSettledAsBlocked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.peerCommits("base.txt", "origin edit\n")
	f.plane.Write("base.txt", "local dirt\n")
	var settled mainsync.Result

	res := f.run(mainsync.Policy{Around: func(context.Context, gitexec.Git) func(mainsync.Result) {
		return func(r mainsync.Result) { settled = r }
	}})

	if res.Outcome != mainsync.OutcomeBlocked || res.Code == 0 || res.Stderr == "" || settled.Outcome != mainsync.OutcomeBlocked {
		t.Fatalf("res=%+v settled=%+v", res, settled)
	}
}

func TestRun_AGateErrorStopsTheSyncBeforeAnyFastForwardAndSkipsADivergedMain(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	before := f.plane.Git("rev-parse", "HEAD")
	f.peerCommits("peer.txt", "from origin\n")
	boom := errors.New("red")
	calls := 0
	gate := func(context.Context, gitexec.MainRelation) error { calls++; return boom }

	res := f.run(mainsync.Policy{Gate: gate})

	if res.Outcome != mainsync.OutcomeGated || !errors.Is(res.Err, boom) || f.plane.Git("rev-parse", "HEAD") != before {
		t.Fatalf("%+v", res)
	}
	f.plane.Write("local.txt", "local\n")
	f.plane.Git("add", "-A")
	f.plane.Git("commit", "-q", "-m", "local")
	if res := f.run(mainsync.Policy{Gate: gate}); res.Outcome != mainsync.OutcomeDiverged || calls != 1 {
		t.Fatalf("outcome=%s gate calls=%d: a diverged main is not gated", res.Outcome, calls)
	}
}
