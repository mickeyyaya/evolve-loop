package guards

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestProtectedSurface_CompiledDefaultOverlaySkills locks the intent that EVERY
// skill the kernel preloads on its own authority (policy's compiled-default
// overlays: fable on deep/top tiers, the solution-* personas on document
// cycles — policy.ResolveOverlays → bridge skill-overlay injection) is
// integrity-protected: a tampered persona would silently rewrite the phase
// agent's operating discipline or the audit's grading rubric. The list is
// policy's own export, so the next compiled skill cannot skip the manifest
// (docs/architecture/skill-overlays.md "Integrity").
func TestProtectedSurface_CompiledDefaultOverlaySkills(t *testing.T) {
	skills := policy.CompiledDefaultOverlaySkills()
	if len(skills) == 0 {
		t.Fatal("policy.CompiledDefaultOverlaySkills() is empty — the compiled default must at least carry fable")
	}
	for _, s := range skills {
		for _, p := range []string{"skills/" + s + "/SKILL.md", "/skills/" + s + "/"} {
			if !IsProtectedSurface(p) {
				t.Errorf("%s must be a protected surface — the kernel preloads it into phase prompts (audit-F1)", p)
			}
		}
	}
}
