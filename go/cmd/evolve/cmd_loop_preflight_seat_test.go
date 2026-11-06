package main

import (
	"bytes"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TestLoopPreflightOptions_TheProductionPreflightMeasuresTheHolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := loopPreflightOptions(loopConfig{ProjectRoot: root, EvolveDir: paths.EvolveDirOf(root)}, &bytes.Buffer{})
	if o.SeatInspect == nil {
		t.Fatal("the production preflight does not measure the seat login")
	}
}
