package bridge

import "testing"

func TestSetModelCatalogDirFn(t *testing.T) {
	orig := modelCatalogDirFn
	t.Cleanup(func() { modelCatalogDirFn = orig })

	SetModelCatalogDirFn(func() string { return "/custom/catalog/dir" })
	if got := modelCatalogDirFn(); got != "/custom/catalog/dir" {
		t.Errorf("after SetModelCatalogDirFn, modelCatalogDirFn() = %q, want /custom/catalog/dir", got)
	}
}
