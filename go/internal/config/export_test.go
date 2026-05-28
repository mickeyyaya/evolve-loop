package config

// StaticSpinePhasesForTesting exposes the package-private staticSpinePhases
// set to external (package config_test) tests for the cross-package contract
// check against core.Phase*. Not part of the public API.
func StaticSpinePhasesForTesting() map[string]struct{} {
	out := make(map[string]struct{}, len(staticSpinePhases))
	for k, v := range staticSpinePhases {
		out[k] = v
	}
	return out
}
