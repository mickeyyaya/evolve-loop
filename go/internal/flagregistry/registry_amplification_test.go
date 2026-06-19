package flagregistry

import (
	"strings"
	"testing"
)

// Observer and inactivity tuning moved to policy.ObserverPolicy. Keeping any
// of the retired names in the registry would recreate a second configuration
// surface.
func TestAmplify_ObserverInactivityFlagsRetired(t *testing.T) {
	for _, name := range []string{
		"EVOLVE_INACTIVITY_DISABLE",
		"EVOLVE_INACTIVITY_GRACE_S",
		"EVOLVE_INACTIVITY_POLL_S",
		"EVOLVE_INACTIVITY_THRESHOLD_S",
		"EVOLVE_INACTIVITY_WARN_PCT",
		"EVOLVE_OBSERVER_AUTOSPAWN",
		"EVOLVE_OBSERVER_ENABLED",
		"EVOLVE_OBSERVER_ENFORCE",
		"EVOLVE_OBSERVER_EOF_GRACE_S",
		"EVOLVE_OBSERVER_NUDGE_BODY",
		"EVOLVE_OBSERVER_NUDGE_S",
		"EVOLVE_OBSERVER_POLL_S",
		"EVOLVE_OBSERVER_STALL_S",
	} {
		if _, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q) succeeded; retired observer/inactivity flag must be absent", name)
		}
	}
}

// Future observer flags that are deliberately registered must retain cluster
// metadata for generated documentation.
func TestAmplify_AllActiveObserverFlagsHaveCluster(t *testing.T) {
	for _, f := range All {
		if !strings.Contains(f.Name, "OBSERVER") || f.Status != StatusActive {
			continue
		}
		if f.Cluster == "" {
			t.Errorf("%s: StatusActive Observer flag has empty Cluster", f.Name)
		}
	}
}
