package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
)

func TestRegistry_DeclaresWriteAxisForSourceWriters(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	checked := 0
	for _, name := range ref.Catalog.Names() {
		p := phaseFromRouter(name)
		if p == "" {
			continue // optional/composable phases are not core Phase constants
		}
		spec, ok := ref.Catalog.Get(name)
		if !ok {
			continue
		}
		checked++
		if got, want := spec.WritesSource, WorktreePhase(p); got != want {
			t.Errorf("phase %q: registry writes_source=%v but the kernel write-axis floor=%v — config SSOT and the catalog-less role-gate literal must agree", name, got, want)
		}
	}
	if checked == 0 {
		t.Fatal("no core phases resolved from the registry — the agreement check ran vacuously")
	}
}
