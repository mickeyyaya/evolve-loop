package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// SessionStatus is a worker session's lifecycle state in the registry and manifest.
type SessionStatus string

const (
	// StatusLive marks a dispatched worker whose tmux session should exist.
	StatusLive SessionStatus = "live"
	// StatusReaped marks a worker torn down cleanly or by the reaper.
	StatusReaped SessionStatus = "reaped"
)

// SessionHandle identifies one worker session, both in memory and in the crash-safe manifest.
type SessionHandle struct {
	WorkerID    string        `json:"worker_id"`
	Agent       string        `json:"agent"`        // the tmux/inbox key
	TmuxSession string        `json:"tmux_session"` // empty for headless workers
	PGID        int           `json:"pgid"`         // 0 = unknown
	Worktree    string        `json:"worktree"`     // writers only
	Branch      string        `json:"branch"`       // writers only
	StartedAt   string        `json:"started_at"`   // RFC3339, stamped by the caller
	Status      SessionStatus `json:"status"`
}

// manifest is rewritten on every mutation so `evolve swarm reap` can find orphans after a SIGKILL.
type manifest struct {
	Cycle    int             `json:"cycle"`
	Phase    string          `json:"phase"`
	PID      int             `json:"pid"` // the orchestrator process that owns these sessions
	Updated  string          `json:"updated,omitempty"`
	Sessions []SessionHandle `json:"sessions"`
}

// SessionRegistry tracks worker sessions in memory, mirrors them to the on-disk manifest, and is safe for concurrent use.
type SessionRegistry struct {
	mu           sync.Mutex
	manifestPath string
	m            manifest
}

// NewSessionRegistry returns a registry backed by manifestPath, or kept in memory only when the path is empty.
func NewSessionRegistry(manifestPath string, cycle int, phase string, pid int) *SessionRegistry {
	return &SessionRegistry{
		manifestPath: manifestPath,
		m:            manifest{Cycle: cycle, Phase: phase, PID: pid, Sessions: []SessionHandle{}},
	}
}

// Register records h as Live and persists; re-registering a WorkerID replaces the prior entry.
func (r *SessionRegistry) Register(h SessionHandle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	h.Status = StatusLive
	prev := r.snapshotSessionsLocked()
	r.upsertLocked(h)
	return r.persistOrRollbackLocked(prev)
}

// MarkReaped flips a session to Reaped and persists; an unknown WorkerID is a no-op.
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

// snapshotSessionsLocked relies on SessionHandle holding only value fields, so a slice copy is a full snapshot.
func (r *SessionRegistry) snapshotSessionsLocked() []SessionHandle {
	prev := make([]SessionHandle, len(r.m.Sessions))
	copy(prev, r.m.Sessions)
	return prev
}

// persistOrRollbackLocked undoes an in-memory mutation the manifest never recorded, since the manifest is the reaper's truth.
func (r *SessionRegistry) persistOrRollbackLocked(prev []SessionHandle) error {
	if err := r.persistLocked(); err != nil {
		r.m.Sessions = prev
		return err
	}
	return nil
}

// Snapshot returns a copy of the sessions, sorted by WorkerID.
func (r *SessionRegistry) Snapshot() []SessionHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SessionHandle, len(r.m.Sessions))
	copy(out, r.m.Sessions)
	sort.Slice(out, func(i, j int) bool { return out[i].WorkerID < out[j].WorkerID })
	return out
}

// Live returns the sessions still marked Live: the teardown work list.
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

func (r *SessionRegistry) persistLocked() error {
	if r.manifestPath == "" {
		return nil
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

// LoadManifest reads a persisted manifest; a missing file yields an empty result, not an error.
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
