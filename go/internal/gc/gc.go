// Package gc is the declarative retention engine for the .evolve data tree
// (L3.1, concurrency campaign). It generalizes the fixed-rule pruneephemeral
// port into policy-driven rules loaded from .evolve/policy.json:gc.
//
// Split of responsibilities:
//   - Plan evaluates the rules against an injected run-dir list plus the
//     salvage/dispatch-log/tracker surfaces and returns a Manifest of
//     would-archive/would-delete actions. Plan never mutates the tree — it IS
//     the dry-run (the L3.4 shadow stage logs exactly this manifest).
//   - Apply executes a Manifest (archive = move under <evolve>/archive/runs/,
//     delete = RemoveAll).
//   - Run-dir DISCOVERY is deliberately not in this file: the production
//     discovery (layout-agnostic, lease-aware) is L3.2; until then callers
//     and tests inject []RunDir.
//
// Hard rules, NOT configurable by policy:
//   - quarantine/** is manual-only: the engine refuses to emit an action for
//     any path under <evolve>/quarantine even if discovery hands one in.
//   - ledger.jsonl / ledger.tip / ledger-segments/** are never touched
//     (append-only tamper-evident history; sealing is L3.3's job).
//   - a LIVE run dir is never touched, no matter its age.
package gc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Policy is the .evolve/policy.json:gc block. The zero value means
// "defaults": see withDefaults. A zero ArchiveAfterDays/DeleteAfterDays
// disables that action entirely — retention never escalates by default.
type Policy struct {
	Runs RunsPolicy `json:"runs,omitempty"`
	// SalvageTTLDays prunes <evolve>/operator-salvage entries. Default 30.
	SalvageTTLDays int `json:"salvage_ttl_days,omitempty"`
	// LogsTTLDays prunes <evolve>/dispatch-logs/*.log. Default 30.
	LogsTTLDays int `json:"logs_ttl_days,omitempty"`
	// TrackerTTLDays prunes <run-dir>/.ephemeral subtrees of KEPT runs.
	// Default 7 (mirrors pruneephemeral).
	TrackerTTLDays int `json:"tracker_ttl_days,omitempty"`
}

// RunsPolicy is the retention ladder for run directories.
type RunsPolicy struct {
	// KeepFull: the newest N run dirs (by mtime, live or not — live runs are
	// protected independently of this count) are always kept in full,
	// however old. Default 10.
	KeepFull int `json:"keep_full,omitempty"`
	// ArchiveAfterDays: beyond KeepFull, a dead run STRICTLY older than this
	// is moved under <evolve>/archive/runs/. 0 = never archive.
	ArchiveAfterDays int `json:"archive_after_days,omitempty"`
	// DeleteAfterDays: beyond KeepFull, a dead run STRICTLY older than this
	// is deleted. 0 = never delete. Delete wins over archive when both match.
	DeleteAfterDays int `json:"delete_after_days,omitempty"`
}

func (p Policy) withDefaults() Policy {
	if p.Runs.KeepFull == 0 {
		p.Runs.KeepFull = 10
	}
	if p.SalvageTTLDays == 0 {
		p.SalvageTTLDays = 30
	}
	if p.LogsTTLDays == 0 {
		p.LogsTTLDays = 30
	}
	if p.TrackerTTLDays == 0 {
		p.TrackerTTLDays = 7
	}
	return p
}

// RunDir is one discovered run directory. Discovery is injected: L3.2's
// layout-agnostic, lease-aware discovery is the production source; tests
// build synthetic lists.
type RunDir struct {
	Path    string
	ModTime time.Time
	// Live marks a run that must never be touched: non-terminal run state or
	// a fresh .lease heartbeat (L3.2 decides; the engine only honors it).
	Live bool
}

// Action is what Apply will do to a path.
type Action string

const (
	// ActionArchive moves a path under <evolve>/archive/runs/ rather than
	// removing it — the retention ladder's intermediate, recoverable step.
	ActionArchive Action = "archive"
	// ActionDelete removes a path recursively (os.RemoveAll). Delete wins
	// over archive when a run matches both age thresholds.
	ActionDelete Action = "delete"
)

// Item is one planned action. Rule names which policy rule fired — the
// shadow manifest (L3.4) logs it so an operator can trace every entry.
type Item struct {
	Path   string `json:"path"`
	Action Action `json:"action"`
	Rule   string `json:"rule"`
}

// Manifest is the full plan. Plan output is deterministic (sorted by path)
// so shadow-soak diffs are stable.
type Manifest struct {
	Items []Item `json:"items"`
}

// Options drives Plan.
type Options struct {
	// EvolveDir is the .evolve root (absolute). Required.
	EvolveDir string
	Policy    Policy
	// Runs is the injected run-dir discovery result.
	Runs []RunDir
	Now  func() time.Time
}

// Plan evaluates retention rules and returns the action manifest. It never
// mutates the tree.
func Plan(opts Options) (Manifest, error) {
	if opts.EvolveDir == "" || !filepath.IsAbs(opts.EvolveDir) {
		return Manifest{}, fmt.Errorf("gc: EvolveDir must be absolute, got %q", opts.EvolveDir)
	}
	pol := opts.Policy.withDefaults()
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	var items []Item
	add := func(path string, action Action, rule string) {
		if protected(opts.EvolveDir, path) {
			return // hard rule: quarantine manual-only, ledger append-only
		}
		items = append(items, Item{Path: path, Action: action, Rule: rule})
	}

	// Rule 1: run-dir retention ladder. Newest KeepFull are kept in full;
	// beyond that, dead runs age into archive then delete. Live always kept.
	for i, r := range sortRunsNewestFirst(opts.Runs) {
		if r.Live || i < pol.Runs.KeepFull {
			continue
		}
		age := ageDays(now(), r.ModTime)
		switch {
		case pol.Runs.DeleteAfterDays > 0 && age > float64(pol.Runs.DeleteAfterDays):
			add(r.Path, ActionDelete, "runs.delete_after_days")
		case pol.Runs.ArchiveAfterDays > 0 && age > float64(pol.Runs.ArchiveAfterDays):
			add(r.Path, ActionArchive, "runs.archive_after_days")
		}
	}

	// Rule 2: tracker .ephemeral subtrees inside KEPT runs (pruneephemeral's
	// phase 1, generalized). Runs already planned away take their .ephemeral
	// with them, so only un-planned runs are scanned. LIVE runs are skipped
	// entirely — the hard rule covers their subtrees too (deleting a running
	// session's tracker state would corrupt it).
	planned := make(map[string]bool, len(items))
	for _, it := range items {
		planned[it.Path] = true
	}
	for _, r := range opts.Runs {
		if r.Live || planned[r.Path] {
			continue
		}
		eph := filepath.Join(r.Path, ".ephemeral")
		if info, err := os.Stat(eph); err == nil && info.IsDir() &&
			ageDays(now(), info.ModTime()) > float64(pol.TrackerTTLDays) {
			add(eph, ActionDelete, "tracker_ttl_days")
		}
	}

	// Rule 3: operator-salvage TTL (top-level entries by mtime).
	for _, e := range dirEntriesOlderThan(filepath.Join(opts.EvolveDir, "operator-salvage"), now(), pol.SalvageTTLDays, nil) {
		add(e, ActionDelete, "salvage_ttl_days")
	}

	// Rule 4: dispatch-log TTL (*.log files only — pruneephemeral's phase 2,
	// generalized from batch-*.log to any .log in that directory).
	logFilter := func(name string, isDir bool) bool {
		return !isDir && strings.HasSuffix(name, ".log")
	}
	for _, e := range dirEntriesOlderThan(filepath.Join(opts.EvolveDir, "dispatch-logs"), now(), pol.LogsTTLDays, logFilter) {
		add(e, ActionDelete, "logs_ttl_days")
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return Manifest{Items: items}, nil
}

// dirEntriesOlderThan lists direct children of dir whose mtime is older than
// ttlDays. A missing dir yields nothing. filter (optional) limits which
// entries qualify.
func dirEntriesOlderThan(dir string, now time.Time, ttlDays int, filter func(name string, isDir bool) bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if filter != nil && !filter(e.Name(), e.IsDir()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if ageDays(now, info.ModTime()) > float64(ttlDays) {
			out = append(out, p)
		}
	}
	return out
}

// Apply executes a manifest: archive items are moved under
// <evolve>/archive/runs/<basename>, delete items are removed recursively.
// Protected paths are refused (defense in depth — Plan never emits them,
// but a hand-edited manifest must not bypass the hard rules). Each item
// failure is reported but does not stop the rest; the joined error (if
// any) lists every failure.
//
// Replay note for the shadow stage (L3.4): deletes are idempotent
// (RemoveAll on a missing path is a no-op) but archives are NOT — re-applying
// a manifest whose archive items already moved fails loudly on the missing
// source. Always Plan fresh, never re-Apply a stored manifest.
func Apply(evolveDir string, m Manifest) error {
	if evolveDir == "" || !filepath.IsAbs(evolveDir) {
		// Same guard as Plan: a relative root would make protected() refuse
		// everything silently and resolve the archive dst against the CWD.
		return fmt.Errorf("gc: evolveDir must be absolute, got %q", evolveDir)
	}
	var errs []error
	for _, it := range m.Items {
		if protected(evolveDir, it.Path) {
			errs = append(errs, fmt.Errorf("gc: refusing protected path %s (quarantine/ledger are manual-only)", it.Path))
			continue
		}
		// TOCTOU re-check (L3.2): liveness can change between Plan and Apply —
		// a fleet scheduler may lease a run, or a new cycle may claim the dir
		// as its workspace, in that window. Re-verify at act time on the item
		// and its parent (subtree items like .ephemeral live under the run
		// dir) and refuse a now-live target loudly. Any verification error
		// counts as live (fail closed).
		if live, why := nowLive(evolveDir, it.Path); live {
			errs = append(errs, fmt.Errorf("gc: refusing %s — became live after Plan (%s); re-Plan and re-Apply", it.Path, why))
			continue
		}
		switch it.Action {
		case ActionDelete:
			if err := os.RemoveAll(it.Path); err != nil {
				errs = append(errs, fmt.Errorf("gc: delete %s: %w", it.Path, err))
			}
		case ActionArchive:
			base := filepath.Base(it.Path)
			archiveDir := filepath.Join(evolveDir, "archive", "runs")
			dst := filepath.Join(archiveDir, base)
			if err := os.MkdirAll(archiveDir, 0o755); err != nil {
				errs = append(errs, fmt.Errorf("gc: archive mkdir for %s: %w", it.Path, err))
				continue
			}
			// Never overwrite an existing archive entry — disambiguate with a
			// numeric suffix (deterministic, no clock dependency).
			for n := 1; ; n++ {
				if _, err := os.Lstat(dst); err != nil {
					break
				}
				dst = filepath.Join(archiveDir, base+"."+strconv.Itoa(n))
			}
			// Rename requires src and dst on the same filesystem; both live
			// under .evolve so this holds unless an operator mounts
			// .evolve/archive separately — then this surfaces as EXDEV.
			if err := os.Rename(it.Path, dst); err != nil {
				errs = append(errs, fmt.Errorf("gc: archive %s → %s: %w", it.Path, dst, err))
			}
		default:
			errs = append(errs, fmt.Errorf("gc: unknown action %q for %s", it.Action, it.Path))
		}
	}
	return errors.Join(errs...)
}

// protected reports whether path may never be acted on: quarantine is
// manual-only, ledger history is append-only, and already-archived entries
// are terminal (re-collecting archive/ would double-archive or destroy the
// only remaining copy).
func protected(evolveDir, path string) bool {
	rel, err := filepath.Rel(evolveDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Outside the evolve dir entirely: refuse — gc only manages .evolve.
		return true
	}
	first := strings.Split(filepath.ToSlash(rel), "/")[0]
	switch first {
	case "quarantine", "ledger.jsonl", "ledger.tip", "ledger-segments", "ledger.lock", "archive":
		return true
	}
	return false
}

// nowLive re-checks liveness at Apply time (wall clock, default lease TTL):
// a fresh .lease on the path or its parent, or either being the in-flight
// run's workspace, refuses the action. currentWorkspace errors count as
// live — fail closed.
func nowLive(evolveDir, path string) (bool, string) {
	now := time.Now()
	for _, d := range []string{path, filepath.Dir(path)} {
		if leaseFresh(d, now, 0) {
			return true, "fresh .lease at " + d
		}
	}
	ws, err := currentWorkspace(evolveDir)
	if err != nil {
		return true, "run-state unreadable: " + err.Error()
	}
	if ws != "" && (path == ws || filepath.Dir(path) == ws) {
		return true, "current run workspace"
	}
	return false, ""
}

// ageDays is a helper shared by the TTL rules.
func ageDays(now time.Time, mod time.Time) float64 {
	return now.Sub(mod).Hours() / 24
}

// sortRunsNewestFirst orders runs by ModTime descending (stable for tests).
func sortRunsNewestFirst(runs []RunDir) []RunDir {
	out := make([]RunDir, len(runs))
	copy(out, runs)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}
