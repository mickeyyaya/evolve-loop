package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type SessionStatus string

const (
	StatusLive   SessionStatus = "live"
	StatusReaped SessionStatus = "reaped"
)

type SessionHandle struct {
	WorkerID    string        `json:"worker_id"`
	Agent       string        `json:"agent"`
	TmuxSession string        `json:"tmux_session"`
	PGID        int           `json:"pgid"`
	Worktree    string        `json:"worktree"`
	Branch      string        `json:"branch"`
	StartedAt   string        `json:"started_at"`
	Status      SessionStatus `json:"status"`
}

type manifest struct {
	Cycle    int             `json:"cycle"`
	Phase    string          `json:"phase"`
	PID      int             `json:"pid"`
	Updated  string          `json:"updated,omitempty"`
	Sessions []SessionHandle `json:"sessions"`
}

type SessionRegistry struct {
	mu           sync.Mutex
	manifestPath string
	m            manifest
}

func NewSessionRegistry(manifestPath string, cycle int, phase string, pid int) *SessionRegistry {
	return &SessionRegistry{
		manifestPath: manifestPath,
		m:            manifest{Cycle: cycle, Phase: phase, PID: pid, Sessions: []SessionHandle{}},
	}
}

func (r *SessionRegistry) Register(h SessionHandle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	h.Status = StatusLive
	prev := r.snapshotSessionsLocked()
	r.upsertLocked(h)
	return r.persistOrRollbackLocked(prev)
}

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

func (r *SessionRegistry) snapshotSessionsLocked() []SessionHandle {
	prev := make([]SessionHandle, len(r.m.Sessions))
	copy(prev, r.m.Sessions)
	return prev
}

func (r *SessionRegistry) persistOrRollbackLocked(prev []SessionHandle) error {
	if err := r.persistLocked(); err != nil {
		r.m.Sessions = prev
		return err
	}
	return nil
}

func (r *SessionRegistry) Snapshot() []SessionHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SessionHandle, len(r.m.Sessions))
	copy(out, r.m.Sessions)
	sort.Slice(out, func(i, j int) bool { return out[i].WorkerID < out[j].WorkerID })
	return out
}

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
