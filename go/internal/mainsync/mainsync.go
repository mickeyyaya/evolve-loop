package mainsync

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

type Outcome string

const (
	OutcomeFetchFailed    Outcome = "fetch-failed"
	OutcomeRelationFailed Outcome = "relation-failed"
	OutcomeGated          Outcome = "gated"
	OutcomeCurrent        Outcome = "current"
	OutcomeAhead          Outcome = "ahead"
	OutcomeDiverged       Outcome = "diverged"
	OutcomeBlocked        Outcome = "blocked"
	OutcomeFastForwarded  Outcome = "fast-forwarded"

	fetchRefspec = "+refs/heads/" + gitexec.MainBranch + ":refs/remotes/" + gitexec.RemoteMain
)

type Result struct {
	Outcome   Outcome
	Relation  gitexec.MainRelation
	Err       error
	Code      int
	Stderr    string
	Absorbed  []inboxmover.Absorbed
	AbsorbErr error
}

type Policy struct {
	Gate   func(ctx context.Context, rel gitexec.MainRelation) error
	Around func(ctx context.Context, g gitexec.Git) (settle func(Result))
}

func Run(ctx context.Context, g gitexec.Git, p Policy) Result {
	if _, stderr, code, err := g.Capture(ctx, "fetch", "-q", gitexec.Remote, fetchRefspec); err != nil || code != 0 {
		return Result{Outcome: OutcomeFetchFailed, Err: err, Code: code, Stderr: strings.TrimSpace(stderr)}
	}
	rel, err := g.RelationToRemote(ctx, gitexec.RemoteMain)
	if err != nil {
		return Result{Outcome: OutcomeRelationFailed, Err: err}
	}
	if rel.Kind != gitexec.RelationDiverged && p.Gate != nil {
		if err := p.Gate(ctx, rel); err != nil {
			return Result{Outcome: OutcomeGated, Relation: rel, Err: err}
		}
	}
	if rel.Kind != gitexec.RelationBehind {
		return Result{Outcome: Outcome(rel.Kind), Relation: rel}
	}
	return fastForward(ctx, g, p, rel)
}

func fastForward(ctx context.Context, g gitexec.Git, p Policy, rel gitexec.MainRelation) Result {
	settle := func(Result) {}
	if p.Around != nil {
		settle = p.Around(ctx, g)
	}
	_, stderr, code, err := g.Capture(ctx, "merge", "--ff-only", gitexec.RemoteMain)
	if err != nil || code != 0 {
		res := Result{Outcome: OutcomeBlocked, Relation: rel, Err: err, Code: code, Stderr: strings.TrimSpace(stderr)}
		settle(res)
		return res
	}
	res := Result{Outcome: OutcomeFastForwarded, Relation: rel}
	settle(res)
	res.Absorbed, res.AbsorbErr = inboxmover.AbsorbRootCopies(inboxmover.Options{ProjectRoot: g.Dir})
	return res
}
