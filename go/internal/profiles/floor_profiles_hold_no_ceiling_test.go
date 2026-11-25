package profiles_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func effectiveFloors(t *testing.T) policy.Floors {
	t.Helper()
	root := filepath.Join(profiles.RealProfilesDir(t), "..", "..")
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load the tracked policy: %v", err)
	}
	return pol.Floors()
}

func TestFloorProfiles_HoldNoAllowedCLIs(t *testing.T) {
	loader, _ := profiles.RealTreeProfiles(t)
	floors := effectiveFloors(t)
	checked := 0
	for _, family := range floors.Families() {
		for _, name := range floors[family] {
			p, err := loader.Get(name)
			if err != nil {
				t.Errorf("floor agent %s does not resolve in the tracked tree: %v", name, err)
				continue
			}
			checked++
			if len(p.AllowedCLIs) != 0 {
				t.Errorf("floor agent %s holds allowed_clis %v: the floor lives in cli_routing.floors only", name, p.AllowedCLIs)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no floor agent was checked, so this guard is vacuous")
	}
}
