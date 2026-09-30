package ciparitygate

import (
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

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

func (g *Gates) lockDegraded(at gateOrigin, req Request, reason lockReason, text string, kv ...string) (release func(), note string) {
	g.warn(at, req, CodeTierLockUnavailable, text, append([]string{"reason", string(reason)}, kv...)...)
	return func() {}, ", " + text
}
