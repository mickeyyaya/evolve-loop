package config

import "strings"

func applyDeepBuildRisk(cfg *RoutingConfig, classes map[string][]string) {
	cleaned := map[string][]string{}
	for class, markers := range classes {
		for _, marker := range markers {
			if marker = strings.ToLower(strings.TrimSpace(marker)); marker != "" {
				cleaned[class] = append(cleaned[class], marker)
			}
		}
	}
	if len(cleaned) > 0 {
		cfg.DeepBuildRisk = cleaned
	}
}
