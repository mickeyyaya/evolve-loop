// Package loopchain is unit 13 of the component breakdown (ADR-0103): the
// loop's chain engine. The Driver is the outer batch-chaining loop (cycle
// 1075; the standing operator directive that lanes keep running until the
// inbox is empty) — at every boundary it decides whether another batch may
// start (operator brake > boundary refresh > drained inbox > runaway cap) and
// after every batch how the exit code moves the chain (rc 0/3 continue, rc 5
// defers on the quota wall, anything else stops and propagates). The
// Refresher is the boundary binary refresh (cycle 1314): ahead-check →
// fleet-lane guard → loop breaker → rebuild → re-exec target → provenance-
// gated re-pin → audit log → arm the breaker → flush the Center → exec. Both
// take their process collaborators explicitly (the batch, the rebuild, the
// exec, the running commit, the quota-pause reader) so the leaf is core-free
// and the host's package-var test seams project into them at call time.
// Failure modes are loop.warning WARN and loop.halt INCIDENT under module
// loop; the INFO-shaped [chain] report lines stay on the warn writer. Design:
// docs/architecture/decomposition/13-loopwave.md.
package loopchain

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes: the ten refresh degrade branches under ONE code naming
// the step, the audit failure, and the chain's four boundary conditions.
const (
	CodeBoundaryRefreshSkipped     signalcenter.Code = "LOOP_BOUNDARY_REFRESH_SKIPPED"
	CodeBoundaryRefreshAuditFailed signalcenter.Code = "LOOP_BOUNDARY_REFRESH_AUDIT_FAILED"
	CodeChainInboxItemInvalid      signalcenter.Code = "LOOP_CHAIN_INBOX_ITEM_INVALID"
	CodeChainQuotaDefer            signalcenter.Code = "LOOP_CHAIN_QUOTA_DEFER"
	CodeChainInboxUnreadable       signalcenter.Code = "LOOP_CHAIN_INBOX_UNREADABLE"
	CodeChainBatchError            signalcenter.Code = "LOOP_CHAIN_BATCH_ERROR"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeBoundaryRefreshSkipped, "the boundary binary refresh degraded to 'continue on the current binary' at fields.step (ahead_check | lane_check | lane_active | breaker | rebuild | target | repin | argv | arm | reexec): the ahead-check errored, a sibling fleet lane held a fresh lease (or the check was unverifiable), the loop breaker refused a second refresh for the same build commit, `make -C go build` failed, no executable at go/bin/evolve, the state.json re-pin was refused, the re-exec argv was empty, the breaker marker could not be written, or exec failed; fields.batch, commit (the running build commit), error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeBoundaryRefreshAuditFailed, "the boundary-refresh-log.jsonl audit entry could not be appended AFTER the state.json pin had already moved; the refresh continues to re-exec — the pin and the audit trail now disagree; fields.batch, path, error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainInboxItemInvalid, "a root-level .evolve/inbox/*.json did not parse as an inbox item (no object with a non-empty id) and is NOT counted as pending work — usually a real todo lost to a typo; fields.batch, name, path")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainQuotaDefer, "a chained batch exited with the resumable rc=5 quota-pause code; the chain defers instead of relaunching into the wall (resume with evolve loop --resume); fields.batch, cycle, wake_at, source are the checkpoint block's (empty when no block is on disk)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainInboxUnreadable, "the chain could not read .evolve/inbox (a read error other than not-exist) and stopped rather than loop blind; fields.batch, path, error, stop_reason=chain_inbox_unreadable, exit=2")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeChainBatchError, "a chained batch exited with a non-continuable rc (not 0/3/5) and the chain stopped, propagating rc; the batch's own signals name the failure (an rc=2 batch may have emitted none); fields.batch, rc, stop_reason=chain_batch_error")
}

// The on-disk names under .evolve/: the re-exec loop breaker marker and the
// additive boundary-refresh audit trail; the authorization class the trail
// stamps (distinguishable from state.json's own two-value Authorized enum).
const (
	AttemptFile                    = "boundary-refresh-attempt.json"
	LogFile                        = "boundary-refresh-log.jsonl"
	AuthorizedClassBoundaryRefresh = "boundary-refresh"
)

// Roots carries the project root (the git checkout the refresh rebuilds and
// re-pins) and the evolve dir (the inbox, the brake, the markers, state.json).
type Roots struct {
	ProjectRoot string
	EvolveDir   string
}

// options are the optional collaborators the Refresher and the Driver share.
type options struct {
	signals     func() *signalcenter.Center
	now         func() time.Time
	attemptFile string
	logFile     string
}

// Option configures a Refresher or a Driver at construction.
type Option func(*options)

func newOptions(opts []Option) options {
	o := options{now: time.Now, attemptFile: AttemptFile, logFile: LogFile}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use. A nil accessor, or one returning nil, is the
// Null Object.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(o *options) { o.signals = c }
}

// WithNow fixes the clock the marker and the audit entry stamp.
func WithNow(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

// WithMarkerFiles overrides the breaker marker and audit log file names
// (relative to EvolveDir); production uses AttemptFile and LogFile.
func WithMarkerFiles(attempt, log string) Option {
	return func(o *options) { o.attemptFile, o.logFile = attempt, log }
}

func (o options) center() *signalcenter.Center {
	if o.signals == nil {
		return nil
	}
	return o.signals()
}

// emit is the unit's one producer shape: a batch-level (cycle 0) event under
// module loop from origin, delivered to the Center (nil is a no-op).
func emit(c *signalcenter.Center, origin string, kind signalcenter.Kind, sev signalcenter.Severity, code signalcenter.Code, reason string, fields map[string]string) {
	c.Emit(signalcenter.Event{
		Module: signalcenter.ModuleLoop, Origin: origin, Kind: kind, Severity: sev, Code: code, Reason: reason, Fields: fields,
	})
}
