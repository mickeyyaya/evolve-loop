package bridge

import "github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"

func UsageModelMatches(cli, word, model string) bool {
	m, err := LoadManifest(cli)
	if err != nil {
		return false
	}
	spec := m.usageWindowSpec()
	return spec != nil && spec.ModelMatches(word, model)
}

func UsageModelFor(cli, word string) (string, bool) {
	m, err := LoadManifest(cli)
	if err != nil {
		return "", false
	}
	return m.usageModelFor(word)
}

func UsageModelMapped(family, word string) bool {
	r, err := Resolve(DefaultDriverSet(), family)
	if err != nil {
		return false
	}
	_, ok := r.Manifest.usageModelFor(word)
	return ok
}

func (m Manifest) usageModelFor(word string) (string, bool) {
	spec := m.usageWindowSpec()
	if spec == nil {
		return "", false
	}
	for _, tier := range modelcatalog.CanonicalTiers {
		if model := m.ModelTierMap[tier]; spec.ModelMatches(word, model) {
			return model, true
		}
	}
	return "", false
}
