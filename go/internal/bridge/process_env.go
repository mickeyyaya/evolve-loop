package bridge

import "os"

func ProcessEnv(bin string) []string {
	r := familyRealization(bin)
	return hermeticEnv(os.Environ(), r.EnvPassthrough, r.Env, nil)
}

func probeEnv(deps Deps, bin string) []string {
	env, _ := deps.cellEnv(&Config{CLI: bin, Agent: "probe", Realization: familyRealization(bin)})
	return env
}

func familyRealization(bin string) Realization {
	r, err := Resolve(DefaultDriverSet(), bin)
	if err != nil {
		return Realization{}
	}
	return Realization{Env: r.Manifest.DefaultEnv, EnvPassthrough: r.Manifest.EnvPassthrough}
}
