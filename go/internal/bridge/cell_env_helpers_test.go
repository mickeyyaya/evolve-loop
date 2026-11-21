package bridge

func fixedRoot(root string) func() (string, error) {
	return func() (string, error) { return root, nil }
}

func mustDispatchEnv(deps Deps, cfg *Config) []string {
	env, err := dispatchEnv(deps, cfg)
	if err != nil {
		panic(err)
	}
	return env
}

func mustProbeEnv(deps Deps, bin string) []string {
	env, err := probeEnv(deps, bin)
	if err != nil {
		panic(err)
	}
	return env
}

func mustPaneEnv(deps Deps, cfg *Config) []string {
	env, err := deps.paneEnv(cfg)
	if err != nil {
		panic(err)
	}
	return env
}
