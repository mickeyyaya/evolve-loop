package modelcatalog

import "path/filepath"

// ShadowFileName is the catalog the refresh pipeline writes under
// catalog.refresh_stage="shadow": the full live pipeline runs and lands here,
// never in FileName — dispatch (which reads only FileName via the manifest
// overlay) is byte-identical to stage "off". Soaking this file against the
// live catalog is what earns the flip to "enforce".
const ShadowFileName = "model-catalog.shadow.json"

// shadowPathFor returns the shadow catalog path under evolveDir.
func shadowPathFor(evolveDir string) string {
	return filepath.Join(evolveDir, ShadowFileName)
}

// ReadShadow loads the shadow catalog. Same contract as Read: a missing file
// is a zero catalog (always stale → first shadow run refreshes), malformed
// JSON is an error.
func ReadShadow(evolveDir string) (Catalog, error) {
	return readCatalogFile(shadowPathFor(evolveDir))
}

// WriteShadow persists the shadow catalog atomically. Unlike Write it rotates
// no .prev copy: retention exists to protect the hand-authored live catalog,
// and the shadow file is disposable pipeline output.
func WriteShadow(evolveDir string, c Catalog) error {
	return writeCatalogFile(evolveDir, ShadowFileName, c)
}
