// Package mintregistry records the advisor-minted phase names that the tree-diff guard exempts.
// See docs/architecture/packages/internal-mintregistry.md.
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

const TTL = 6 * time.Hour

type entry struct {
	Name     string    `json:"name"`
	MintedAt time.Time `json:"minted_at"`
}

func Path(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "active-mints.json")
}

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
				continue
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

func QuarantineCorrupt(path string) (bool, error) {
	quarantined := false
	err := flock.WithPathLock(path, func() error {
		if _, readErr := read(path); readErr == nil || errors.Is(readErr, os.ErrNotExist) {
			return nil
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
