package cliupdate

func Unwalled(walled func(family string) (string, bool)) func(family string) (bool, string) {
	return func(family string) (bool, string) {
		if action, isWalled := walled(family); isWalled {
			return false, "credential wall on the cli-health bench: " + action
		}
		return true, ""
	}
}
