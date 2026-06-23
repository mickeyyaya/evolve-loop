package bridge

import "testing"

// TestSetModelCatalogDirFn exercises the adapter's re-export of the
// EVOLVE_MODEL_CATALOG_DIR DI seam: it forwards the resolver to the underlying
// gobridge package. Calling it must not panic; a cleanup re-installs a benign
// resolver so the package global isn't left pointing at this test's closure.
func TestSetModelCatalogDirFn(t *testing.T) {
	t.Cleanup(func() { SetModelCatalogDirFn(func() string { return "" }) })
	SetModelCatalogDirFn(func() string { return "/adapter/catalog/dir" })
}
