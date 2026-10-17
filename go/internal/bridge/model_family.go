package bridge

func ModelFamily(driverOrFamily string) string {
	if driverOrFamily == "" {
		return ""
	}
	if m, err := loadManifestRaw(driverOrFamily); err == nil {
		return m.ModelFamily
	}
	if r, err := Resolve(DefaultDriverSet().WithLoader(loadManifestRaw), driverOrFamily); err == nil {
		return r.Manifest.ModelFamily
	}
	return ""
}
