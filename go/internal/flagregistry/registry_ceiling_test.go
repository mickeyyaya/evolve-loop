package flagregistry

import "testing"

const FlagCeiling = 4

func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}

const LiveFeatureFlagCeiling = 0

func TestRegistry_LiveFeatureFlagCeiling(t *testing.T) {
	if got := len(LiveFeatureFlags()); got > LiveFeatureFlagCeiling {
		t.Errorf("len(LiveFeatureFlags) = %d exceeds LiveFeatureFlagCeiling=%d — "+
			"deprecate a flag to its replacement rather than raising the ceiling", got, LiveFeatureFlagCeiling)
	}
}
