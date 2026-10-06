package bridge

func ProcessEnv(bin string) []string {
	return driverEnv(Deps{}, familyEnv(bin))
}

func familyEnv(bin string) map[string]string {
	m, err := LoadManifest(bin + "-tmux")
	if err != nil {
		return nil
	}
	return m.DefaultEnv
}
