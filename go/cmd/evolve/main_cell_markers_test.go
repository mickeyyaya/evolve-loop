package main

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestMain_ClearsInheritedCellMarkers(t *testing.T) {
	for _, name := range ipcenv.CellMarkers() {
		if value, set := os.LookupEnv(name); set {
			t.Errorf("TestMain left the inherited cell marker %s=%q; a CLI test then runs as a phase agent", name, value)
		}
	}
}
