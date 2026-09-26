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

// modelCatalogDirFn resolves the model-catalog.json directory: policy's bridge CatalogDir, else the project's .evolve.
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

// SetModelCatalogDirFn replaces the catalog directory resolver without touching the process environment.
func SetModelCatalogDirFn(fn func() string) {
	modelCatalogDirFn = fn
}

// overlayManifestCatalog merges live catalog tier models over m.ModelTierMap; a missing catalog never blocks a load.
func overlayManifestCatalog(m Manifest) Manifest {
	cat := loadCatalogCached()
	if cat.Empty() {
		return m
	}
	return applyCatalogTierMap(m, cat)
}

// LoadManifest runs per dispatch, so the catalog is re-parsed only when its (dir, mtime) changes. A cycle-start
// refresh rewrites the file, so freshness holds without a parse per load.
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
		// Remember the dir so a later write to it is seen by the mtime check.
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

// applyCatalogTierMap merges each canonical tier's live model over m.ModelTierMap. With no live hit it returns m
// unchanged, so dispatch stays byte-identical to the embedded manifest.
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
