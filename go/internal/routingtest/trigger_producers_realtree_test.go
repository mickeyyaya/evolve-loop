package routingtest

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestTheCheckedInRegistry_EveryTriggerFieldHasAProducer(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	registry := config.RegistryPath(root)
	cfg, _ := config.New().Load(registry, nil)
	builtin, err := phasespec.Load(registry)
	if err != nil {
		t.Fatalf("load the builtin registry: %v", err)
	}
	user, _, discoverWarnings := phasespec.DiscoverUserSpecsFromRoots(phasespec.Roots(root))
	catalog, mergeWarnings := builtin.Merge(user)
	for _, w := range append(discoverWarnings, mergeWarnings...) {
		t.Fatalf("the checked-in phases must load clean: %s", w)
	}
	phasespec.ApplyUserRouting(&cfg, user, builtin)
	if len(cfg.Triggers) == 0 {
		t.Fatal("no routing triggers loaded — the registry or the phase roots moved")
	}
	for _, v := range phasespec.UnproducedTriggerFields(cfg, catalog.All(), router.HarnessProduces) {
		t.Error(v)
	}
}
