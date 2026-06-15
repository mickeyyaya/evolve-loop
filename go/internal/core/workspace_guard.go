package core

import (
	"fmt"
	"os"
	"time"
)

func archivePollutedWorkspace(workspace string, now func() time.Time) error {
	info, err := os.Stat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return fmt.Errorf("readdir workspace: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	stamp := now().UTC().Format("20060102T150405.000000000")
	archived := workspace + ".polluted-" + stamp
	if err := os.Rename(workspace, archived); err != nil {
		return fmt.Errorf("rename to %s: %w", archived, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] archived polluted workspace: %s -> %s (%d files)\n",
		workspace, archived, len(entries))
	return nil
}

// defaultGitHEAD runs `git rev-parse HEAD` in cwd.
// Returns empty string on error AND emits a one-line WARN to stderr so
// operators see the degraded-mode signal that yields SKIPPED_UNKNOWN.
// finalizeOutcome treats equal strings as no movement.
