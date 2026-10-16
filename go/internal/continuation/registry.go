package continuation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

const registryName = "continuation-registry.json"

func RegistryPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", registryName)
}

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

func WriteRegistryEntry(projectRoot, scopeID string, c Continuation) error {
	if scopeID == "" {
		return fmt.Errorf("continuation: empty scope id is not a registry key")
	}
	return flock.WithPathLock(RegistryPath(projectRoot), func() error {
		return writeRegistryEntryLocked(projectRoot, scopeID, c)
	})
}

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

func ListRegistryEntries(projectRoot string) (map[string]Continuation, error) {
	return readRegistry(projectRoot)
}

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

func ExpandHostPaths(c Continuation) Continuation {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return c
	}
	prefix := "~" + string(filepath.Separator)
	expand := func(p string) string {
		if strings.HasPrefix(p, prefix) {
			return filepath.Join(home, strings.TrimPrefix(p, prefix))
		}
		return p
	}
	c.Worktree = expand(c.Worktree)
	c.FindingsPath = expand(c.FindingsPath)
	return c
}

func AppendReleased(doc map[string]any, c Continuation, reason string) []any {
	c = RedactHostPaths(c)
	var list []any
	if prev, ok := doc["released_continuations"].([]any); ok {
		list = prev
	}
	return append(list, map[string]any{
		"worktree":      c.Worktree,
		"branch":        c.Branch,
		"snapshot_sha":  c.SnapshotSHA,
		"base_sha":      c.BaseSHA,
		"findings_path": c.FindingsPath,
		"cycle":         c.Cycle,
		"released_at":   time.Now().UTC().Format(time.RFC3339),
		"reason":        reason,
	})
}
