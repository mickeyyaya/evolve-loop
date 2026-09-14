package ciparitygate

import (
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// acquireLock takes a best-effort cross-lane exclusive lock so the retake
// runs serialized against other lanes' tier/test load, via the shared
// internal/adapters/flock primitive (which owns the runtime.KeepAlive raw-fd
// defense and in-process held-tracking — never re-derive raw flock here).
// Best-effort by design: a lock failure degrades to an unserialized retake
// (noted in the log and as TIER_LOCK_UNAVAILABLE), never blocks the gate.
// The lock lives under the cycle-shared lockRoot (project root first) so
// lanes contend on ONE file; the wait is bounded by TierLockWait on the
// injected clock, polling every 2 s.
func (g *Gates) acquireLock(at gateOrigin, req Request) (release func(), note string) {
	root := req.lockRoot()
	if root == "" {
		return g.lockDegraded(at, req, lockNoRoot, "lock unavailable: no project root")
	}
	path := filepath.Join(paths.EvolveDirOf(root), "locks", "integration-tier.lock")
	deadline := g.now().Add(g.timeouts.TierLockWait)
	for {
		rel, held, err := flock.TryLock(path)
		if err != nil {
			return g.lockDegraded(at, req, lockError, "lock unavailable: "+err.Error(), "path", path, "err", err.Error())
		}
		if !held {
			return rel, ""
		}
		if g.now().After(deadline) {
			return g.lockDegraded(at, req, lockTimeout, "lock wait timed out (retake unserialized)", "path", path, "wait", g.timeouts.TierLockWait.String())
		}
		g.sleep(2 * time.Second)
	}
}

// lockDegraded is the ONE writer of fields.reason: it records how the lock
// degraded (TIER_LOCK_UNAVAILABLE — silent before unit 14 beyond the log
// header) and returns the no-op release with the log-header note the
// original wrote — ", " + the reason text, byte for byte.
func (g *Gates) lockDegraded(at gateOrigin, req Request, reason lockReason, text string, kv ...string) (release func(), note string) {
	g.warn(at, req, CodeTierLockUnavailable, text, append([]string{"reason", string(reason)}, kv...)...)
	return func() {}, ", " + text
}
