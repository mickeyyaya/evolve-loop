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
	"strings"

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

// publishRegistryLocked marshals and atomically publishes the registry map.
// The caller MUST hold flock.WithPathLock(RegistryPath(projectRoot)) — shared
// by the write and delete paths so the on-disk publish contract cannot
// diverge between them.
func publishRegistryLocked(projectRoot string, byScope map[string]Continuation) error {
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

// DeleteRegistryEntry releases scopeID's binding unconditionally. The missing
// half of the registry lifecycle (2026-08-10 stall: entries were immortal, so
// a binding whose snapshot had landed or whose worktree was reaped re-armed
// the defect-ledger gate's out-of-band check on every future lane of that
// scope — the absorbing-FAIL state, cycles 1412/1418). Deleting an absent
// scope is a clean no-op; an empty scope id is rejected to mirror
// WriteRegistryEntry. Callers are ORCHESTRATOR-side only — lane agents have
// no write path to the root-owned registry, which is what keeps the gate's
// cycle-1285 anti-tamper property intact. When the release is conditional on
// WHICH ancestor the binding names, use DeleteRegistryEntryIfCycle — a
// read-then-delete across two locks loses a sibling lane's concurrent rebind.
func DeleteRegistryEntry(projectRoot, scopeID string) error {
	if scopeID == "" {
		return fmt.Errorf("continuation: empty scope id is not a registry key")
	}
	return flock.WithPathLock(RegistryPath(projectRoot), func() error {
		byScope, err := readRegistry(projectRoot)
		if err != nil {
			return err
		}
		if _, ok := byScope[scopeID]; !ok {
			return nil
		}
		delete(byScope, scopeID)
		return publishRegistryLocked(projectRoot, byScope)
	})
}

// DeleteRegistryEntryIfCycle releases scopeID's binding ONLY if it currently
// names ancestor cycle — check and delete under ONE lock hold, so a sibling
// lane that rebinds the scope between an unlocked read and this call keeps
// its fresh binding (the TOCTOU lost-update the adversarial review blocked).
// Returns whether a binding was released.
func DeleteRegistryEntryIfCycle(projectRoot, scopeID string, cycle int) (bool, error) {
	if scopeID == "" {
		return false, fmt.Errorf("continuation: empty scope id is not a registry key")
	}
	released := false
	err := flock.WithPathLock(RegistryPath(projectRoot), func() error {
		byScope, err := readRegistry(projectRoot)
		if err != nil {
			return err
		}
		bound, ok := byScope[scopeID]
		if !ok || bound.Cycle != cycle {
			return nil
		}
		delete(byScope, scopeID)
		released = true
		return publishRegistryLocked(projectRoot, byScope)
	})
	return released, err
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

// ListRegistryEntries returns every scope→binding pair the registry holds, so
// an operator surface (`evolve continuation list`) reads the same map the
// resolver does instead of hand-parsing the file under its flock sidecar. An
// absent registry is an empty map, not an error — no binding is the normal
// state; a corrupt one is loud, exactly as readRegistry decides for every other
// caller.
func ListRegistryEntries(projectRoot string) (map[string]Continuation, error) {
	return readRegistry(projectRoot)
}

// RedactHostPaths returns c with the operator's home directory collapsed to
// "~". The preserved-pointer annotations written by the retire/consume paths
// ride the ship commit into TRACKED .evolve/inbox items on a public remote, and
// Worktree/FindingsPath are absolute host paths that carry the account name
// (audit cycle-1507 M1). The tilde form stays operator-usable and stays a
// stable salvage pointer; SnapshotSHA/BaseSHA/Branch — the fields salvage
// actually resolves work from — are untouched. An unresolvable home is a
// no-op rather than a failure: redaction must never block a release.
func RedactHostPaths(c Continuation) Continuation {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return c
	}
	prefix := strings.TrimSuffix(home, string(filepath.Separator)) + string(filepath.Separator)
	tilde := func(p string) string {
		if strings.HasPrefix(p, prefix) {
			return "~" + string(filepath.Separator) + strings.TrimPrefix(p, prefix)
		}
		return p
	}
	c.Worktree = tilde(c.Worktree)
	c.FindingsPath = tilde(c.FindingsPath)
	return c
}
