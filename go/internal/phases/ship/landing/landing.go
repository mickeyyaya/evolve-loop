// Package landing is the two-phase landing of ship: the fast-forward merge,
// the push with its inline repair, and the ship-binding witness.
// See docs/architecture/packages/internal-phases-ship-landing.md.
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

const (
	stepIntegrate = "integrate"
	stepPush      = "push"
	stepBinding   = "binding"
	stepResume    = "resume"
)

type Git func(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error)

type Streams struct {
	Stdout io.Writer
	Stderr io.Writer
}

type Landing struct {
	git     Git
	streams func() Streams
	phase   string
	cycle   int
	runID   string
	signals func() *signalcenter.Center
	sleep   func(time.Duration)
}

type Option func(*Landing)

func New(git Git, streams func() Streams, opts ...Option) *Landing {
	l := &Landing{git: git, streams: streams, sleep: time.Sleep}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Landing) { l.signals = c }
}

func WithSleep(sleep func(time.Duration)) Option {
	return func(l *Landing) { l.sleep = sleep }
}

func WithRun(phase string, cycle int, runID string) Option {
	return func(l *Landing) { l.phase, l.cycle, l.runID = phase, cycle, runID }
}

func (l *Landing) SignalsWired() bool { return l.center() != nil }

func (l *Landing) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}

func (l *Landing) warn(origin string, code signalcenter.Code, reason string, fields map[string]string) {
	l.center().Emit(signalcenter.Event{
		Cycle: l.cycle, RunID: l.runID, Phase: l.phase, Module: signalcenter.ModuleShip, Origin: origin,
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}

func fail(step string, code shiperr.ShipErrorCode, class shiperr.ShipErrorClass, msg string, kv ...string) *shiperr.ShipError {
	return shiperr.NewShipError(code, class, shiperr.StageAtomicShip, msg, append(kv, shiperr.StepKey, step)...)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func emitLog(sink func(string), line string) {
	if sink != nil {
		sink(line)
	}
}
