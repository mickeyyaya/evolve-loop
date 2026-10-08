// Package landing is the two-phase landing of ship; see docs/architecture/decomposition/07-shipgitops.md.
package landing

import (
	"context"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBinaryResetFailed  signalcenter.Code = "SHIP_LANDING_BINARY_RESET_FAILED"
	CodePushRepairDeclined signalcenter.Code = "SHIP_LANDING_PUSH_REPAIR_DECLINED"
	CodeHeadReadFailed     signalcenter.Code = "SHIP_LANDING_HEAD_READ_FAILED"
	CodeBindingWriteFailed signalcenter.Code = "SHIP_LANDING_BINDING_WRITE_FAILED"
	CodeAdvanceFailed      signalcenter.Code = "SHIP_LANDING_MAIN_ADVANCE_FAILED"
	CodeOriginFetchFailed  signalcenter.Code = "SHIP_LANDING_RESUME_FETCH_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeBinaryResetFailed, "git checkout HEAD -- <binary> before the ff-merge exited non-zero or failed to spawn; the merge still runs and may fail if the tracked binary is dirty; fields.step=integrate, path, git_rc, git_err")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodePushRepairDeclined, "the inline repair after a rejected push declined at the named probe (fetch, origin_ref, head, push_retry, or transport_retry after the bounded backoff); the original transient GIT_PUSH_REJECTED returns with repair_attempted/repair_outcome=declined; fields.step=push, branch, probe")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeHeadReadFailed, "git rev-parse HEAD after the push landed errored or returned empty; the result's CommitSHA (the dossier's delivery identity) stays empty and the ship proceeds; fields.step=push, ref, err")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeBindingWriteFailed, "ship-binding.json could not be created, written or renamed into the run workspace; the push already landed, the caller keeps shipping and keeps its WARN log line; fields.step=binding, path, err")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeAdvanceFailed, "after the push landed, the fast-forward of the plane main to the pushed commit failed; origin holds the commit, and the ship continues; the next landing fast-forwards main to origin under the ship lock when the ship journal holds each commit between them; fields.step=integrate, branch, commit, git_rc, git_err")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeOriginFetchFailed, "the git fetch of origin before a landing resume failed; the resume decides on the last known origin ref, and its push reports an outage; fields.step=resume, branch, git_rc, git_err")
}

// The step vocabulary the landing stamps into shiperr.StepKey on every error
// it builds and into fields.step on every warning.
const (
	stepIntegrate = "integrate"
	stepPush      = "push"
	stepBinding   = "binding"
	stepResume    = "resume"
)

// Git runs `git <args>` at the project root with the host's runner, cwd and
// environment, streaming to the given writers — the host's Options.run curried
// with "git". Every landing step runs at the project root (no -C).
type Git func(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error)

// Streams are the ship's operator streams the merge and the pushes write to;
// probes and captures use io.Discard exactly as before the move.
type Streams struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Landing owns the fleet landing. Stateless: the once-per-Run guard of the
// push repair stays on the host's Options and arrives in each PushRequest.
type Landing struct {
	git     Git
	streams func() Streams
	phase   string
	cycle   int
	runID   string
	signals func() *signalcenter.Center
	sleep   func(time.Duration)
}

// Option configures a Landing at construction (functional options).
type Option func(*Landing)

// New builds the landing over its two required collaborators: the Git port
// and the streams accessor (read at every use — the host defaults its streams
// after construction and direct-helper tests set them late). A nil git or
// streams is a programming error and panics at first use — no guard.
func New(git Git, streams func() Streams, opts ...Option) *Landing {
	l := &Landing{git: git, streams: streams, sleep: time.Sleep}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use, because the host's Center is a field set by
// the root after the options are built. A nil accessor, or one returning nil,
// is the Null Object; SignalsWired proves the production roots wired one.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Landing) { l.signals = c }
}

func WithSleep(sleep func(time.Duration)) Option {
	return func(l *Landing) { l.sleep = sleep }
}

// WithRun stamps where every event the landing emits comes from: the phase
// name (Event.Phase — the host's spelling of core.PhaseShip, handed in because
// the leaf cannot import core and spells no phase itself) and the run
// identity (Event.Cycle / Event.RunID; a standalone `evolve ship` carries 0
// and ""). Without it all three stay empty.
func WithRun(phase string, cycle int, runID string) Option {
	return func(l *Landing) { l.phase, l.cycle, l.runID = phase, cycle, runID }
}

// SignalsWired reports whether the landing currently reaches a Center.
func (l *Landing) SignalsWired() bool { return l.center() != nil }

func (l *Landing) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}

// warn is the unit's one producer: a ship.warning WARN under module ship from
// the exported method named by origin, stamped with the run identity. A nil
// Center is the Null Object (Emit on nil is a no-op).
func (l *Landing) warn(origin string, code signalcenter.Code, reason string, fields map[string]string) {
	l.center().Emit(signalcenter.Event{
		Cycle: l.cycle, RunID: l.runID, Phase: l.phase, Module: signalcenter.ModuleShip, Origin: origin,
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}

// fail builds a ShipError at the atomic-ship stage with the landing step
// stamped into its Debug map beside the caller's keys — the one construction
// site of every error the landing returns.
func fail(step string, code shiperr.ShipErrorCode, class shiperr.ShipErrorClass, msg string, kv ...string) *shiperr.ShipError {
	return shiperr.NewShipError(code, class, shiperr.StageAtomicShip, msg, append(kv, shiperr.StepKey, step)...)
}

// errText renders err for a Debug-map value; "" when nil (the host's errStr
// rule — a declared four-line twin until the commit sites move, 07-F8).
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// emitLog appends a line to the caller's log sink; a nil sink is the Null
// Object.
func emitLog(sink func(string), line string) {
	if sink != nil {
		sink(line)
	}
}
