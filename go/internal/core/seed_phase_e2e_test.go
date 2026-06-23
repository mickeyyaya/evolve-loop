package core

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// seedRepoRoot walks up from this test file to the repo root (the dir
// containing .evolve/phases), following the usercatalog_research_test.go
// precedent of proving config-only claims against the REAL repo files.
func seedRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	// .../go/internal/core/seed_phase_e2e_test.go → up 4 to repo root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// TestSeedPhase_BugReproductionReachesAdvisorCatalog is the ADR-0038 end-to-end
// proof: a user phase living as pure config under .evolve/phases/ flows from
// the real merged catalog into an enriched advisor card, so the advisor can
// make an informed SELECT on bugfix cycles. (The original ADR-0038 seed was
// named reproduce-bug; the two-tier naming rule renamed it bug-reproduction.)
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
