package core

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestAdvisorCatalog_ReturnsTheCardsAndOnDemandNamesTheCycleUses(t *testing.T) {
	t.Parallel()
	cat := phasespec.Catalog{}
	cards, onDemand := AdvisorCatalog(cat)
	if !reflect.DeepEqual(cards, phaseCardsFromCatalog(cat)) || !reflect.DeepEqual(onDemand, onDemandCatalogNames(cat)) {
		t.Fatalf("AdvisorCatalog diverges from the cycle's own builders: cards=%v onDemand=%v", cards, onDemand)
	}
}
