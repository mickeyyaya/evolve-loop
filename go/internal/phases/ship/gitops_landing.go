package ship

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// landing lazily builds and caches the leaf on *Options; Run threads the
// same pointer to every stage, so the cache never leaks back to the caller.
func (o *Options) landing() *landing.Landing {
	if o.land == nil {
		o.land = o.wiredLanding()
	}
	return o.land
}

// Streams and the Center are read through accessors at every use, not
// captured eagerly: Run defaults the streams after Options is built, and
// the root sets Signals separately.
func (o *Options) wiredLanding() *landing.Landing {
	return landing.New(
		func(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
			return o.run(ctx, "git", args, stdout, stderr)
		},
		func() landing.Streams { return landing.Streams{Stdout: o.Stdout, Stderr: o.Stderr} },
		landing.WithRun(phaseName, o.CycleID, o.RunID),
		landing.WithSignals(func() *signalcenter.Center { return o.Signals }))
}

// logTo is the res.Logs sink the landing writes its lines through.
func logTo(res *RunResult) func(string) {
	return func(line string) { res.Logs = append(res.Logs, line) }
}

func pushWithRepair(ctx context.Context, opts *Options, res *RunResult, branch string, site landing.PushSite) error {
	out, err := opts.landing().Push(ctx, landing.PushRequest{
		Branch: branch, Site: site, DryRun: opts.DryRun,
		RepairAttempted: opts.repairAttempted[core.CodeGitPushRejected], Log: logTo(res),
	})
	if out.RepairAttempted {
		ensureRepairMap(opts)
		opts.repairAttempted[core.CodeGitPushRejected] = true
		res.RepairAttempted = string(core.CodeGitPushRejected)
	}
	if out.RepairOutcome != "" {
		res.RepairOutcome = string(out.RepairOutcome)
	}
	if err == nil {
		res.CommitSHA = out.Head
	}
	return err
}

func writeShipBinding(opts *Options, committedTree, commitSHA string) error {
	cid, ok, err := cycleIDForShip(opts)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("no cycle_id in cycle-state.json")
	}
	return opts.landing().WriteBinding(core.RunWorkspacePath(opts.ProjectRoot, cid), dossier.ShipBinding{
		AuditBoundTreeSHA: opts.internalAuditBoundTreeSHA,
		TreeSHACommitted:  committedTree,
		CommitSHA:         strings.TrimSpace(commitSHA),
		Cycle:             cid,
	})
}

func isAncestor(ctx context.Context, opts *Options, anc, desc string) bool {
	return opts.landing().IsAncestor(ctx, anc, desc)
}

func captureGitOutput(ctx context.Context, opts *Options, args ...string) (string, error) {
	return opts.landing().Capture(ctx, args...)
}
