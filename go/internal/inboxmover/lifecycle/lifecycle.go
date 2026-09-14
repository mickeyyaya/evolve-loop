// Package lifecycle is unit 06 of the component breakdown (ADR-0103): the
// inbox lifecycle mover. One Mover owns the five transitions over ONE inbox
// dir — Claim (inbox/ → processing/cycle-N/), Promote (→ processed | rejected
// | retry | quarantine), ReleaseFromQuarantine, the cycle drain Release (with
// the ADR-0072 S5 failure_count bump and quarantine park) and RecoverOrphans —
// plus the processed-record primitives (the one id→file resolver, the one
// atomic item rewrite, the failure counter's one reader and one writer) and
// the chained inbox-lifecycle ledger line. Every collaborator is injected at
// construction with a Null-Object default: the ledger appender, stderr, the
// clock, the active-cycle reader, the landing probe, the protected-path
// predicate, the continuation retire hook, the run-workspace spelling and the
// Signal Center accessor. The host (internal/inboxmover) resolves the
// production defaults, builds the Mover once per call and keeps every caller's
// spelling behind its facades. The leaf never imports gitexec, core, the
// guards or the host. Failure modes report as inbox.warning under module
// inbox through ONE producer with two links: the Center when a root wired one,
// else the byte-identical legacy `[inbox-mover] WARN:|ERROR:` line onto the
// injected stderr — nothing goes silent on a Center-less root.
// Design: docs/architecture/decomposition/06-inboxmover.md.
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

// The unit's codes — twelve WARN conditions that replaced fifteen hand-written
// stderr lines and gave three silent arms a voice — registered with their docs.
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
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeItemRewriteFailed, "an item record could not be rewritten atomically: the quarantine release's counter reset (fields.step=counter_reset — the item releases with its stale count), the drain's failure_count bump (fields.step=failure_bump — quarantine is skipped, the item releases to the root) or the drain's continuation stamp (fields.step=continuation_stamp — the item releases unstamped); fields.task_id, path, err")
}

// LegacyPrefix is the console voice of every line the mover prints — the
// fallback link, the kept INFO/usage lines — and of the host's sibling lines
// (its logf) until unit 06b folds that renderer: ONE spelling, consumed
// everywhere (TestInboxMoverPrefix_OneHome).
const LegacyPrefix = "[inbox-mover] "

// Sentinel errors — the exit-code contract of the cmd layer (errors.Is on the
// same pointers the host re-exports).
var (
	ErrNotFound = errors.New("inboxmover: task not found")
	ErrMvFailed = errors.New("inboxmover: mv failed")
	ErrBadArgs  = errors.New("inboxmover: bad arguments")
	ErrBadState = errors.New("inboxmover: invalid new_state")
	// ErrConsoleRouted refuses the lane handoff of an operator-owned item
	// (ADR-0074 I1): route:"console-*" or a protected fix surface. Prompts
	// advise; Claim enforces — a triage LLM naming the item cannot move it.
	ErrConsoleRouted = errors.New("inboxmover: item is console-routed (operator-owned) — refusing lane claim")
)

// validStates is the set of allowed promote targets. "quarantine" is the
// ADR-0072 S5 terminal state: a task that has failed task_retry_ceiling times
// routes here (a sibling dir the triage scanner never walks) instead of being
// released back to the inbox root every cycle, so a poison todo stops being
// re-picked forever.
var validStates = map[string]bool{
	"processed":  true,
	"rejected":   true,
	"retry":      true,
	"quarantine": true,
}

// LedgerAppender is the chained-append seam (interface at point of use);
// satisfied by *ledger.FileLedger.
type LedgerAppender interface {
	AppendLifecycle(ctx context.Context, r ledger.LifecycleRecord) error
}

// Mover owns the inbox lifecycle transitions over ONE inbox dir. Two
// positional required collaborators (the dir and the ledger appender — a nil
// appender is the Null Object: nothing is appended, nothing is said) and
// eight functional options, each with a Null-Object default, so a leaf test
// needs no git, no registry and no Center, and every production literal that
// leaves a seam nil behaves exactly as it did before the unit.
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

// Option configures a Mover at construction (functional options).
type Option func(*Mover)

// New builds the Mover over its inbox dir and ledger appender.
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

// WithStderr installs the writer the kept INFO/usage lines and the fallback
// link print to; nil keeps io.Discard.
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

// WithActiveCycle installs the active-cycle reader RecoverOrphans consults;
// nil keeps the erroring default, whose swallowed error means "-1" — every
// processing dir recovers.
func WithActiveCycle(fn func() (string, error)) Option {
	return func(m *Mover) {
		if fn != nil {
			m.activeCycle = fn
		}
	}
}

// WithLanded installs the delivery-evidence probe a processed-promotion with a
// sha consults; nil keeps the default that treats every sha as landed.
func WithLanded(fn func(sha string) (bool, error)) Option {
	return func(m *Mover) {
		if fn != nil {
			m.landed = fn
		}
	}
}

// WithProtectedPath installs the control-plane membership predicate of the
// ADR-0074 claim floor; nil keeps what is installed (the default nil disables
// only the files-derived rule — an explicit route:"console-*" field always
// refuses).
func WithProtectedPath(fn func(path string) bool) Option {
	return func(m *Mover) {
		if fn != nil {
			m.isProtected = fn
		}
	}
}

// WithRetire installs the hook Promote fires after the INFO line and before
// the ledger line (the host releases the item's continuation binding there);
// nil keeps the no-op.
func WithRetire(fn func(itemPath, taskID, reason string)) Option {
	return func(m *Mover) {
		if fn != nil {
			m.retire = fn
		}
	}
}

// WithRunWorkspace installs the cycle → run-workspace spelling the drain reads
// the continuation manifest from; nil keeps what is installed (the default nil
// means no manifest is read).
func WithRunWorkspace(fn func(cycle int) string) Option {
	return func(m *Mover) {
		if fn != nil {
			m.runWorkspace = fn
		}
	}
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use. A nil accessor, or one returning nil, selects
// the fallback link: the legacy line on the injected stderr.
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

// fault is the Parameter Object of the one producer: the code, the exported
// method that produced it, the cycle the call knows, the legacy severity token
// ("WARN: " | "ERROR: ") the fallback link prints, the sentence (the legacy
// line minus its prefix) and the fields a triage needs (always step, task_id).
type fault struct {
	code   signalcenter.Code
	origin string
	cycle  int
	legacy string
	reason string
	fields map[string]string
}

// warn is the unit's ONE producer with two links: a wired Center receives an
// inbox.warning WARN under module inbox; a Center-less root gets the legacy
// `[inbox-mover] <token><sentence>` line, byte for byte what the hand-written
// logf printed before the unit — so no root goes silent (06-F1 removes the
// second link root by root as each gains a Center).
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

// linef prints a kept line — the INFO transitions, the usage/bad-args ERROR
// lines and the ledger-append WARN — in the legacy voice on both links.
func (m *Mover) linef(format string, args ...any) {
	fmt.Fprintf(m.stderr, LegacyPrefix+format+"\n", args...)
}
