package bridge

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var modelCatalogDirFn = func() string {
	layout := paths.ResolveFromEnv()
	pol, err := policy.Load(filepath.Join(layout.EvolveDir, "policy.json"))
	if err == nil {
		if dir := pol.BridgeConfig().CatalogDir; dir != "" {
			return dir
		}
	}
	return layout.EvolveDir
}

func SetModelCatalogDirFn(fn func() string) {
	modelCatalogDirFn = fn
}

func overlayManifestCatalog(m Manifest) Manifest {
	cat := loadCatalogCached()
	if cat.Empty() {
		return m
	}
	return applyCatalogTierMap(m, cat)
}

var (
	catalogMu  sync.Mutex
	catalogDir string
	catalogMod time.Time
	catalogVal modelcatalog.Catalog
)

func loadCatalogCached() modelcatalog.Catalog {
	dir := modelCatalogDirFn()
	fi, err := os.Stat(filepath.Join(dir, modelcatalog.FileName))

	catalogMu.Lock()
	defer catalogMu.Unlock()
	if err != nil {
		catalogDir, catalogMod, catalogVal = dir, time.Time{}, modelcatalog.Catalog{}
		return catalogVal
	}
	if dir == catalogDir && fi.ModTime().Equal(catalogMod) {
		return catalogVal
	}
	cat, _ := modelcatalog.Read(dir)
	catalogDir, catalogMod, catalogVal = dir, fi.ModTime(), cat
	return cat
}

func applyCatalogTierMap(m Manifest, cat modelcatalog.Catalog) Manifest {
	base := policy.BaseCLI(m.CLI)
	var merged map[string]string
	for _, tier := range modelcatalog.CanonicalTiers {
		live, ok := cat.DispatchModel(base, tier)
		if !ok {
			continue
		}
		if merged == nil {
			merged = make(map[string]string, len(m.ModelTierMap)+len(modelcatalog.CanonicalTiers))
			for k, v := range m.ModelTierMap {
				merged[k] = v
			}
		}
		merged[tier] = live
	}
	if merged == nil {
		return m
	}
	m.ModelTierMap = merged
	return m
}
