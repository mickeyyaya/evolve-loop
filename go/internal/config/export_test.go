package config

// StaticSpinePhasesForTesting returns a copy of staticSpinePhases for the external contract test.
func StaticSpinePhasesForTesting() map[string]struct{} {
	out := make(map[string]struct{}, len(staticSpinePhases))
	for k, v := range staticSpinePhases {
		out[k] = v
	}
	return out
}
