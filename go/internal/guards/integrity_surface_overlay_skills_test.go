package guards

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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
