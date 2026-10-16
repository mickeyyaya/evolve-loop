// Package envchain encodes the canonical four-tier env-precedence chain used
// across evolve-loop.
package envchain

import (
	"os"
	"strings"
)

func Resolve(key string, reqEnv map[string]string, profile, def string) string {
	if v, ok := reqEnv[key]; ok && v != "" {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	if profile != "" {
		return profile
	}
	return def
}

func ResolveNoOS(key string, reqEnv map[string]string, profile, def string) string {
	if v, ok := reqEnv[key]; ok && v != "" {
		return v
	}
	if profile != "" {
		return profile
	}
	return def
}

func PhaseEnvKey(phase, suffix string) string {
	upper := strings.ReplaceAll(strings.ToUpper(phase), "-", "_")
	return "EVOLVE_" + upper + "_" + suffix
}
