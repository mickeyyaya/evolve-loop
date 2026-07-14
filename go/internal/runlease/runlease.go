// Package runlease is the single source of truth for the per-run .lease
// heartbeat file (L3.2, concurrency campaign) — the ONE shared interface
// between the retention engine (internal/gc reads leases to classify a run
// dir as live) and the fleet scheduler (CE.3 writes and refreshes them for
// every run it supervises). A leaf package — stdlib only — so both sides can
// import it without bending the import graph.
//
// Contract:
//   - The file lives at <run-dir>/.lease and holds one JSON object.
//   - The writer refreshes HeartbeatAt at least every DefaultTTL/2.
//   - A reader treats the run as LIVE while now - HeartbeatAt < ttl.
//   - A missing or unparsable lease is simply "no lease" — liveness then
//     falls back to the run-state signal (gc's workspaceIsCurrent). Parse
//     errors never make a run MORE collectable than no file would.
//   - Writes are atomic (tmp + rename) so a reader never sees a torn file.
//   - WRITE-ORDERING INVARIANT (CE.3): a scheduler MUST write the lease
//     BEFORE transitioning its run to a non-terminal state, so a gc pass
//     that snapshots liveness never races a run into existence unleased.
//     gc additionally re-checks leases at Apply time (defense in depth),
//     but that re-check only narrows the window — the ordering closes it.
package runlease

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type tempFile interface {
	Name() string
	Write([]byte) (int, error)
	Close() error
}

// writer holds the filesystem seams for an atomic lease write. The seams are
// per-INSTANCE, not package globals: a test injects a failing createTemp/rename
// on its OWN writer without mutating shared state, so write tests parallelize
// safely (the old package-var seams raced any sibling test the moment someone
// added t.Parallel()). newWriter supplies the real os-backed seams.
type writer struct {
	createTemp func(dir, pattern string) (tempFile, error)
	rename     func(oldpath, newpath string) error
}

// newWriter returns a writer wired to the real filesystem.
func newWriter() writer {
	return writer{
		createTemp: func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) },
		rename:     os.Rename,
	}
}

// FileName is the lease file's name inside a run directory.
const FileName = ".lease"

// DefaultTTL is the freshness window: a heartbeat older than this no longer
// proves liveness. Writers refresh at least twice per window.
const DefaultTTL = 10 * time.Minute

// Lease is the on-disk schema.
type Lease struct {
	// RunID is the run's ULID (CA.5).
	RunID string `json:"run_id,omitempty"`
	// OwnerPID is informational (forensics: which scheduler held the lease).
	// Readers must NOT treat a matching live PID as proof of run liveness —
	// freshness of HeartbeatAt is the only liveness signal.
	OwnerPID int `json:"owner_pid,omitempty"`
	// HeartbeatAt is RFC3339Nano UTC. Stamped by Write; callers never set it.
	HeartbeatAt string `json:"heartbeat_at"`
}

// PathIn returns the lease path for a run directory.
func PathIn(runDir string) string {
	return filepath.Join(runDir, FileName)
}

// Write atomically writes (or refreshes) the lease in runDir, stamping
// HeartbeatAt with now. It is the package-level convenience over a real-seam
// writer — the single production caller (core/runlease_hook.go) needs no seams.
func Write(runDir string, l Lease, now time.Time) error {
	return newWriter().write(runDir, l, now)
}

// write is the seam-driven implementation behind Write.
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

// Read loads the lease from runDir. ok=false (no error) when the file is
// absent. A present-but-unparsable file returns an error so the caller can
// decide — gc treats it as "no lease" but logs the anomaly.
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

// Fresh reports whether the lease's heartbeat is within ttl of now.
// ttl <= 0 uses DefaultTTL. An unparsable timestamp is never fresh.
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

// OwnerLive reports whether the lease's owner is genuinely live: the heartbeat
// is Fresh AND the owning process is still running. It adds a pid-liveness
// probe on top of Fresh so a crashed owner whose heartbeat has not yet aged
// past the TTL (the common 2-6min post-crash window) no longer reads as "live"
// and no longer blocks a seal — while a running owner (fresh heartbeat, alive
// pid) still does, unchanged.
//
// The probe is consulted ONLY as an additional way to prove DEADness; a stale
// heartbeat is never live regardless of pid state (guards pid reuse). When
// alive is nil or OwnerPID is 0 (a lease written before pid tracking, or a
// caller that never sets it), OwnerLive falls back to freshness-only — the
// probe is not called — preserving the pre-pid behavior for un-migrated callers.
func OwnerLive(l Lease, now time.Time, ttl time.Duration, alive func(int) bool) bool {
	if !Fresh(l, now, ttl) {
		return false
	}
	if l.OwnerPID == 0 || alive == nil {
		return true
	}
	return alive(l.OwnerPID)
}
