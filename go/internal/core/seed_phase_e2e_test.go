package core

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func seedRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestSeedPhase_BugReproductionReachesAdvisorCatalog(t *testing.T) {
	t.Parallel()
	root := seedRepoRoot(t)
	builtin, err := phasespec.Load(filepath.Join(root, "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	user, _, warns := phasespec.DiscoverUserSpecsFromRoots([]string{filepath.Join(root, ".evolve", "phases")})
	for _, w := range warns {
		t.Logf("discover warning: %s", w)
	}
	cat, mWarns := builtin.Merge(user)
	for _, w := range mWarns {
		t.Logf("merge warning: %s", w)
	}

	spec, ok := cat.Get("bug-reproduction")
	if !ok {
		t.Fatal("bug-reproduction not in the merged catalog — seed phase missing from .evolve/phases/")
	}
	if v := phasespec.ValidateUserSpec(spec); len(v) != 0 {
		t.Fatalf("seed phase violates the user floor: %v", v)
	}

	cards := phaseCardsFromCatalog(cat)
	var card *struct {
		whenToUse  string
		categories []string
	}
	var b strings.Builder
	writeCatalog(&b, cards)
	rendered := b.String()

	for _, c := range cards {
		if c.Name == "bug-reproduction" {
			card = &struct {
				whenToUse  string
				categories []string
			}{c.WhenToUse, c.Categories}
		}
	}
	if card == nil {
		t.Fatal("bug-reproduction card absent from the advisor catalog projection")
	}
	if card.whenToUse == "" || len(card.categories) == 0 {
		t.Errorf("card metadata empty: when_to_use=%q categories=%v", card.whenToUse, card.categories)
	}
	if !strings.Contains(rendered, "bug-reproduction") {
		t.Error("bug-reproduction not rendered into the advisor SELECT catalog")
	}
	if !strings.Contains(rendered, "(bugfix)") && !strings.Contains(rendered, "bug-reproduction [evaluate") {
		t.Errorf("expected an enriched or at least selectable rendering; got:\n%s", rendered)
	}
}
