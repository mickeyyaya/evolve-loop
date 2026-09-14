package loopchain

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// QuotaPause is the checkpoint block's projection the chain reads when a
// batch hit the quota wall (the host's detectQuotaPause).
type QuotaPause struct {
	Cycle  int
	WakeAt string
	Source string
}

// DriverDeps are the Driver's collaborators, explicit at construction: the
// batch (the host's runLoopBatch over the same config every time), the
// boundary refresh, the audit trail's last entry, the fleet width read purely
// to record it, and the quota-pause reader. A nil dep panics at first use.
type DriverDeps struct {
	Batch       func() int
	Refresh     func(batch int) bool
	LastRefresh func() (*RefreshLogEntry, error)
	FleetWidth  func() int
	QuotaPause  func() (QuotaPause, bool)
}

// Result is what a chain run produced AND the chain's stdout wire schema — the
// ONE home of the summary JSON `evolve loop --until-inbox-empty` prints after
// the per-batch documents (the host aliases it as chainResult and marshals it
// as is). ChainMode is stamped true by Run (a Result IS a chain run);
// BoundaryRefresh is the last audit entry when the stop is a re-exec, omitted
// on every ordinary stop (nil-when-clean); Exit is the process exit code and
// never on the wire.
type Result struct {
	ChainMode       bool             `json:"chain_mode"`
	MaxBatches      int              `json:"max_batches"`
	Batches         []BatchRecord    `json:"batches"`
	StopReason      string           `json:"chain_stop_reason"`
	BoundaryRefresh *RefreshLogEntry `json:"boundary_refresh,omitempty"`
	Exit            int              `json:"-"`
}

// Driver drives the batch until a boundary condition stops it. The same
// config is handed to EVERY batch — no per-batch re-derivation — so fleet
// width and every other resolved setting are preserved across the chain.
type Driver struct {
	roots  Roots
	cc     policy.ChainConfig
	deps   DriverDeps
	stderr io.Writer
	opts   options
}

// NewDriver builds the driver over its roots, the chain config, its deps and
// the warn writer the kept [chain] report lines print to.
func NewDriver(roots Roots, cc policy.ChainConfig, deps DriverDeps, warn io.Writer, opts ...Option) *Driver {
	return &Driver{roots: roots, cc: cc, deps: deps, stderr: warn, opts: newOptions(opts)}
}

// SignalsWired reports whether the driver currently reaches a Center.
func (d *Driver) SignalsWired() bool { return d.opts.center() != nil }

// Run is the chain loop: at every boundary, the pending-inbox read, the
// operator brake, the boundary refresh and the start decision — in that
// precedence — then one batch and the continue decision on its exit code.
func (d *Driver) Run() Result {
	res := Result{ChainMode: true, MaxBatches: d.cc.MaxBatches}
	for n := 0; ; n++ {
		pending, proceed := d.boundary(n, &res)
		if !proceed || d.runBatch(n, pending, &res) {
			break
		}
	}
	return res
}

// boundary decides whether batch n may start. Precedence: an unreadable
// inbox stops loudly (loop.halt); invalid items are named BEFORE the stop
// decision so the operator sees them even on the boundary that ends the
// chain; the operator brake is resolved ONCE and gates the side-effecting
// refresh; a refresh that fired is terminal (the new process resumes the
// chain); then the pure start decision.
func (d *Driver) boundary(n int, res *Result) (pending int, proceed bool) {
	pending, ok := d.pendingWork(n, res)
	if !ok {
		return 0, false
	}
	brake := BrakeEngaged(d.roots.EvolveDir)
	if brake {
		d.stop(res, StopOperatorBrake, pending)
		return 0, false
	}
	if d.deps.Refresh(n + 1) {
		res.StopReason = StopBoundaryRefreshReexec
		if entry, err := d.deps.LastRefresh(); err == nil {
			res.BoundaryRefresh = entry
		}
		fmt.Fprintf(d.stderr, "[chain] stopping after %d batch(es): %s — re-exec is terminal, the new process resumes the chain\n", len(res.Batches), res.StopReason)
		return 0, false
	}
	if reason, stop := StartDecision(n, d.cc.MaxBatches, pending, brake); stop {
		d.stop(res, reason, pending)
		return 0, false
	}
	return pending, true
}

// stop records reason and prints the ONE stop sentence the brake and the
// start decision share (a boundary stop that is not a re-exec).
func (d *Driver) stop(res *Result, reason string, pending int) {
	res.StopReason = reason
	fmt.Fprintf(d.stderr, "[chain] stopping after %d batch(es): %s (inbox pending=%d, cap=%d)\n", len(res.Batches), reason, pending, d.cc.MaxBatches)
}

// pendingWork counts the real pending items: an unreadable inbox is a
// loop.halt INCIDENT (exit 2, the chain never loops blind); every non-item
// *.json is a loop.warning WARN naming the file and is NOT counted.
func (d *Driver) pendingWork(n int, res *Result) (int, bool) {
	inbox := filepath.Join(d.roots.EvolveDir, "inbox")
	pending, skipped, err := InboxPendingCount(d.roots.EvolveDir)
	if err != nil {
		res.StopReason, res.Exit = StopInboxUnreadable, 2
		d.halt(CodeChainInboxUnreadable, fmt.Sprintf("cannot read the inbox (%v) — stopping the chain rather than looping blind", err),
			map[string]string{"batch": strconv.Itoa(n + 1), "path": inbox, "error": err.Error(), "stop_reason": res.StopReason, "exit": "2"})
		return 0, false
	}
	for _, name := range skipped {
		d.warn(CodeChainInboxItemInvalid, fmt.Sprintf("skipping .evolve/inbox/%s — not a valid inbox item (no parseable object with an `id`); it is NOT counted as pending work", name),
			map[string]string{"batch": strconv.Itoa(n + 1), "name": name, "path": filepath.Join(inbox, name)})
	}
	return pending, true
}

// runBatch records the width (read fresh, never re-resolved into the config
// — an operator widening mid-chain still takes effect; what must never
// happen is the CHAIN narrowing it), runs the batch and applies the continue
// decision: rc 5 defers on the quota wall (loop.warning), any other stop is
// a loop.halt INCIDENT propagating rc.
func (d *Driver) runBatch(n, pending int, res *Result) (stop bool) {
	width := d.deps.FleetWidth()
	fmt.Fprintf(d.stderr, "[chain] batch %d/%d starting — inbox pending=%d, fleet lanes=%d\n", n+1, d.cc.MaxBatches, pending, width)
	rc := d.deps.Batch()
	res.Batches = append(res.Batches, BatchRecord{Batch: n + 1, RC: rc, FleetCount: width, InboxPending: pending})
	reason, code, stop := ContinueDecision(rc)
	if !stop {
		return false
	}
	res.StopReason, res.Exit = reason, code
	if reason == StopQuotaDefer {
		d.quotaDefer(n + 1)
		return true
	}
	d.halt(CodeChainBatchError, fmt.Sprintf("batch %d exited rc=%d — stopping the chain (%s)", n+1, rc, reason),
		map[string]string{"batch": strconv.Itoa(n + 1), "rc": strconv.Itoa(rc), "stop_reason": reason})
	return true
}

// quotaDefer reports the deferral with the checkpoint's reset-time hint when
// one was written; the resume hint stays a report line. The point is the
// negative behaviour: the chain does NOT start another batch into families
// that are already drained.
func (d *Driver) quotaDefer(batch int) {
	fields := map[string]string{"batch": strconv.Itoa(batch), "cycle": "", "wake_at": "", "source": ""}
	if qp, ok := d.deps.QuotaPause(); ok {
		fields["cycle"], fields["wake_at"], fields["source"] = strconv.Itoa(qp.Cycle), qp.WakeAt, qp.Source
		d.warn(CodeChainQuotaDefer, fmt.Sprintf("batch %d hit the quota wall (cycle=%d wake-at=%s source=%s) — DEFERRING, not relaunching", batch, qp.Cycle, qp.WakeAt, qp.Source), fields)
	} else {
		d.warn(CodeChainQuotaDefer, fmt.Sprintf("batch %d hit the quota wall (no checkpoint block on disk) — DEFERRING, not relaunching", batch), fields)
	}
	fmt.Fprintln(d.stderr, "[chain]   the checkpoint is intact; resume when quota resets: evolve loop --resume")
}

// warn and halt are the driver's two producers over the ONE emit shape: a
// loop.warning WARN and a loop.halt INCIDENT, origin Driver.Run.
func (d *Driver) warn(code signalcenter.Code, reason string, fields map[string]string) {
	emit(d.opts.center(), "Driver.Run", signalcenter.KindLoopWarning, signalcenter.SeverityWarn, code, reason, fields)
}

func (d *Driver) halt(code signalcenter.Code, reason string, fields map[string]string) {
	emit(d.opts.center(), "Driver.Run", signalcenter.KindLoopHalt, signalcenter.SeverityIncident, code, reason, fields)
}
