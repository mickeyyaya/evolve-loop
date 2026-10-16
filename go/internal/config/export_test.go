package config

func StaticSpinePhasesForTesting() map[string]struct{} {
	out := make(map[string]struct{}, len(staticSpinePhases))
	for k, v := range staticSpinePhases {
		out[k] = v
	}
	return out
}
