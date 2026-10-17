package bridge

func ProcessEnv(bin string) []string {
	return driverEnv(Deps{}, familyEnv(bin))
}

func familyEnv(bin string) map[string]string {
	r, err := Resolve(DefaultDriverSet(), bin)
	if err != nil {
		return nil
	}
	return r.Manifest.DefaultEnv
}
