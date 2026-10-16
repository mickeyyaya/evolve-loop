package loopchain

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type QuotaPause struct {
	Cycle  int
	WakeAt string
	Source string
}

type DriverDeps struct {
	Batch       func() int
	Refresh     func(batch int) bool
	LastRefresh func() (*RefreshLogEntry, error)
	FleetWidth  func() int
	QuotaPause  func() (QuotaPause, bool)
}

type Result struct {
	ChainMode       bool             `json:"chain_mode"`
	MaxBatches      int              `json:"max_batches"`
	Batches         []BatchRecord    `json:"batches"`
	StopReason      string           `json:"chain_stop_reason"`
	BoundaryRefresh *RefreshLogEntry `json:"boundary_refresh,omitempty"`
	Exit            int              `json:"-"`
}

type Driver struct {
	roots  Roots
	cc     policy.ChainConfig
	deps   DriverDeps
	stderr io.Writer
	opts   options
}

func NewDriver(roots Roots, cc policy.ChainConfig, deps DriverDeps, warn io.Writer, opts ...Option) *Driver {
	return &Driver{roots: roots, cc: cc, deps: deps, stderr: warn, opts: newOptions(opts)}
}

func (d *Driver) SignalsWired() bool { return d.opts.center() != nil }

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

func (d *Driver) stop(res *Result, reason string, pending int) {
	res.StopReason = reason
	fmt.Fprintf(d.stderr, "[chain] stopping after %d batch(es): %s (inbox pending=%d, cap=%d)\n", len(res.Batches), reason, pending, d.cc.MaxBatches)
}

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

func (d *Driver) warn(code signalcenter.Code, reason string, fields map[string]string) {
	emit(d.opts.center(), "Driver.Run", signalcenter.KindLoopWarning, signalcenter.SeverityWarn, code, reason, fields)
}

func (d *Driver) halt(code signalcenter.Code, reason string, fields map[string]string) {
	emit(d.opts.center(), "Driver.Run", signalcenter.KindLoopHalt, signalcenter.SeverityIncident, code, reason, fields)
}
