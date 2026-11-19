// Package loopchain chains loop batches and refreshes the running binary at each boundary.
package loopchain

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBoundaryRefreshSkipped     signalcenter.Code = "LOOP_BOUNDARY_REFRESH_SKIPPED"
	CodeBoundaryRefreshAuditFailed signalcenter.Code = "LOOP_BOUNDARY_REFRESH_AUDIT_FAILED"
	CodeChainInboxItemInvalid      signalcenter.Code = "LOOP_CHAIN_INBOX_ITEM_INVALID"
	CodeChainQuotaDefer            signalcenter.Code = "LOOP_CHAIN_QUOTA_DEFER"
	CodeChainInboxUnreadable       signalcenter.Code = "LOOP_CHAIN_INBOX_UNREADABLE"
	CodeChainBatchError            signalcenter.Code = "LOOP_CHAIN_BATCH_ERROR"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeBoundaryRefreshSkipped, "the boundary binary refresh degraded to continue on the current binary at fields.step (ahead_check, lane_check, lane_active, breaker, interrupted, rebuild, target, repin, argv, arm, or reexec). Causes include check errors, active sibling leases, breaker hits, signals, build failure, missing binary, or exec failure. Fields: fields.batch, commit, error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeBoundaryRefreshAuditFailed, "the boundary-refresh-log.jsonl audit entry failed to append AFTER the state.json pin moved. Refresh continues to re-exec; pin and audit trail now disagree. fields.batch, path, error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainInboxItemInvalid, "a root-level .evolve/inbox/*.json failed to parse as an inbox item (no object with a non-empty id). It is NOT counted as pending work. fields.batch, name, path")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainQuotaDefer, "a chained batch exited with resumable rc=5. The chain defers instead of relaunching into the wall or red main (resume with evolve loop --resume). fields.pause names the pause; fields.batch, cycle, wake_at, source come from checkpoint")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainInboxUnreadable, "the chain failed to read .evolve/inbox (a read error other than not-exist) and stopped rather than loop blind; fields.batch, path, error, stop_reason=chain_inbox_unreadable, exit=2")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainBatchError, "a chained batch exited with a non-continuable rc (not 0/3/5) and the chain stopped, propagating rc. The batch's own signals name the failure. fields.batch, rc, stop_reason=chain_batch_error")
}

const (
	AttemptFile                    = "boundary-refresh-attempt.json"
	LogFile                        = "boundary-refresh-log.jsonl"
	AuthorizedClassBoundaryRefresh = "boundary-refresh"
)

type Roots struct {
	ProjectRoot string
	EvolveDir   string
}

type options struct {
	signals     func() *signalcenter.Center
	now         func() time.Time
	attemptFile string
	logFile     string
	handoff     Handoff
}

type Option func(*options)

func newOptions(opts []Option) options {
	o := options{now: time.Now, attemptFile: AttemptFile, logFile: LogFile}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(o *options) { o.signals = c }
}

func WithNow(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

func WithMarkerFiles(attempt, log string) Option {
	return func(o *options) { o.attemptFile, o.logFile = attempt, log }
}

type Handoff struct {
	PID       int
	WavesDone int
}

func WithHandoff(h Handoff) Option {
	return func(o *options) { o.handoff = h }
}

func (o options) center() *signalcenter.Center {
	if o.signals == nil {
		return nil
	}
	return o.signals()
}

func emit(c *signalcenter.Center, origin string, kind signalcenter.Kind, sev signalcenter.Severity, code signalcenter.Code, reason string, fields map[string]string) {
	c.Emit(signalcenter.Event{
		Module: signalcenter.ModuleLoop, Origin: origin, Kind: kind, Severity: sev, Code: code, Reason: reason, Fields: fields,
	})
}
