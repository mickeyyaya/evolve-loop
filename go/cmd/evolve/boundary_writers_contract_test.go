package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func TestBoundaryWritersContractBindsEveryHomeToTheLaneBase(t *testing.T) {
	t.Parallel()
	homes := map[string]string{
		"dossier":       dossier.Home + "/",
		"inbox":         inboxstamps.Pathspec,
		"retrospective": recoveryretro.DocsHome + "/",
	}
	for writer, home := range homes {
		if !core.BoundaryOwned(home + "probe") {
			t.Errorf("the %s home %q is not in the lane-base boundary list", writer, home)
		}
	}
	if core.BoundaryOwned("go/internal/core/landed_main.go") {
		t.Error("a source path must not count as boundary-owned")
	}
}
