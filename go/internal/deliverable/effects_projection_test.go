package deliverable

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func checkedInRegistry(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestPhaseRegistry_EveryDeclaredEffectHasACheck(t *testing.T) {
	cat := checkedInRegistry(t)
	declared := map[string][]string{} // effect → phases declaring it
	for _, spec := range cat.All() {
		for _, e := range spec.Effects {
			declared[e] = append(declared[e], spec.Name)
			if _, bound := effects[e]; !bound {
				t.Errorf("phase %q declares effect %q but effects.go binds no check for it — the gate would report unbound_effect on every cycle", spec.Name, e)
			}
		}
	}
	for name := range effects {
		if len(declared[name]) == 0 {
			t.Errorf("effects.go binds a check for %q but no registry phase declares it — dead code, or a phase whose persona performs the effect is not declaring it", name)
		}
	}
	if phases := declared["inbox-claim"]; len(phases) != 1 || phases[0] != "triage" {
		t.Errorf("inbox-claim is triage's effect (persona Step 0a.4) and nobody else's; declared by %v", phases)
	}
}
