package modelcatalog

import "path/filepath"

const ShadowFileName = "model-catalog.shadow.json"

func shadowPathFor(evolveDir string) string {
	return filepath.Join(evolveDir, ShadowFileName)
}

func ReadShadow(evolveDir string) (Catalog, error) {
	return readCatalogFile(shadowPathFor(evolveDir))
}

func WriteShadow(evolveDir string, c Catalog) error {
	return writeCatalogFile(evolveDir, ShadowFileName, c)
}
