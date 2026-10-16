// Package continuation is the one schema of the binding that lets a failed cycle's preserved work be resumed.
// See docs/architecture/packages/internal-continuation.md.
package continuation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Continuation struct {
	Worktree     string `json:"worktree"`
	Branch       string `json:"branch"`
	SnapshotSHA  string `json:"snapshot_sha"`
	BaseSHA      string `json:"base_sha"`
	FindingsPath string `json:"findings_path"`
	Cycle        int    `json:"cycle"`
}

const ManifestName = "continuation-manifest.json"

func WriteManifest(workspace string, c Continuation) error {
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("continuation: marshal manifest: %w", err)
	}
	path := filepath.Join(workspace, ManifestName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("continuation: write manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("continuation: publish manifest: %w", err)
	}
	return nil
}

func ReadManifest(workspace string) (Continuation, bool, error) {
	body, err := os.ReadFile(filepath.Join(workspace, ManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return Continuation{}, false, nil
		}
		return Continuation{}, false, fmt.Errorf("continuation: read manifest: %w", err)
	}
	var c Continuation
	if err := json.Unmarshal(body, &c); err != nil {
		return Continuation{}, false, fmt.Errorf("continuation: parse manifest: %w", err)
	}
	return RedactHostPaths(c), true, nil
}
