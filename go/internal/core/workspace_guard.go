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
	// lane-scope.json is provisioned by the fleet supervisor BEFORE the cycle
	// runs (cycle-640 lane pin) — pre-phase by design, not pollution.
	// minimal: a genuinely polluted dir is archived whole, pin included; the
	// env-snapshot fallback re-materializes the pin for fleet lanes.
	pollution := 0
	for _, e := range entries {
		if e.Name() != LaneScopeFile {
			pollution++
		}
	}
	if pollution == 0 {
		return nil
	}
	stamp := now().UTC().Format("20060102T150405.000000000")
	archived := workspace + ".polluted-" + stamp
	if err := os.Rename(workspace, archived); err != nil {
		return fmt.Errorf("rename to %s: %w", archived, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] archived polluted workspace: %s -> %s (%d files)\n",
		workspace, archived, pollution)
	return nil
}

// defaultGitHEAD runs `git rev-parse HEAD` in cwd.
// Returns empty string on error AND emits a one-line WARN to stderr so
// operators see the degraded-mode signal that yields SKIPPED_UNKNOWN.
// finalizeOutcome treats equal strings as no movement.
