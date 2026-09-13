package ship

// gitops_landing.go — the ADR-0103 unit-07 seam between the ship phase and
// the landing leaf (internal/phases/ship/landing): the landing's ONE wired
// construction, the push step's projection of the host-owned once-guard, and
// the Strangler Fig facades (writeShipBinding, isAncestor, captureGitOutput)
// the ship paths, the resume repair and the by-name tests keep.

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

// landing returns the unit-07 leaf, lazily built and cached on the Options
// value — Run takes Options by value and hands the same &opts to every
// stage, so the cache never leaks back to the caller; direct-helper tests
// build an Options literal and call the facades.
func (o *Options) landing() *landing.Landing {
	if o.land == nil {
		o.land = o.wiredLanding()
	}
	return o.land
}

// wiredLanding is the ONE construction of the landing
// (TestLanding_OneConstructionSite): git through Options.run curried with
// "git" (the runner seam the recorded-runner tests inject, cwd ProjectRoot),
// the operator streams and the Center through accessors read at every use
// (Run defaults the streams after the options are built; the root sets
// Signals), the phase name (phaseName — core.PhaseShip's spelling; the leaf
// spells none) and the run identity from the request.
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

// pushWithRepair is the ONE host projection of the push step (the three
// hand-copied push blocks before the move): the once-per-Run repair ledger
// goes in, and what ran comes back out — unconditionally, even on error,
// because the repair records itself before its probes. CommitSHA is set only
// when the push landed.
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

// writeShipBinding emits <run workspace>/ship-binding.json for post-ship
// audit — the facade the worktree integrate, the resume repair and the
// by-name tests keep. The run-scope resolution (cycleIDForShip) stays here;
// the layout is core.RunWorkspacePath + dossier.ShipBindingFile, the SSOTs
// the idempotency reader shares. Best-effort; failure is a WARN, not a ship
// failure.
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

// isAncestor reports whether anc is an ancestor of desc (git merge-base) —
// the facade the landing-note probes and the resume repair keep.
func isAncestor(ctx context.Context, opts *Options, anc, desc string) bool {
	return opts.landing().IsAncestor(ctx, anc, desc)
}

// captureGitOutput runs git <args...> and returns stdout, tolerating rc=1
// (git diff's "differences exist") — the facade its ten read sites keep.
func captureGitOutput(ctx context.Context, opts *Options, args ...string) (string, error) {
	return opts.landing().Capture(ctx, args...)
}
