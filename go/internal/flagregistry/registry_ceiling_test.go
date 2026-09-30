package flagregistry

import "testing"

// FlagCeiling is the one-way upper bound on len(All).
const FlagCeiling = 26

func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}

// LiveFeatureFlagCeiling is the one-way upper bound on len(LiveFeatureFlags()).
const LiveFeatureFlagCeiling = 11

func TestRegistry_LiveFeatureFlagCeiling(t *testing.T) {
	if got := len(LiveFeatureFlags()); got > LiveFeatureFlagCeiling {
		t.Errorf("len(LiveFeatureFlags) = %d exceeds LiveFeatureFlagCeiling=%d — "+
			"deprecate a flag to its replacement rather than raising the ceiling", got, LiveFeatureFlagCeiling)
	}
}
