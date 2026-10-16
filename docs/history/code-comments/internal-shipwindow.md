# Comment history: `internal/shipwindow`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d3

### `go/internal/shipwindow/shipwindow.go:1` — above `package shipwindow`

```text
// Package shipwindow serializes the audit→ship critical section across
// concurrent lanes (cycle-778, inbox ship-window-lease). A sibling lane
// landing on main between this lane's audit-binding HEAD snapshot
// (core recordAuditBinding) and its ship push forces a deep-tier
// AUDIT_BINDING_HEAD_MOVED re-audit; measured over cycles 767-774 that
// inflation ran audit ~10x for 8 cycles. width-scaled-binding-retry makes
// lanes SURVIVE the race by retrying; this lease makes them AVOID it by
// making the binding-snapshot→push window mutually exclusive.
//
// Semantics (run-lease liveness pattern, runlease.OwnerLive):
//   - The lease lives at <evolveDir>/ship-window.lock, one JSON object,
//     created atomically (tmp + link) so a reader never sees a torn file.
//   - A held lease is respected only while LIVE: heartbeat Fresh within TTL
//     AND holder pid alive. Stale on either boundary (dead holder before TTL,
//     or TTL expiry despite a live-but-hung holder) ⇒ waiters break it and
//     proceed — the lease is the sole liveness oracle, no operator needed.
//   - Waiters queue as ticket files under <evolveDir>/ship-window.queue and
//     acquire in FIFO order of their Acquire calls (starvation fairness).
//     A crashed waiter's ticket (dead pid) is swept so it cannot wedge FIFO.
//   - No heartbeat refresh while held: the guarded window is
//     binding-snapshot→push (~5-10 min observed), and DefaultTTL is a ceiling
//     comfortably above it. A holder that outlives the TTL is by definition
//     hung and MUST be breakable — refreshing would defeat that.
```

### `go/internal/shipwindow/shipwindow_test.go:3` — above `import (`

```text
// shipwindow_test.go — RED test contract for the ship-window lease
// (cycle-778, inbox id ship-window-lease, weight 0.97, operator-boosted).
//
// PROBLEM (evolve tokens report, cycles 767-774): audit ran ~10x for 8 cycles.
// The extras are AUDIT_BINDING_HEAD_MOVED re-audits: a sibling lane lands on
// main between this lane's audit-binding snapshot (core/phase_bindings.go
// recordAuditBinding: `git rev-parse HEAD`) and its ship push, so ship's
// verifyAuditBinding sees a moved HEAD and forces a deep-tier re-audit.
// width-scaled-binding-retry makes lanes SURVIVE that race; this lease makes
// them AVOID it by serializing ONLY the binding-snapshot→push critical section.
//
// CONTRACT the Builder implements in this package (tests are the spec;
// do NOT modify them — implement production code until they pass):
//
//	// PathIn returns the lease file path: <evolveDir>/ship-window.lock.
//	func PathIn(evolveDir string) string
//
//	// DefaultTTL is the stale-break ceiling for a held lease. The window is
//	// binding-snapshot→push (~5-10 min observed), NOT the whole audit; pick
//	// a ceiling comfortably above that (runlease.DefaultTTL is the pattern).
//	const/var DefaultTTL time.Duration
//
//	type Options struct {
//	    TTL      time.Duration      // 0 ⇒ DefaultTTL
//	    Now      func() time.Time   // nil ⇒ time.Now (test seam, runlease-style)
//	    Alive    func(pid int) bool // nil ⇒ real pid probe; holder-death oracle
//	    Poll     time.Duration      // waiter poll interval; 0 ⇒ sane default
//	    OwnerPID int                // 0 ⇒ os.Getpid()
//	}
//
//	// Acquire blocks until the caller holds the ship-window lease for
//	// evolveDir, or ctx is done (then returns ctx's error). A held lease is
//	// respected only while LIVE (runlease semantics: heartbeat Fresh within
//	// TTL AND holder pid alive); a stale lease (TTL expired OR holder dead)
//	// is broken and acquisition proceeds. Waiters acquire in FIFO order of
//	// their Acquire calls (starvation fairness).
//	func Acquire(ctx context.Context, evolveDir string, opts Options) (*Lease, error)
//
//	// Release frees the lease so the next queued waiter proceeds.
//	func (l *Lease) Release() error
```
