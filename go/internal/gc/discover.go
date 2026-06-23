package gc

// discover.go — layout-agnostic, lease-aware run-dir discovery (L3.2).
//
// The .evolve/runs directory is NOT a clean namespace: the real tree mixes
// cycle-N dirs with loose log files, reset-sealed snapshots and one-off
// manual dirs (342 entries / 428MB at the time of writing). Discovery
// therefore NEVER parses names. A direct child of runs/ qualifies as a run
// dir only by EVIDENCE:
//   - it is a directory, AND
//   - it contains a known run-manifest marker (run.json — the CB.4 guard
//     mirror — or a phase artifact), OR its absolute path is in the
//     caller-supplied ledger-reference set.
//
// Anything that does not qualify is not returned, so the retention engine
// can never touch it — unknown layouts are left alone by construction.
//
// Liveness (RunDir.Live) — either signal suffices:
//   - run-state: the host-global cycle-state.json names this dir as the
//     in-flight run's workspace (non-terminal run state), or
//   - a fresh .lease heartbeat (internal/runlease — the shared contract the
//     CE.3 fleet scheduler writes; in fleet mode M runs are in flight but
//     the global cycle-state can only name one).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/runlease"
)

// runMarkers are the files whose presence identifies a directory as a run
// workspace. run.json is the canonical post-CB.4 marker (the WriteCycleState
// guard mirror); the rest cover pre-CB.4 runs that only hold phase
// artifacts.
var runMarkers = []string{
	"run.json",
	"phase-timing.json",
	"interaction-summary.json",
	"scout-report.md",
	"build-report.md",
	"audit-report.md",
}

// DiscoverOptions tunes Discover.
type DiscoverOptions struct {
	Now func() time.Time
	// LeaseTTL is the .lease freshness window; <=0 means runlease.DefaultTTL.
	LeaseTTL time.Duration
	// LedgerRefs are absolute run-dir paths referenced by ledger entries
	// (artifact_path parents). Optional second evidence source: a dir listed
	// here qualifies even without a marker file.
	LedgerRefs []string
}

// Discover walks <evolveDir>/runs and returns the evidenced run dirs with
// their liveness classification. A missing runs/ dir yields an empty list.
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
		// Fail closed: with the run-state liveness signal unreadable, a live
		// run without a lease could be misclassified as dead. No discovery →
		// no collection this pass.
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
		var info os.FileInfo
		if e.IsDir() {
			info, err = e.Info()
			if err != nil {
				continue
			}
		} else {
			// Possibly a symlink to a run dir (an operator alias for a
			// relocated run). Stat follows it; loose files and dangling
			// links at runs/ root are not runs. Skipping symlinked runs
			// here would make them INVISIBLE — absent from discovery and
			// therefore from the keep_full/liveness protections.
			st, serr := os.Stat(dir)
			if serr != nil || !st.IsDir() {
				continue
			}
			info = st
		}
		if !hasRunMarker(dir) && !refs[dir] {
			continue // no evidence — leave it alone
		}
		out = append(out, RunDir{
			Path:    dir,
			ModTime: info.ModTime(),
			Live:    dir == currentWS || leaseFresh(dir, now(), o.LeaseTTL),
		})
	}
	return out, nil
}

func hasRunMarker(dir string) bool {
	for _, m := range runMarkers {
		if info, err := os.Stat(filepath.Join(dir, m)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// currentWorkspace reads the in-flight run's workspace path from the
// host-global cycle-state.json. The two fields read here are a documented
// subset of core.CycleState (the schema's single source); gc stays a
// stdlib-only leaf by not importing core. An ABSENT file or cycle_id==0 is
// the normal idle state (no current workspace); an unreadable or unparsable
// file is an ERROR — Discover fails closed on it, because a live run whose
// lease has not been written yet would otherwise be misclassified as dead.
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
		return "", nil // terminal / no cycle in flight
	}
	// A cycle in flight MUST name an absolute workspace; anything else is a
	// malformed state whose liveness signal we cannot trust — fail closed.
	if !filepath.IsAbs(cs.WorkspacePath) {
		return "", fmt.Errorf("gc: cycle-state.json has cycle_id=%d but workspace_path %q is not absolute", cs.CycleID, cs.WorkspacePath)
	}
	return filepath.Clean(cs.WorkspacePath), nil
}

// leaseFresh reports a fresh .lease heartbeat in dir. Parse errors count as
// "no lease" (never more collectable than no file), matching the runlease
// contract.
func leaseFresh(dir string, now time.Time, ttl time.Duration) bool {
	l, ok, err := runlease.Read(dir)
	if err != nil || !ok {
		return false
	}
	return runlease.Fresh(l, now, ttl)
}
