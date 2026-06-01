package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// catalog_overlay.go wires the live model catalog (internal/modelcatalog) into
// manifest loading. At dispatch a phase's abstract tier (fast|balanced|deep) is
// translated to a concrete model via Manifest.ModelTierMap; this overlay lets a
// LIVE-queried catalog entry override that translation, so dispatch tracks the
// models the CLI actually offers rather than the embedded (possibly stale) map.
//
// Safety: only Source=="live" catalog entries override (the DispatchModel gate),
// and a missing/empty catalog returns the manifest unchanged — so this is
// byte-identical to pre-catalog behavior until a live catalog is written.

// modelCatalogDir resolves the directory holding model-catalog.json.
// EVOLVE_MODEL_CATALOG_DIR overrides (test seam); otherwise
// <EVOLVE_PROJECT_ROOT|.>/.evolve.
func modelCatalogDir() string {
	if d := os.Getenv("EVOLVE_MODEL_CATALOG_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("EVOLVE_PROJECT_ROOT"), ".evolve")
}

// overlayManifestCatalog merges live catalog tier models over m.ModelTierMap.
// A missing/empty/unreadable catalog leaves m unchanged (never blocks a load).
func overlayManifestCatalog(m Manifest) Manifest {
	cat := loadCatalogCached()
	if cat.Empty() {
		return m
	}
	return applyCatalogTierMap(m, cat)
}

// catalog memoization: LoadManifest is on the per-dispatch path, so the catalog
// is read at most once per (dir, mtime). A stat runs every call (cheap); the
// JSON is re-parsed only when the file changes — which is exactly what a
// cycle-start refresh does (it rewrites the file → new mtime), so freshness is
// preserved without a parse on every manifest load.
var (
	catalogMu  sync.Mutex
	catalogDir string
	catalogMod time.Time
	catalogVal modelcatalog.Catalog
)

func loadCatalogCached() modelcatalog.Catalog {
	dir := modelCatalogDir()
	fi, err := os.Stat(filepath.Join(dir, modelcatalog.FileName))

	catalogMu.Lock()
	defer catalogMu.Unlock()
	if err != nil {
		// No file (or unreadable) → empty catalog; remember the dir so a later
		// write to it is detected by the mtime check below.
		catalogDir, catalogMod, catalogVal = dir, time.Time{}, modelcatalog.Catalog{}
		return catalogVal
	}
	if dir == catalogDir && fi.ModTime().Equal(catalogMod) {
		return catalogVal // unchanged since last read
	}
	cat, _ := modelcatalog.Read(dir)
	catalogDir, catalogMod, catalogVal = dir, fi.ModTime(), cat
	return cat
}

// applyCatalogTierMap is the pure overlay: returns m with each canonical tier's
// LIVE model merged over m.ModelTierMap. When no live entry hits for this CLI it
// returns m unchanged (same ModelTierMap) — the byte-identical property.
func applyCatalogTierMap(m Manifest, cat modelcatalog.Catalog) Manifest {
	base := baseCLIName(m.CLI)
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

// baseCLIName strips driver suffixes (-tmux, -p) to the provider family used as
// the catalog key (claude-tmux → claude).
func baseCLIName(cli string) string {
	for _, suffix := range []string{"-tmux", "-p"} {
		if strings.HasSuffix(cli, suffix) {
			return strings.TrimSuffix(cli, suffix)
		}
	}
	return cli
}
