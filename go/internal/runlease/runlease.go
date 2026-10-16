// Package runlease reads and writes the per-run .lease heartbeat file. A missing
// lease is no lease, and a corrupt lease is an error. See
// docs/architecture/packages/internal-runlease.md.
package runlease

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type tempFile interface {
	Name() string
	Write([]byte) (int, error)
	Close() error
}

type writer struct {
	createTemp func(dir, pattern string) (tempFile, error)
	rename     func(oldpath, newpath string) error
}

func newWriter() writer {
	return writer{
		createTemp: func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) },
		rename:     os.Rename,
	}
}

const FileName = ".lease"

const DefaultTTL = 10 * time.Minute

type Lease struct {
	RunID       string `json:"run_id,omitempty"`
	OwnerPID    int    `json:"owner_pid,omitempty"`
	HeartbeatAt string `json:"heartbeat_at"`
}

func PathIn(runDir string) string {
	return filepath.Join(runDir, FileName)
}

func Write(runDir string, l Lease, now time.Time) error {
	return newWriter().write(runDir, l, now)
}

func (w writer) write(runDir string, l Lease, now time.Time) error {
	l.HeartbeatAt = now.UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("runlease: marshal: %w", err)
	}
	tmp, err := w.createTemp(runDir, FileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("runlease: tmp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("runlease: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("runlease: close: %w", err)
	}
	if err := w.rename(tmpPath, PathIn(runDir)); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("runlease: rename: %w", err)
	}
	return nil
}

func Read(runDir string) (l Lease, ok bool, err error) {
	raw, err := os.ReadFile(PathIn(runDir))
	if errors.Is(err, os.ErrNotExist) {
		return Lease{}, false, nil
	}
	if err != nil {
		return Lease{}, false, fmt.Errorf("runlease: read: %w", err)
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return Lease{}, false, fmt.Errorf("runlease: parse %s: %w", PathIn(runDir), err)
	}
	return l, true, nil
}

func Fresh(l Lease, now time.Time, ttl time.Duration) bool {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	hb, err := time.Parse(time.RFC3339Nano, l.HeartbeatAt)
	if err != nil {
		return false
	}
	return now.Sub(hb) < ttl
}

func OwnerLive(l Lease, now time.Time, ttl time.Duration, alive func(int) bool) bool {
	if !Fresh(l, now, ttl) {
		return false
	}
	if l.OwnerPID == 0 || alive == nil {
		return true
	}
	return alive(l.OwnerPID)
}

type LiveRun struct {
	Dir   string
	Lease Lease
}

func LiveOwner(runDir string, now time.Time) (Lease, bool) {
	l, ok, err := Read(runDir)
	if err != nil || !ok || !OwnerLive(l, now, DefaultTTL, PIDAlive) {
		return Lease{}, false
	}
	return l, true
}

func LiveRuns(runsDir string, now time.Time) []LiveRun {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}
	var live []LiveRun
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(runsDir, e.Name())
		if l, ok := LiveOwner(dir, now); ok {
			live = append(live, LiveRun{Dir: dir, Lease: l})
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].Dir < live[j].Dir })
	return live
}
