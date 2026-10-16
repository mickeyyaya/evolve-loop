// Package cliadmit provides cross-process admission control for shared LLM
// CLIs.
package cliadmit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const DefaultTTL = 30 * time.Second

type holder struct {
	PID       int       `json:"pid"`
	Seq       int64     `json:"seq"`
	Heartbeat time.Time `json:"heartbeat"`
}

var (
	nowFn       = time.Now
	slotsPathFn = defaultSlotsPath
)

var acquireSeq int64

func defaultSlotsPath(cli string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "evolve", "cli-"+cli+".slots")
}

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
			fresh := holders[:0]
			for _, h := range holders {
				if now.Sub(h.Heartbeat) <= ttl {
					fresh = append(fresh, h)
				}
			}
			if len(fresh) >= max {
				return nil
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
		return nil, nil
	}
	return holders, nil
}
