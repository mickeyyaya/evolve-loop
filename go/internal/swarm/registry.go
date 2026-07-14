package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// SessionStatus is a worker session's lifecycle state in the registry/manifest.
type SessionStatus string

const (
	// StatusLive — the worker is dispatched and its tmux session should exist.
	StatusLive SessionStatus = "live"
	// StatusReaped — the worker was torn down cleanly (or by the reaper).
	StatusReaped SessionStatus = "reaped"
)

// SessionHandle identifies one dispatched worker session for tracking and
// teardown. It is the unit recorded both in memory and in the crash-safe
// manifest, so the reaper can kill an orphan after a hard parent crash.
type SessionHandle struct {
	WorkerID    string        `json:"worker_id"`
	Agent       string        `json:"agent"`        // "<phase>-w<i>" — the tmux/inbox key
	TmuxSession string        `json:"tmux_session"` // resolveSession name (may be empty for headless)
	PGID        int           `json:"pgid"`         // process-group id for group-kill (0 = unknown)
	Worktree    string        `json:"worktree"`     // writers: the per-worker worktree path
	Branch      string        `json:"branch"`       // writers: cycle-<N>-w<i>
	StartedAt   string        `json:"started_at"`   // RFC3339 (caller stamps; pure pkg avoids time.Now)
	Status      SessionStatus `json:"status"`
}

// manifest is the on-disk shape persisted atomically on every mutation. It
// survives a hard SIGKILL of the orchestrator so `evolve swarm reap` can find
// and kill orphaned sessions.
type manifest struct {
	Cycle    int             `json:"cycle"`
	Phase    string          `json:"phase"`
	PID      int             `json:"pid"` // the orchestrator process that owns these sessions
	Updated  string          `json:"updated,omitempty"`
	Sessions []SessionHandle `json:"sessions"`
}

// SessionRegistry tracks live worker sessions in memory and mirrors them to a
// crash-safe on-disk manifest. It is the single source of truth for teardown:
// the dispatcher Registers before launch and Unregisters after reap; the reaper
// reads the manifest to clean orphans.
//
// Safe for concurrent use — the dispatcher registers/unregisters from worker
// goroutines. Every mutation re-persists the whole manifest atomically
// (tmp+rename); the session count per swarm is small (single digits) so
// rewriting the file each time is simpler and safer than append-and-compact.
type SessionRegistry struct {
	mu           sync.Mutex
	manifestPath string
	m            manifest
}

// NewSessionRegistry creates a registry backed by manifestPath. cycle/phase/pid
// are recorded in the manifest header so the reaper knows which orchestrator
// owned the sessions. The manifest directory is created on first Persist.
func NewSessionRegistry(manifestPath string, cycle int, phase string, pid int) *SessionRegistry {
	return &SessionRegistry{
		manifestPath: manifestPath,
		m:            manifest{Cycle: cycle, Phase: phase, PID: pid, Sessions: []SessionHandle{}},
	}
}

// Register records a newly launched session (status forced to Live) and
// persists. Re-registering the same WorkerID replaces the prior entry
// (idempotent across a retry).
func (r *SessionRegistry) Register(h SessionHandle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	h.Status = StatusLive
	prev := r.snapshotSessionsLocked()
	r.upsertLocked(h)
	return r.persistOrRollbackLocked(prev)
}

// MarkReaped flips a session to Reaped and persists. Unknown WorkerID is a
// no-op (the reaper may race the dispatcher's own teardown).
func (r *SessionRegistry) MarkReaped(workerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := r.snapshotSessionsLocked()
	for i := range r.m.Sessions {
		if r.m.Sessions[i].WorkerID == workerID {
			r.m.Sessions[i].Status = StatusReaped
		}
	}
	return r.persistOrRollbackLocked(prev)
}

// snapshotSessionsLocked returns a value copy of the current sessions, taken
// BEFORE a mutation so it can be restored if the durable write fails.
// SessionHandle is all value fields, so a slice copy is a full snapshot.
func (r *SessionRegistry) snapshotSessionsLocked() []SessionHandle {
	prev := make([]SessionHandle, len(r.m.Sessions))
	copy(prev, r.m.Sessions)
	return prev
}

// persistOrRollbackLocked persists the manifest and, on failure, restores the
// sessions slice to the pre-mutation snapshot — the manifest is the reaper's
// source of truth, so an in-memory mutation that was never durably written
// must not silently diverge from disk.
func (r *SessionRegistry) persistOrRollbackLocked(prev []SessionHandle) error {
	if err := r.persistLocked(); err != nil {
		r.m.Sessions = prev
		return err
	}
	return nil
}

// Snapshot returns a copy of the current sessions (safe to range without the
// lock). Deterministic order (by WorkerID) for stable logs/tests.
func (r *SessionRegistry) Snapshot() []SessionHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SessionHandle, len(r.m.Sessions))
	copy(out, r.m.Sessions)
	sort.Slice(out, func(i, j int) bool { return out[i].WorkerID < out[j].WorkerID })
	return out
}

// Live returns the sessions still marked Live (the teardown work-list).
func (r *SessionRegistry) Live() []SessionHandle {
	var live []SessionHandle
	for _, h := range r.Snapshot() {
		if h.Status == StatusLive {
			live = append(live, h)
		}
	}
	return live
}

func (r *SessionRegistry) upsertLocked(h SessionHandle) {
	for i := range r.m.Sessions {
		if r.m.Sessions[i].WorkerID == h.WorkerID {
			r.m.Sessions[i] = h
			return
		}
	}
	r.m.Sessions = append(r.m.Sessions, h)
}

// persistLocked writes the manifest atomically (tmp + rename), mirroring the
// crash-safe pattern used elsewhere in core (reset.go writeJSONMapFileAtomic).
// Updated is left to the caller's stamping discipline — the pure package does
// not call time.Now; callers that want a timestamp set it on the handle.
func (r *SessionRegistry) persistLocked() error {
	if r.manifestPath == "" {
		return nil // in-memory-only mode (tests that don't exercise persistence)
	}
	if err := os.MkdirAll(filepath.Dir(r.manifestPath), 0o755); err != nil {
		return fmt.Errorf("swarm manifest dir: %w", err)
	}
	data, err := json.MarshalIndent(r.m, "", "  ")
	if err != nil {
		return fmt.Errorf("swarm manifest marshal: %w", err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", r.manifestPath, os.Getpid())
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("swarm manifest write: %w", err)
	}
	if err := os.Rename(tmp, r.manifestPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("swarm manifest rename: %w", err)
	}
	return nil
}

// LoadManifest reads a persisted manifest (used by the reaper to find orphans
// after a parent crash). A missing file is not an error — it returns an empty
// manifest so the reaper is a safe no-op.
func LoadManifest(path string) (cycle int, phase string, pid int, sessions []SessionHandle, err error) {
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return 0, "", 0, nil, nil
		}
		return 0, "", 0, nil, fmt.Errorf("read swarm manifest: %w", rerr)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return 0, "", 0, nil, fmt.Errorf("parse swarm manifest: %w", err)
	}
	return m.Cycle, m.Phase, m.PID, m.Sessions, nil
}
