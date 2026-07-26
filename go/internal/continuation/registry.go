package continuation

// registry.go — ADR-0076 slice C, G2: the scope-id-keyed continuation registry.
// G1 binds preserved work to an INBOX CLAIM (the stamp on
// .evolve/inbox/processing/cycle-N/<item>.json), so a lane whose scope came
// from the wave planner rather than a claim had nothing to stamp — cycle-1078's
// preserved snapshot was orphaned for exactly that reason. This is the second
// scope-identity class: a root-owned map of lane-scope todo id → the SAME
// Continuation value the manifest carries. One schema, two identity classes; a
// registry-only field or a forked format would reopen the drift this package
// exists to prevent.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// registryName is the root-owned artifact holding scope-id → Continuation.
const registryName = "continuation-registry.json"

// RegistryPath is the single well-known location of the registry, so no reader
// has to guess it.
func RegistryPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", registryName)
}

// readRegistry loads the whole registry. A missing file is an empty map; a
// present-but-unparseable one is an error — schema drift must be loud (same
// rule as ReadManifest).
func readRegistry(projectRoot string) (map[string]Continuation, error) {
	body, err := os.ReadFile(RegistryPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Continuation{}, nil
		}
		return nil, fmt.Errorf("continuation: read registry: %w", err)
	}
	byScope := map[string]Continuation{}
	if err := json.Unmarshal(body, &byScope); err != nil {
		return nil, fmt.Errorf("continuation: parse registry: %w", err)
	}
	return byScope, nil
}

// WriteRegistryEntry binds c to scopeID, read-modify-write and atomically: the
// registry is a MAP shared by every lane, so a whole-file overwrite would
// orphan every other lane's preserved work. A re-attempt of the same scope
// replaces its own entry (latest preserved work wins). An empty scope id is
// never a key, and a corrupt registry is refused loudly rather than silently
// flattened.
//
// The whole read-modify-write holds the registry's flock sidecar. The registry
// lives at the shared PROJECT ROOT, so a fleet wave's lanes write this one map
// concurrently: unserialized, two lanes that both read the pre-state lose one
// binding — the same orphaned-salvage defect G2 exists to close — and their
// shared `.tmp` publishes interleave. flock.WithPathLock is the established
// home for this "<file>.lock" convention (checkpoint.ApplyToStateFile,
// recurrence.ledger), and it serializes goroutines and processes alike.
func WriteRegistryEntry(projectRoot, scopeID string, c Continuation) error {
	if scopeID == "" {
		return fmt.Errorf("continuation: empty scope id is not a registry key")
	}
	return flock.WithPathLock(RegistryPath(projectRoot), func() error {
		return writeRegistryEntryLocked(projectRoot, scopeID, c)
	})
}

// writeRegistryEntryLocked is the critical section of WriteRegistryEntry. The
// caller MUST already hold flock.WithPathLock(RegistryPath(projectRoot)).
func writeRegistryEntryLocked(projectRoot, scopeID string, c Continuation) error {
	byScope, err := readRegistry(projectRoot)
	if err != nil {
		return err
	}
	byScope[scopeID] = c
	body, err := json.MarshalIndent(byScope, "", "  ")
	if err != nil {
		return fmt.Errorf("continuation: marshal registry: %w", err)
	}
	path := RegistryPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("continuation: registry dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("continuation: write registry: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("continuation: publish registry: %w", err)
	}
	return nil
}

// ReadRegistryEntry returns the binding scopeID carries. An absent registry, an
// unknown scope and a blank scope id are all clean misses (zero, false, nil) —
// the ordinary case for every cycle that has nothing to resume. A corrupt
// registry is an error.
func ReadRegistryEntry(projectRoot, scopeID string) (Continuation, bool, error) {
	if scopeID == "" {
		return Continuation{}, false, nil
	}
	byScope, err := readRegistry(projectRoot)
	if err != nil {
		return Continuation{}, false, err
	}
	c, ok := byScope[scopeID]
	if !ok {
		return Continuation{}, false, nil
	}
	return c, true, nil
}
