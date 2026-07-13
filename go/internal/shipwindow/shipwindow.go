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
package shipwindow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// FileName is the lease file's name inside the .evolve directory — the
// identity operators grep and gc sweeps.
const FileName = "ship-window.lock"

// DefaultTTL is the stale-break ceiling for a held lease. The guarded window
// is binding-snapshot→push (~5-10 min observed), NOT the whole audit; 15
// minutes sits comfortably above it while still bounding how long a hung
// holder can wedge sibling ships.
const DefaultTTL = 15 * time.Minute

const (
	queueDirName = "ship-window.queue"
	defaultPoll  = 500 * time.Millisecond
)

// ticketSeq breaks same-nanosecond ticket-name ties within a process so FIFO
// order stays total even under a frozen Now test seam.
var ticketSeq uint64

// Options configures Acquire. The zero value is production-ready: real clock,
// real pid probe, DefaultTTL, sane poll, own pid.
type Options struct {
	// TTL is the stale-break ceiling; 0 ⇒ DefaultTTL.
	TTL time.Duration
	// Now is the clock seam (runlease-style); nil ⇒ time.Now.
	Now func() time.Time
	// Alive is the holder-death oracle; nil ⇒ a real pid probe.
	Alive func(pid int) bool
	// Poll is the waiter re-check interval; 0 ⇒ a sane default.
	Poll time.Duration
	// OwnerPID identifies the holder for liveness probes; 0 ⇒ os.Getpid().
	OwnerPID int
}

func (o Options) normalized() Options {
	if o.TTL <= 0 {
		o.TTL = DefaultTTL
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Alive == nil {
		o.Alive = pidAlive
	}
	if o.Poll <= 0 {
		o.Poll = defaultPoll
	}
	if o.OwnerPID == 0 {
		o.OwnerPID = os.Getpid()
	}
	return o
}

// record is the on-disk lock schema. HeartbeatAt/OwnerPID mirror
// runlease.Lease so liveness classification reuses runlease.OwnerLive
// verbatim; Token uniquely identifies the acquisition so Release never
// removes a lease that was broken and re-acquired by another lane.
type record struct {
	OwnerPID    int    `json:"owner_pid"`
	HeartbeatAt string `json:"heartbeat_at"`
	Token       string `json:"token"`
}

// Lease is a held ship-window lease; Release frees it.
type Lease struct {
	path  string
	token string
}

// PathIn returns the lease file path for an .evolve directory:
// <evolveDir>/ship-window.lock.
func PathIn(evolveDir string) string {
	return filepath.Join(evolveDir, FileName)
}

// Acquire blocks until the caller holds the ship-window lease for evolveDir,
// or ctx is done (then returns ctx's error). A held lease is respected only
// while LIVE (heartbeat Fresh within TTL AND holder pid alive); a stale lease
// is broken and acquisition proceeds. Waiters acquire in FIFO order of their
// Acquire calls.
func Acquire(ctx context.Context, evolveDir string, opts Options) (*Lease, error) {
	o := opts.normalized()
	queueDir := filepath.Join(evolveDir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		return nil, fmt.Errorf("shipwindow: queue dir: %w", err)
	}
	// Ticket names sort lexicographically = chronologically (zero-padded
	// nanos, then a per-process sequence for frozen-clock ties, then pid for
	// cross-process uniqueness).
	ticket := fmt.Sprintf("%020d-%09d-%d", o.Now().UnixNano(), atomic.AddUint64(&ticketSeq, 1), o.OwnerPID)
	ticketPath := filepath.Join(queueDir, ticket)
	if err := os.WriteFile(ticketPath, []byte(strconv.Itoa(o.OwnerPID)+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("shipwindow: enqueue: %w", err)
	}
	for {
		l, err := tryAcquire(evolveDir, queueDir, ticket, o)
		if err != nil || l != nil {
			_ = os.Remove(ticketPath)
			return l, err
		}
		select {
		case <-ctx.Done():
			_ = os.Remove(ticketPath)
			return nil, ctx.Err()
		case <-time.After(o.Poll):
		}
	}
}

// tryAcquire makes one non-blocking acquisition attempt: FIFO head gate →
// liveness check (break if stale) → atomic create-with-content (link). A nil
// Lease with nil error means "not yet — poll again".
func tryAcquire(evolveDir, queueDir, ticket string, o Options) (*Lease, error) {
	head, err := queueHead(queueDir, ticket, o.Alive)
	if err != nil {
		return nil, err
	}
	if head != ticket {
		return nil, nil
	}
	lockPath := PathIn(evolveDir)
	rec, ok, err := readRecord(lockPath)
	if err != nil && !ok {
		return nil, fmt.Errorf("shipwindow: read lease: %w", err)
	}
	// err with ok=true is a present-but-unparsable lock. Atomic link writes
	// make torn files impossible, so corruption is crash debris — the zero
	// record's empty heartbeat classifies it stale below and it gets broken.
	if err != nil {
		rec = record{}
	}
	if ok {
		if runlease.OwnerLive(runlease.Lease{OwnerPID: rec.OwnerPID, HeartbeatAt: rec.HeartbeatAt}, o.Now(), o.TTL, o.Alive) {
			return nil, nil
		}
		// Break the stale lease by renaming it away: only one breaker wins
		// the rename, so a loser can never remove a FRESH lease a faster
		// sibling just created at lockPath.
		breakPath := lockPath + ".breaking." + ticket
		if rerr := os.Rename(lockPath, breakPath); rerr == nil {
			_ = os.Remove(breakPath)
		} else if !errors.Is(rerr, os.ErrNotExist) {
			return nil, fmt.Errorf("shipwindow: break stale lease: %w", rerr)
		}
	}
	b, err := json.Marshal(record{
		OwnerPID:    o.OwnerPID,
		HeartbeatAt: o.Now().UTC().Format(time.RFC3339Nano),
		Token:       ticket,
	})
	if err != nil {
		return nil, fmt.Errorf("shipwindow: marshal: %w", err)
	}
	tmp := lockPath + ".tmp." + ticket
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("shipwindow: stage lease: %w", err)
	}
	// link is the mutual-exclusion point: atomic create-with-full-content
	// that fails EEXIST if any sibling won the race first.
	linkErr := os.Link(tmp, lockPath)
	_ = os.Remove(tmp)
	if linkErr != nil {
		if errors.Is(linkErr, os.ErrExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("shipwindow: create lease: %w", linkErr)
	}
	return &Lease{path: lockPath, token: ticket}, nil
}

// queueHead returns the first queued ticket whose owner is still alive. The
// caller's own ticket is never swept; a crashed waiter's ticket (dead pid) is
// removed so it cannot wedge FIFO forever. os.ReadDir returns names sorted,
// which IS the FIFO order (ticket names sort chronologically).
func queueHead(queueDir, own string, alive func(int) bool) (string, error) {
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		return "", fmt.Errorf("shipwindow: read queue: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == own {
			return own, nil
		}
		if pid := ticketPID(name); pid > 0 && !alive(pid) {
			_ = os.Remove(filepath.Join(queueDir, name))
			continue
		}
		return name, nil
	}
	return "", fmt.Errorf("shipwindow: own ticket %s missing from queue", own)
}

// ticketPID extracts the owner pid from a ticket name's final '-' segment;
// 0 when unparsable (such a ticket is never swept — fail-safe).
func ticketPID(name string) int {
	i := strings.LastIndexByte(name, '-')
	if i < 0 {
		return 0
	}
	pid, err := strconv.Atoi(name[i+1:])
	if err != nil {
		return 0
	}
	return pid
}

// readRecord loads the lock file. ok=false with nil error means absent;
// ok=true with non-nil error means present but unparsable (caller decides —
// tryAcquire treats it as stale crash debris).
func readRecord(path string) (record, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return record{}, false, nil
	}
	if err != nil {
		return record{}, false, err
	}
	var r record
	if jerr := json.Unmarshal(raw, &r); jerr != nil {
		return record{}, true, fmt.Errorf("shipwindow: parse %s: %w", path, jerr)
	}
	return r, true, nil
}

// Release frees the lease so the next queued waiter proceeds. Idempotent and
// break-safe: if the lease was already broken (and possibly re-acquired by
// another lane — token mismatch), Release is a no-op rather than removing a
// lease it no longer owns.
func (l *Lease) Release() error {
	rec, ok, err := readRecord(l.path)
	if err != nil && !ok {
		return fmt.Errorf("shipwindow: release read: %w", err)
	}
	if err != nil || !ok || rec.Token != l.token {
		return nil
	}
	if rerr := os.Remove(l.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("shipwindow: release: %w", rerr)
	}
	return nil
}

// pidAlive is the default holder-death oracle: signal 0 probes existence.
// EPERM means the pid exists but belongs to another user — alive.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	serr := p.Signal(syscall.Signal(0))
	return serr == nil || errors.Is(serr, syscall.EPERM)
}
