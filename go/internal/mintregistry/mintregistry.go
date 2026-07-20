// Package mintregistry is the shared per-project record of advisor-minted
// phase names — the cross-lane channel that lets the tree-diff guard
// distinguish a concurrent lane's sanctioned mint of
// .evolve/phases/<name>/phase.json from a real deliverable leak (the
// cycle-967 false-abort: lane-970's mint was charged to lane-967's PASS
// scout because both lanes diff the SAME shared tree).
//
// The registry is written ONLY by the registrar (the trust-kernel clamp that
// persists the mint itself) and read by the guard. It is anchored to the
// PROJECT root, not the configured evolve dir, because the guard only sees
// writes under ProjectRoot — an exemption anchored anywhere else could never
// match a leaked path.
package mintregistry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// TTL bounds how long a registered mint exempts its .evolve/phases/<name>
// path. It must outlast the longest phase window between a lane's baseline
// snapshot and its post-phase check (well under an hour in practice) while
// keeping the exemption from widening into a standing allowlist — a re-mint
// refreshes the entry, so an actively-used name never expires mid-batch.
const TTL = 6 * time.Hour

// entry is one registered mint. MintedAt drives TTL filtering.
type entry struct {
	Name     string    `json:"name"`
	MintedAt time.Time `json:"minted_at"`
}

// Path is the single source of truth for the registry location:
// <projectRoot>/.evolve/active-mints.json.
func Path(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "active-mints.json")
}

// Append registers name as an active mint at now, serialized against
// concurrent lanes via the sidecar-lock convention (flock.WithPathLock — the
// registry's inode is rename-replaced by atomicwrite, so the data file itself
// is never the lock object). Appends also garbage-collect expired entries and
// replace a re-minted name in place, so the file stays bounded. A corrupt
// registry is reset (loudly) rather than bricking every future mint — the
// guard side independently fails safe on corruption via ActiveNames.
func Append(path, name string, now time.Time) error {
	err := flock.WithPathLock(path, func() error {
		entries, err := read(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[mintregistry] WARN resetting corrupt registry %s: %v\n", path, err)
			entries = nil
		}
		out := make([]entry, 0, len(entries)+1)
		for _, e := range entries {
			if e.Name == name || !e.MintedAt.After(now.Add(-TTL)) {
				continue // replaced below / expired
			}
			out = append(out, e)
		}
		out = append(out, entry{Name: name, MintedAt: now})
		return atomicwrite.JSON(path, out)
	})
	if err != nil {
		return fmt.Errorf("mintregistry: %w", err)
	}
	return nil
}

// ActiveNames returns the set of registered mint names still within TTL at
// now. A missing registry is an empty set; a corrupt one returns an error
// AND an empty set so callers fail toward the guard staying armed. Lock-free
// by design: atomicwrite's rename means a reader always sees a complete
// registry (old or new), never a torn one.
func ActiveNames(path string, now time.Time) (map[string]bool, error) {
	entries, err := read(path)
	if err != nil {
		return map[string]bool{}, err
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.MintedAt.After(now.Add(-TTL)) {
			names[e.Name] = true
		}
	}
	return names, nil
}

// QuarantineCorrupt renames a corrupt registry aside (to <path>.corrupt-<ts>)
// so the exemption-disabled window after a corrupt read is bounded to one
// check instead of persisting until the next mint rewrites the file. Runs
// under the sidecar lock and RE-READS before acting: a concurrent Append may
// have already repaired the file, and a healthy or missing registry must be
// left untouched. Returns whether a quarantine happened.
func QuarantineCorrupt(path string) (bool, error) {
	quarantined := false
	err := flock.WithPathLock(path, func() error {
		if _, readErr := read(path); readErr == nil || errors.Is(readErr, os.ErrNotExist) {
			return nil // healthy or missing — untouched
		}
		dest := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		if renameErr := os.Rename(path, dest); renameErr != nil {
			return renameErr
		}
		fmt.Fprintf(os.Stderr, "[mintregistry] quarantined corrupt registry %s -> %s\n", path, dest)
		quarantined = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("mintregistry: quarantine: %w", err)
	}
	return quarantined, nil
}

// read loads the raw entry list; a missing file is an empty registry.
func read(path string) ([]entry, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mintregistry: read %s: %w", path, err)
	}
	var entries []entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("mintregistry: corrupt registry %s: %w", path, err)
	}
	return entries, nil
}
