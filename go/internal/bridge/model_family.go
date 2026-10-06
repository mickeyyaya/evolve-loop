package bridge

func ModelFamily(driverOrFamily string) string {
	if driverOrFamily == "" {
		return ""
	}
	for _, name := range []string{driverOrFamily, driverOrFamily + "-tmux"} {
		if m, err := loadManifestRaw(name); err == nil {
			return m.ModelFamily
		}
	}
	return ""
}
