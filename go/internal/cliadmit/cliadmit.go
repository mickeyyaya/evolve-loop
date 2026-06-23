// Package cliadmit provides cross-process admission control for shared LLM
// CLIs. It enforces a per-CLI concurrent-holder cap across M independent
// evolve loop processes via a flock'd holder-set JSON under
// $XDG_RUNTIME_DIR/evolve/cli-<name>.slots.
//
// Safe default: max<=0 is unbounded — byte-identical to today. The
// EVOLVE_CLI_MAX_CONCURRENT_<CLI> dial is opt-in.
package cliadmit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolveloop/go/internal/atomicwrite"
)

// DefaultTTL is the maximum age of a holder's heartbeat before it is
// considered stale (crashed process). Holders older than this are pruned
// on the next Acquire, making the cap self-healing.
const DefaultTTL = 30 * time.Second

// holder is one entry in the per-CLI slots file.
type holder struct {
	PID       int       `json:"pid"`
	Seq       int64     `json:"seq"`       // unique per Acquire call, intra-process
	Heartbeat time.Time `json:"heartbeat"` // set at Acquire; used for stale detection
}

// Package-level seams for testing.
var (
	// nowFn returns the current time; injectable for stale-prune tests.
	nowFn = time.Now
	// slotsPathFn returns the slots file path for a given CLI name.
	slotsPathFn = defaultSlotsPath
)

// acquireSeq is an atomic counter that gives each Acquire call a unique
// sequence number within the process, so the same PID can hold multiple
// distinct slots without collision.
var acquireSeq int64

// defaultSlotsPath returns the canonical path for the holder-set JSON.
// Prefers $XDG_RUNTIME_DIR; falls back to os.TempDir().
func defaultSlotsPath(cli string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "evolve", "cli-"+cli+".slots")
}

// Acquire blocks until a slot is available in the per-CLI admission set.
// max<=0 is unbounded and returns immediately (safe default — no slots file
// created, no lock acquired). On success it returns a release function the
// caller MUST call exactly once (defer it). On error it returns a no-op
// release so the caller can always defer without a nil check; the error
// signals "proceed uncapped + WARN".
func Acquire(ctx context.Context, cli string, max int, ttl time.Duration) (func(), error) {
	if max <= 0 {
		return func() {}, nil
	}

	path := slotsPathFn(cli)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return func() {}, fmt.Errorf("cliadmit: mkdir: %w", err)
	}

	pid := os.Getpid()
	seq := atomic.AddInt64(&acquireSeq, 1)
	backoff := 50 * time.Millisecond

	for {
		var admitted bool
		lockErr := flock.WithPathLock(path, func() error {
			now := nowFn()
			holders, err := readHolders(path)
			if err != nil {
				return err
			}
			// Prune stale holders (self-healing for crashed processes).
			fresh := holders[:0]
			for _, h := range holders {
				if now.Sub(h.Heartbeat) <= ttl {
					fresh = append(fresh, h)
				}
			}
			if len(fresh) >= max {
				return nil // at capacity; caller will retry
			}
			fresh = append(fresh, holder{PID: pid, Seq: seq, Heartbeat: now})
			admitted = true
			return atomicwrite.JSON(path, fresh)
		})
		if lockErr != nil {
			return func() {}, lockErr
		}
		if admitted {
			break
		}

		select {
		case <-ctx.Done():
			return func() {}, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 200*time.Millisecond {
			backoff *= 2
		}
	}

	release := func() {
		_ = flock.WithPathLock(path, func() error {
			holders, err := readHolders(path)
			if err != nil {
				return err
			}
			out := make([]holder, 0, len(holders))
			for _, h := range holders {
				if h.PID != pid || h.Seq != seq {
					out = append(out, h)
				}
			}
			return atomicwrite.JSON(path, out)
		})
	}
	return release, nil
}

// readHolders reads the holder slice from path. Returns nil (not an error)
// when the file is absent, empty, or unparseable — those are all treated as
// an empty set (self-healing).
func readHolders(path string) ([]holder, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) || (err == nil && len(data) == 0) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cliadmit: read %s: %w", path, err)
	}
	var holders []holder
	if jsonErr := json.Unmarshal(data, &holders); jsonErr != nil {
		// Malformed file is treated as empty so a corrupt slots file does
		// not permanently block new admits.
		return nil, nil
	}
	return holders, nil
}
