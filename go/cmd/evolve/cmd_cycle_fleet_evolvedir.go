package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

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
