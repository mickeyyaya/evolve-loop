// Package sessionrecord is the single source of truth for the per-run tmux
// session registry (CB.5, concurrency campaign): the on-disk JSONL schema one
// writer (the bridge driver, at session creation) and one reader (run
// teardown's swarm.ReapRunSessions) share. A leaf package — stdlib only — so
// both bridge and swarm can import it without bending the import graph
// (swarm → bridge → core forbids bridge → swarm).
//
// The registry lives INSIDE the run's workspace (.evolve/runs/cycle-<N>/),
// which is what makes registry-based reaping structurally run-isolated: a
// run's teardown can only ever see the sessions its own file records — there
// is no server-wide listing to mis-match against (the 2026-06-11 killer-B
// class).
package sessionrecord

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type appendFile interface {
	Write([]byte) (int, error)
	Close() error
}

var openAppendFileFn = func(path string) (appendFile, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// FileName is the registry's name inside a run workspace.
const FileName = "tmux-sessions.jsonl"

// RunScopeToken is the session-name run namespace: "r" + the first 8 ULID
// chars. The single source shared by the bridge's resolveSession (mints it
// into evolve-bridge-r<runid8>-… names) and the observer's run-scope
// assertion (CB.6: a probe that knows its run id refuses sessions without
// this token). Lives in this leaf so the observer adapter doesn't need the
// whole bridge package for one string rule.
func RunScopeToken(runID string) string {
	if len(runID) > 8 {
		runID = runID[:8]
	}
	return "r" + runID
}

// Record is one created tmux session. Append-only; a session is recorded at
// creation and never updated — liveness is tmux's truth, not the registry's.
type Record struct {
	Session   string `json:"session"`
	RunID     string `json:"run_id,omitempty"`
	Cycle     int    `json:"cycle,omitempty"`
	Agent     string `json:"agent,omitempty"`
	PID       int    `json:"pid,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// PathIn returns the registry path for a run workspace dir.
func PathIn(workspace string) string {
	return filepath.Join(workspace, FileName)
}

// Append appends one record. O_APPEND minimizes interleaving between
// concurrent same-run writers (parallel fan-out launches), but line
// atomicity for regular files is NOT guaranteed on every OS (macOS in
// particular) — ReadAll's malformed-line skip is the durability defense
// for the rare race, degrading one record to leak-on-crash rather than
// ever corrupting a neighbor's.
func Append(path string, r Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("sessionrecord: marshal: %w", err)
	}
	f, err := openAppendFileFn(path)
	if err != nil {
		return fmt.Errorf("sessionrecord: open: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("sessionrecord: append: %w", err)
	}
	// A writer's Close error can mean the line never hit the disk — surface it.
	if err := f.Close(); err != nil {
		return fmt.Errorf("sessionrecord: close: %w", err)
	}
	return nil
}

// ReadAll returns every record in the registry. A missing file is an empty
// registry (a run that launched no sessions), not an error. Malformed lines
// are skipped — the reaper must still reap the well-formed remainder of a
// registry a crash half-wrote.
func ReadAll(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sessionrecord: open: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only handle; Close error carries no signal
	var out []Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	if err := sc.Err(); err != nil {
		return out, fmt.Errorf("sessionrecord: scan: %w", err)
	}
	return out, nil
}
