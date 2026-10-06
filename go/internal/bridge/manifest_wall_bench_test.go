package bridge

import (
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

var wallRuleName = regexp.MustCompile(`(?i)quota|rate|limit|exhaust`)

func TestManifests_EveryQuotaOrRateWallRuleBenchesItsFamily(t *testing.T) {
	names := ManifestNames()
	if len(names) < 4 {
		t.Fatalf("found %d manifests %v; the walk has lost its subjects", len(names), names)
	}
	walls := 0
	for _, cli := range names {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatalf("%s: %v", cli, err)
		}
		for _, rule := range m.InteractivePrompts {
			if rule.Policy != "escalate" || !wallRuleName.MatchString(rule.Name) {
				continue
			}
			walls++
			if !clihealth.Benchable(rule.Name) {
				t.Errorf("%s: escalate rule %q names a quota or rate wall but clihealth.Benchable rejects it, so the family is never benched", cli, rule.Name)
			}
		}
	}
	if walls == 0 {
		t.Fatal("no quota or rate wall rule found in any manifest; the walk proves nothing")
	}
}
