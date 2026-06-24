package core

// writeaxis_test.go — PA-DDK DDK-7b (ADR-0060). The source-write axis (which
// phases write into the cycle worktree) is now declared in the registry via
// `writes_source`, with the WorktreePhase literal as the catalog-less floor for
// the role-gate. This test pins that the two AGREE for every core phase — so the
// config declaration and the security-boundary literal can never silently drift.
// Rename-proof: it iterates whatever the loaded registry contains; no phase name
// is hardcoded.

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
