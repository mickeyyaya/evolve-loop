package bridge

import "testing"

// TestSetModelCatalogDirFn pins the EVOLVE_MODEL_CATALOG_DIR → DI conversion:
// SetModelCatalogDirFn installs the resolver the overlay uses to locate
// model-catalog.json. Saves/restores the package global so other tests are
// unaffected.
func TestSetModelCatalogDirFn(t *testing.T) {
	orig := modelCatalogDirFn
	t.Cleanup(func() { modelCatalogDirFn = orig })

	SetModelCatalogDirFn(func() string { return "/custom/catalog/dir" })
	if got := modelCatalogDirFn(); got != "/custom/catalog/dir" {
		t.Errorf("after SetModelCatalogDirFn, modelCatalogDirFn() = %q, want /custom/catalog/dir", got)
	}
}
