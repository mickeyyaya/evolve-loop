package ciparity

var RequiredComposedGates = []string{"compile", "test", "acs", "apicover"}

func MissingComposedGates(results map[string]string) []string {
	var missing []string
	for _, g := range RequiredComposedGates {
		if results[g] != "pass" {
			missing = append(missing, g)
		}
	}
	return missing
}

type GateOutcome struct {
	Status string
	Tail   string
}

func GateStatuses(results map[string]GateOutcome) map[string]string {
	statuses := make(map[string]string, len(results))
	for gate, outcome := range results {
		statuses[gate] = outcome.Status
	}
	return statuses
}
