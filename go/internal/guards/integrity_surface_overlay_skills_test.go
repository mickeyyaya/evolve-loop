package guards

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestProtectedSurface_CompiledDefaultOverlaySkills(t *testing.T) {
	skills := policy.CompiledDefaultOverlaySkills()
	if !slices.Contains(skills, "fable") {
		t.Fatalf("policy.CompiledDefaultOverlaySkills() = %v: the compiled default must carry fable, the persona that deep and top phases preload", skills)
	}
	for _, s := range skills {
		for _, p := range []string{"skills/" + s + "/SKILL.md", "/skills/" + s + "/"} {
			if !IsProtectedSurface(p) {
				t.Errorf("%s must be a protected surface — the kernel preloads it into phase prompts (audit-F1)", p)
			}
		}
	}
}
