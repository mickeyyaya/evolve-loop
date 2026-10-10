package gc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

var runMarkers = append([]string{
	"run.json",
	"phase-timing.json",
	"interaction-summary.json",
}, phasecontract.RequiredArtifacts()...)

type DiscoverOptions struct {
	Now        func() time.Time
	LeaseTTL   time.Duration
	LedgerRefs []string
}

func Discover(evolveDir string, o DiscoverOptions) ([]RunDir, error) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	refs := make(map[string]bool, len(o.LedgerRefs))
	for _, r := range o.LedgerRefs {
		refs[filepath.Clean(r)] = true
	}
	currentWS, err := currentWorkspace(evolveDir)
	if err != nil {
		return nil, err
	}

	runsDir := filepath.Join(evolveDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gc: read runs dir: %w", err)
	}
	var out []RunDir
	for _, e := range entries {
		dir := filepath.Join(runsDir, e.Name())
		info, ok := runDirInfo(dir, e)
		if !ok {
			continue
		}
		if !hasRunMarker(dir) && !refs[dir] && !gcpolicy.IsPollutedArchive(e.Name()) {
			continue
		}
		out = append(out, RunDir{
			Path:    dir,
			ModTime: info.ModTime(),
			Live:    dir == currentWS || leaseFresh(dir, now(), o.LeaseTTL),
		})
	}
	return out, nil
}

func runDirInfo(dir string, e os.DirEntry) (os.FileInfo, bool) {
	if e.IsDir() {
		info, err := e.Info()
		return info, err == nil
	}
	st, serr := os.Stat(dir)
	if serr != nil || !st.IsDir() {
		return nil, false
	}
	return st, true
}

func hasRunMarker(dir string) bool {
	for _, m := range runMarkers {
		if info, err := os.Stat(filepath.Join(dir, m)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func currentWorkspace(evolveDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(evolveDir, "cycle-state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("gc: currentWorkspace: %w", err)
	}
	var cs struct {
		CycleID       int    `json:"cycle_id"`
		WorkspacePath string `json:"workspace_path"`
	}
	if err := json.Unmarshal(raw, &cs); err != nil {
		return "", fmt.Errorf("gc: currentWorkspace: parse cycle-state.json: %w", err)
	}
	if cs.CycleID == 0 {
		return "", nil
	}
	if !filepath.IsAbs(cs.WorkspacePath) {
		return "", fmt.Errorf("gc: cycle-state.json has cycle_id=%d but workspace_path %q is not absolute", cs.CycleID, cs.WorkspacePath)
	}
	return filepath.Clean(cs.WorkspacePath), nil
}

func leaseFresh(dir string, now time.Time, ttl time.Duration) bool {
	l, ok, err := runlease.Read(dir)
	if err != nil || !ok {
		return false
	}
	return runlease.Fresh(l, now, ttl)
}
