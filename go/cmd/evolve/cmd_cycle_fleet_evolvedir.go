package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// fleetLaneEvolveDirOK refuses a fleet lane whose evolve dir is not <projectRoot>/.evolve. The lane's
// cycle-state override lives under <projectRoot>/.evolve/runs and applies only inside the evolve dir
// storage resolves against, so any other dir would put the lane back on the shared cycle-state.json.
func fleetLaneEvolveDirOK(projectRoot, evolveDir string) error {
	if os.Getenv(ipcenv.FleetKey) == "" {
		return nil
	}
	want := filepath.Join(projectRoot, ".evolve")
	got, err := filepath.Abs(evolveDir)
	if err != nil || got != want {
		return fmt.Errorf("a fleet lane keeps its cycle state under %s; --evolve-dir %s is not it", want, evolveDir)
	}
	return nil
}
