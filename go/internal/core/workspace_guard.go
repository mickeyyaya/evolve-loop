package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
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
	// lane-scope.json is provisioned by the fleet supervisor before the cycle
	// runs — pre-phase by design, not pollution.
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
	archived := workspace + gcpolicy.PollutedArchiveName(now())
	if err := os.Rename(workspace, archived); err != nil {
		return fmt.Errorf("rename to %s: %w", archived, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] archived polluted workspace: %s -> %s (%d files)\n",
		workspace, archived, pollution)
	return nil
}
