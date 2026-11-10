package bridge

import "maps"

func (m Manifest) TierPins() map[string]string {
	return maps.Clone(m.tierPins)
}

func declaredTierPins(m Manifest) map[string]string {
	pins := make(map[string]string, len(m.ModelTierPins))
	for _, tier := range m.ModelTierPins {
		if model := m.ModelTierMap[tier]; model != "" {
			pins[tier] = model
		}
	}
	return pins
}
