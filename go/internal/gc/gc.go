// Package gc is the declarative retention engine for the .evolve data tree.
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

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
)

type (
	Policy          = gcpolicy.Policy
	RunsPolicy      = gcpolicy.RunsPolicy
	WorktreesPolicy = gcpolicy.WorktreesPolicy
)

type RunDir struct {
	Path    string
	ModTime time.Time
	Live    bool
}

type Action string

const (
	ActionArchive Action = "archive"
	ActionDelete  Action = "delete"
)

type Item struct {
	Path   string `json:"path"`
	Action Action `json:"action"`
	Rule   string `json:"rule"`
}

type Manifest struct {
	Items    []Item   `json:"items"`
	Warnings []string `json:"warnings,omitempty"`
}

type Options struct {
	EvolveDir string
	Policy    Policy
	Runs      []RunDir
	Now       func() time.Time
}

func Plan(opts Options) (Manifest, error) {
	if opts.EvolveDir == "" || !filepath.IsAbs(opts.EvolveDir) {
		return Manifest{}, fmt.Errorf("gc: EvolveDir must be absolute, got %q", opts.EvolveDir)
	}
	pol := opts.Policy.WithDefaults()
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	var items []Item
	add := func(path string, action Action, rule string) {
		if protected(opts.EvolveDir, path) {
			return
		}
		items = append(items, Item{Path: path, Action: action, Rule: rule})
	}

	planRunLadder(opts.Runs, pol, now, add)

	planTrackerTTL(opts.Runs, items, pol, now, add)

	for _, e := range dirEntriesOlderThan(OperatorSalvageDir(opts.EvolveDir), now(), pol.SalvageTTLDays, nil) {
		add(e, ActionDelete, "salvage_ttl_days")
	}

	var warnings []string
	logPlanner{now: now(), add: add, warn: func(w string) { warnings = append(warnings, w) }}.planCatalog(opts.EvolveDir, pol.LogCatalog())

	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return Manifest{Items: items, Warnings: warnings}, nil
}

func planRunLadder(runs []RunDir, pol Policy, now func() time.Time, add func(string, Action, string)) {
	for i, r := range sortRunsNewestFirst(runs) {
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
}

func planTrackerTTL(runs []RunDir, items []Item, pol Policy, now func() time.Time, add func(string, Action, string)) {
	planned := make(map[string]bool, len(items))
	for _, it := range items {
		planned[it.Path] = true
	}
	for _, r := range runs {
		if r.Live || planned[r.Path] {
			continue
		}
		eph := filepath.Join(r.Path, ".ephemeral")
		if info, err := os.Stat(eph); err == nil && info.IsDir() &&
			ageDays(now(), info.ModTime()) > float64(pol.TrackerTTLDays) {
			add(eph, ActionDelete, "tracker_ttl_days")
		}
	}
}

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

func Apply(evolveDir string, m Manifest) error {
	if evolveDir == "" || !filepath.IsAbs(evolveDir) {
		return fmt.Errorf("gc: evolveDir must be absolute, got %q", evolveDir)
	}
	var errs []error
	for _, it := range m.Items {
		if protected(evolveDir, it.Path) {
			errs = append(errs, fmt.Errorf("gc: refusing protected path %s (quarantine/ledger are manual-only)", it.Path))
			continue
		}
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
			if err := archiveItem(evolveDir, it.Path); err != nil {
				errs = append(errs, err)
			}
		default:
			errs = append(errs, fmt.Errorf("gc: unknown action %q for %s", it.Action, it.Path))
		}
	}
	return errors.Join(errs...)
}

func archiveItem(evolveDir, path string) error {
	base := filepath.Base(path)
	archiveDir := filepath.Join(evolveDir, "archive", "runs")
	dst := filepath.Join(archiveDir, base)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("gc: archive mkdir for %s: %w", path, err)
	}
	for n := 1; ; n++ {
		if _, err := os.Lstat(dst); err != nil {
			break
		}
		dst = filepath.Join(archiveDir, base+"."+strconv.Itoa(n))
	}
	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("gc: archive %s → %s: %w", path, dst, err)
	}
	return nil
}

func protected(evolveDir, path string) bool {
	rel, err := filepath.Rel(evolveDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return true
	}
	first := strings.Split(filepath.ToSlash(rel), "/")[0]
	switch first {
	case "quarantine", "ledger.jsonl", "ledger.tip", "ledger-segments", ledgerartifacts.DirName, "ledger.lock", "archive":
		return true
	}
	return false
}

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

func ageDays(now time.Time, mod time.Time) float64 {
	return now.Sub(mod).Hours() / 24
}

func sortRunsNewestFirst(runs []RunDir) []RunDir {
	out := make([]RunDir, len(runs))
	copy(out, runs)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}
