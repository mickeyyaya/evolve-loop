package bridge

import "testing"

func TestSetModelCatalogDirFn(t *testing.T) {
	t.Cleanup(func() { SetModelCatalogDirFn(func() string { return "" }) })
	SetModelCatalogDirFn(func() string { return "/adapter/catalog/dir" })
}
