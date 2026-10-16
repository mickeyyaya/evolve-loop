// Package sessionrecord defines the per-run tmux session registry that the bridge writes and the run teardown reads.
// See docs/architecture/packages/internal-sessionrecord.md.
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

const FileName = "tmux-sessions.jsonl"

const ReapedSuffix = ".reaped"

func RunScopeToken(runID string) string {
	if len(runID) > 8 {
		runID = runID[:8]
	}
	return "r" + runID
}

type Record struct {
	Session   string `json:"session"`
	RunID     string `json:"run_id,omitempty"`
	Cycle     int    `json:"cycle,omitempty"`
	Agent     string `json:"agent,omitempty"`
	PID       int    `json:"pid,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func PathIn(workspace string) string {
	return filepath.Join(workspace, FileName)
}

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
	if err := f.Close(); err != nil {
		return fmt.Errorf("sessionrecord: close: %w", err)
	}
	return nil
}

func ReadAll(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sessionrecord: open: %w", err)
	}
	defer func() { _ = f.Close() }()
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

func ReadAllResolving(path string) ([]Record, error) {
	live, err := ReadAll(path)
	if err != nil {
		return nil, err
	}
	tomb, err := ReadAll(path + ReapedSuffix)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(live)+len(tomb))
	out := make([]Record, 0, len(live)+len(tomb))
	for _, group := range [][]Record{live, tomb} {
		for _, r := range group {
			if _, dup := seen[r.Session]; dup {
				continue
			}
			seen[r.Session] = struct{}{}
			out = append(out, r)
		}
	}
	return out, nil
}
