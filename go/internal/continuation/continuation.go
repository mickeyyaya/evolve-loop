// Package continuation is the single source for the ADR-0076 slice C
// continuation binding: the record that lets a FAILed cycle's preserved,
// snapshot-committed work be resumed by a later cycle instead of restarted
// cold. Three consumers share this one type — the orchestrator (writes the
// per-cycle manifest at the preserve decision), the inbox mover (stamps
// released items from the manifest, transactionally with the release), and the
// batch loader (parses the stamp back off the item) — so the schema can never
// drift between writer and readers.
//
// Work resumes; grades do not: a continuation carries no verdict state, and
// every phase re-runs on adoption.
package continuation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Continuation binds preserved work to its item. SnapshotSHA is an IMMUTABLE
// commit on Branch — adoption validates against it, never against mutable
// dirty state.
type Continuation struct {
	Worktree     string `json:"worktree"`      // preserved worktree path (informational; adoption seeds from the SHA)
	Branch       string `json:"branch"`        // cycle branch carrying the snapshot commit
	SnapshotSHA  string `json:"snapshot_sha"`  // immutable ref of the preserved work
	BaseSHA      string `json:"base_sha"`      // main-ancestor base the work builds on
	FindingsPath string `json:"findings_path"` // failure-digest artifact (project-root relative; read tolerantly)
	Cycle        int    `json:"cycle"`         // FAILed cycle that produced it
}

// manifestName is the per-cycle workspace artifact the orchestrator writes at
// the preserve decision and the inbox mover reads at release time.
const manifestName = "continuation-manifest.json"

// WriteManifest atomically persists c as workspace's continuation manifest.
func WriteManifest(workspace string, c Continuation) error {
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("continuation: marshal manifest: %w", err)
	}
	path := filepath.Join(workspace, manifestName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("continuation: write manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("continuation: publish manifest: %w", err)
	}
	return nil
}

// ReadManifest loads workspace's continuation manifest. A missing file is a
// clean (Continuation{}, false); a present-but-unparseable file is an error —
// schema drift must be loud, never a silent fresh start.
func ReadManifest(workspace string) (Continuation, bool, error) {
	body, err := os.ReadFile(filepath.Join(workspace, manifestName))
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
	return c, true, nil
}
