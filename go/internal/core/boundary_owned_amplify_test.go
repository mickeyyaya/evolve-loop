package core_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func TestBoundaryOwnedAmplify_Table(t *testing.T) {
	t.Parallel()
	homes := []string{dossier.Home, inboxstamps.Pathspec, recoveryretro.DocsHome}
	for _, h := range homes {
		base := h
		if base[len(base)-1] != '/' {
			base += "/"
		}
		for _, p := range []string{base + "a.md", base + "deep/er/x.json", base + ".hidden"} {
			if !core.BoundaryOwned(p) {
				t.Errorf("%q must be boundary-owned", p)
			}
		}
	}
	for _, p := range []string{
		"",
		"/",
		"go/internal/core/landed_main.go",
		"knowledge-base/cycles-extra/x.md",
		"docs/retrospectives-extra/x.md",
		".evolve/inbox-other/x.json",
		"prefix/knowledge-base/cycles/x.md",
		"README.md",
	} {
		if core.BoundaryOwned(p) {
			t.Errorf("%q must not be boundary-owned", p)
		}
	}
}
