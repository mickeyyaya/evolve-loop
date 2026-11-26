package bridge

func probeEnv(deps Deps, bin string) ([]string, error) {
	env, _, err := deps.cellEnv(&Config{CLI: bin, Agent: "probe", Realization: familyRealization(bin)})
	return env, err
}

func familyRealization(bin string) Realization {
	r, err := Resolve(DefaultDriverSet(), bin)
	if err != nil {
		return Realization{}
	}
	return Realization{Env: r.Manifest.DefaultEnv, EnvPassthrough: r.Manifest.EnvPassthrough}
}
