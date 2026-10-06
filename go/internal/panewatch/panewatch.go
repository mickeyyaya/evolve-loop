// Package panewatch publishes each tmux phase pane's liveness snapshot for observers and operators.
// See docs/architecture/packages/internal-panewatch.md.
package panewatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const fileSuffix = "-pane-watch.json"

type Snapshot struct {
	Session      string    `json:"session"`
	Socket       string    `json:"socket,omitempty"`
	CLI          string    `json:"cli"`
	Agent        string    `json:"agent"`
	Cycle        int       `json:"cycle"`
	RunID        string    `json:"run_id,omitempty"`
	Model        string    `json:"model,omitempty"`
	ModelLabel   string    `json:"model_label,omitempty"`
	Busy         bool      `json:"busy"`
	ProgressHash string    `json:"progress_hash"`
	ProgressAt   time.Time `json:"progress_at"`
	TokenLine    string    `json:"token_line,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	WriterPID    int       `json:"writer_pid"`
}

func (s Snapshot) WriterAlive(alive func(int) bool) bool {
	return s.WriterPID > 0 && alive != nil && alive(s.WriterPID)
}

func ReadLive(workspace, agent string, alive func(int) bool) (Snapshot, bool, error) {
	s, ok, err := Read(workspace, agent)
	if err != nil || !ok || !s.WriterAlive(alive) {
		return Snapshot{}, false, err
	}
	return s, true, nil
}

func Path(workspace, agent string) string {
	return filepath.Join(workspace, agent+fileSuffix)
}

func Write(workspace string, s Snapshot) error {
	return atomicwrite.JSON(Path(workspace, s.Agent), s)
}

func Read(workspace, agent string) (Snapshot, bool, error) {
	return readFile(Path(workspace, agent))
}

func ReadAll(workspace string) ([]Snapshot, error) {
	paths, err := filepath.Glob(filepath.Join(workspace, "*"+fileSuffix))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []Snapshot
	var errs []error
	for _, path := range paths {
		s, ok, err := readFile(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ok {
			out = append(out, s)
		}
	}
	return out, errors.Join(errs...)
}

func readFile(path string) (Snapshot, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, false, fmt.Errorf("pane watch %s: %w", strings.TrimSuffix(filepath.Base(path), fileSuffix), err)
	}
	return s, true, nil
}

func Remove(workspace, agent string) error {
	err := os.Remove(Path(workspace, agent))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
