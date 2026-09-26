// Package lifecycle moves inbox items between their lifecycle states and
// records every move on the chained ledger.
// See docs/architecture/packages/internal-inboxmover-lifecycle.md.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Codes of the inbox.warning conditions the mover reports; init registers each with its doc.
const (
	CodeClaimNotFound                  signalcenter.Code = "INBOX_CLAIM_NOT_FOUND"
	CodeClaimRefused                   signalcenter.Code = "INBOX_CLAIM_REFUSED"
	CodeClaimMoveFailed                signalcenter.Code = "INBOX_CLAIM_MOVE_FAILED"
	CodePromoteNotFound                signalcenter.Code = "INBOX_PROMOTE_NOT_FOUND"
	CodePromoteUnlandedSHA             signalcenter.Code = "INBOX_PROMOTE_UNLANDED_SHA"
	CodePromoteMoveFailed              signalcenter.Code = "INBOX_PROMOTE_MOVE_FAILED"
	CodeLandedCheckFailed              signalcenter.Code = "INBOX_LANDED_CHECK_FAILED"
	CodeReleaseDoubleMove              signalcenter.Code = "INBOX_RELEASE_DOUBLE_MOVE"
	CodeReleaseMoveFailed              signalcenter.Code = "INBOX_RELEASE_MOVE_FAILED"
	CodeQuarantineFailed               signalcenter.Code = "INBOX_QUARANTINE_FAILED"
	CodeContinuationManifestUnreadable signalcenter.Code = "INBOX_CONTINUATION_MANIFEST_UNREADABLE"
	CodeItemRewriteFailed              signalcenter.Code = "INBOX_ITEM_REWRITE_FAILED"
	CodeItemRoutedConsole              signalcenter.Code = "INBOX_ITEM_ROUTED_CONSOLE"
	CodeRouteNotFound                  signalcenter.Code = "INBOX_ROUTE_NOT_FOUND"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeClaimNotFound, "a lane claim named an id the inbox root does not hold (absent, already claimed into processing/, or parked); the claim returns ErrNotFound (exit 1) and moves nothing — the FAIL closeout's lane-scope claim of an id the triage persona already claimed reports this for an expected state; fields.step=locate, task_id, inbox_dir")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeClaimRefused, "a lane claim named an operator-owned item (route:console-* or a protected fix surface, ADR-0074 I1); the claim returns ErrConsoleRouted (exit 3) and the item stays at the root; fields.step=route, task_id, reason")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeClaimMoveFailed, "a claim could not create processing/cycle-N/ (fields.step=mkdir, dest_dir) or could not rename the item into it (fields.step=rename — the item may already be claimed); ErrMvFailed (exit 2), the item stays where it was; fields.task_id, err")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodePromoteNotFound, "a promote named an id neither processing/cycle-*/ nor the inbox root holds — already moved; the ship.sh-compat NoOp success is returned; fields.step=locate, task_id, state")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodePromoteUnlandedSHA, "a processed-promotion carried a ship sha the landing probe says is not on main; the item is rerouted to retry/ under reason ship-promote-retry-unlanded-sha instead of buried in processed/; fields.step=landing, task_id, sha, state=retry")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodePromoteMoveFailed, "a promote could not create its destination dir (fields.step=mkdir — ErrMvFailed, NoOp false, ledger promote-warn/mkdir-failed) or could not rename the item into it (fields.step=rename — the compat NoOp success, ledger promote-warn/mv-failed); the item stays where it was; fields.task_id, state, src_rel, dest, err")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeLandedCheckFailed, "the landing probe could not answer for a processed-promotion's sha — in production the host's git probe (shaLandedOnMain) on an exec fault or a git exit outside {0,1}: an unknown sha, no local main, a non-git ProjectRoot; the promote fails OPEN (the sha is treated as landed) so a gate fault never blocks a promotion, and the item lands in processed/ with this line as the only trace; fields.step=landing, task_id, sha, err")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeReleaseDoubleMove, "the cycle drain found the item's basename already at the inbox root (a concurrent release landed it first); the root copy is never clobbered, the processing copy stays and is not counted; fields.step=release_cycle, task_id, base")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeReleaseMoveFailed, "an item could not be renamed back to the inbox root — by the cycle drain (fields.step=release_cycle, origin Mover.Release) or by orphan recovery (fields.step=recover_orphans, origin Mover.RecoverOrphans); the item stays in processing/cycle-N/ and the walk continues; fields.task_id, base, err")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeQuarantineFailed, "an item at the ADR-0072 S5 retry ceiling could not be parked in quarantine/ (fields.outcome=error: the park's promote errored; outcome=noop: the park's rename failed) and falls open to a root release — the poison item WILL be re-picked; the preceding INBOX_PROMOTE_MOVE_FAILED from Mover.Promote names the cause; fields.step=quarantine, task_id, failure_count, ceiling")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeContinuationManifestUnreadable, "the FAILed cycle's continuation manifest exists but could not be read or parsed; every item of the drain releases unstamped (a later claim starts fresh instead of resuming the salvage); fields.step=manifest, workspace, err")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeItemRewriteFailed, "an item record could not be rewritten atomically: the closeout's route-console rewrite (step=route), the quarantine release's counter reset (fields.step=counter_reset — the item releases with its stale count), the drain's failure_count bump (fields.step=failure_bump — quarantine is skipped, the item releases to the root) or the drain's continuation stamp (fields.step=continuation_stamp — the item releases unstamped); fields.task_id, path, err")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeItemRoutedConsole, "a closeout routed an item to console-manual: the FAIL closeout when a phase gate refused it deterministically (a top_n card naming a protected surface — the per-item breaker of the 2026-09-14 poison-loop incident), or the planned-no-work closeout when a fleet lane's triage answered for its scoped item without committing it (F30 — the reason names the bucket and the lane's evidence): fields.task_id, path, reason, step=route — the item is operator-owned from here on and no lane claims it again")
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeRouteNotFound, "the FAIL closeout could not find the item it was told to route (fields.task_id, inbox_dir, step=locate) — or found it held by another cycle's claim (fields.held_by_cycle) — the refusal is NOT recorded on it and it WILL be re-picked; neither processing/cycle-*/ nor the inbox root holds the id")
}

// LegacyPrefix opens every console line the mover and its host print.
const LegacyPrefix = "[inbox-mover] "

// Sentinel errors: the cmd layer maps them to exit codes through errors.Is.
var (
	ErrNotFound = errors.New("inboxmover: task not found")
	ErrMvFailed = errors.New("inboxmover: mv failed")
	ErrBadArgs  = errors.New("inboxmover: bad arguments")
	ErrBadState = errors.New("inboxmover: invalid new_state")
	// ErrConsoleRouted refuses a lane claim of an operator-owned item.
	// See ADR-0074.
	ErrConsoleRouted = errors.New("inboxmover: item is console-routed (operator-owned) — refusing lane claim")
)

var validStates = map[string]bool{
	"processed":  true,
	"rejected":   true,
	"retry":      true,
	"quarantine": true,
}

// LedgerAppender appends a lifecycle record to the chained ledger; *ledger.FileLedger satisfies it.
type LedgerAppender interface {
	AppendLifecycle(ctx context.Context, r ledger.LifecycleRecord) error
}

// Mover moves the items of one inbox dir between lifecycle states.
type Mover struct {
	inboxDir     string
	ledger       LedgerAppender
	stderr       io.Writer
	now          func() time.Time
	activeCycle  func() (string, error)
	landed       func(sha string) (bool, error)
	isProtected  func(path string) bool
	retire       func(itemPath, taskID, reason string)
	runWorkspace func(cycle int) string
	signals      func() *signalcenter.Center
}

// Option configures a Mover at construction.
type Option func(*Mover)

// New builds a Mover over inboxDir; a nil appender records nothing and says nothing.
func New(inboxDir string, appender LedgerAppender, opts ...Option) *Mover {
	m := &Mover{
		inboxDir:    inboxDir,
		ledger:      appender,
		stderr:      io.Discard,
		now:         time.Now,
		activeCycle: func() (string, error) { return "", errors.New("lifecycle: no active-cycle reader wired") },
		landed:      func(string) (bool, error) { return true, nil },
		retire:      func(string, string, string) {},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithStderr sets the writer for the console lines; nil keeps io.Discard.
func WithStderr(w io.Writer) Option {
	return func(m *Mover) {
		if w != nil {
			m.stderr = w
		}
	}
}

// WithNow installs the clock the ledger TS is stamped from; nil keeps time.Now.
func WithNow(fn func() time.Time) Option {
	return func(m *Mover) {
		if fn != nil {
			m.now = fn
		}
	}
}

// WithActiveCycle sets RecoverOrphans' active-cycle reader; nil keeps a failing one, so every dir recovers.
func WithActiveCycle(fn func() (string, error)) Option {
	return func(m *Mover) {
		if fn != nil {
			m.activeCycle = fn
		}
	}
}

// WithLanded sets the landing probe for a processed promotion with a sha; nil treats every sha as landed.
func WithLanded(fn func(sha string) (bool, error)) Option {
	return func(m *Mover) {
		if fn != nil {
			m.landed = fn
		}
	}
}

// WithProtectedPath sets the claim floor's protected-path predicate; nil keeps the installed one.
func WithProtectedPath(fn func(path string) bool) Option {
	return func(m *Mover) {
		if fn != nil {
			m.isProtected = fn
		}
	}
}

// WithRetire sets the hook Promote fires between its console line and its ledger line; nil keeps the no-op.
func WithRetire(fn func(itemPath, taskID, reason string)) Option {
	return func(m *Mover) {
		if fn != nil {
			m.retire = fn
		}
	}
}

// WithRunWorkspace sets where the drain reads a cycle's continuation manifest; nil keeps the installed one.
func WithRunWorkspace(fn func(cycle int) string) Option {
	return func(m *Mover) {
		if fn != nil {
			m.runWorkspace = fn
		}
	}
}

// WithSignals sets the Signal Center accessor, read at every use; no Center selects the console fallback.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(m *Mover) { m.signals = c }
}

// SignalsWired reports whether the Mover currently reaches a Center.
func (m *Mover) SignalsWired() bool { return m.center() != nil }

func (m *Mover) center() *signalcenter.Center {
	if m.signals == nil {
		return nil
	}
	return m.signals()
}

// fault is one warning; legacy is the severity token the console fallback prints before reason.
type fault struct {
	code   signalcenter.Code
	origin string
	cycle  int
	legacy string
	reason string
	fields map[string]string
}

// warn emits f to the wired Center, or prints it as a console line when none is wired.
func (m *Mover) warn(f fault) {
	if c := m.center(); c != nil {
		c.Emit(signalcenter.Event{
			Cycle: f.cycle, Module: signalcenter.ModuleInbox, Origin: f.origin, Kind: signalcenter.KindInboxWarning,
			Severity: signalcenter.SeverityWarn, Code: f.code, Reason: f.reason, Fields: f.fields,
		})
		return
	}
	m.linef("%s%s", f.legacy, f.reason)
}

// linef prints a console line; the INFO, usage and ledger-append lines print whether or not a Center is wired.
func (m *Mover) linef(format string, args ...any) {
	fmt.Fprintf(m.stderr, LegacyPrefix+format+"\n", args...)
}
