package llmroute

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// DefaultTriggers returns a copy of the trigger set a profile without cli_fallback_on_exit gets.
func DefaultTriggers() []int {
	return append([]int(nil), defaultFallbackOnExit...)
}

// AllowedDiscovered keeps discovered CLIs whose family allowed_clis permits; nil, empty or "all" permits every family.
func AllowedDiscovered(discovered []string, prof *profiles.Profile) []string {
	if prof == nil || len(prof.AllowedCLIs) == 0 {
		return discovered
	}
	allow := make(map[string]struct{}, len(prof.AllowedCLIs))
	for _, a := range prof.AllowedCLIs {
		a = strings.TrimSpace(a)
		if a == "all" {
			return discovered
		}
		allow[a] = struct{}{}
	}
	var out []string
	for _, d := range discovered {
		if _, ok := allow[Family(d)]; ok {
			out = append(out, d)
		}
	}
	return out
}

// ExcludeFamilies drops discovered drivers whose family is banned (workflow.universal_fallback_exclude).
func ExcludeFamilies(discovered, families []string) []string {
	if len(families) == 0 {
		return discovered
	}
	banned := make(map[string]struct{}, len(families))
	for _, f := range families {
		banned[strings.TrimSpace(f)] = struct{}{}
	}
	var out []string
	for _, d := range discovered {
		if _, hit := banned[Family(d)]; !hit {
			out = append(out, d)
		}
	}
	return out
}

// KnownDriver reports whether name is a registered bridge driver (cliBinaryFor).
func KnownDriver(name string) bool {
	_, ok := cliBinaryFor[name]
	return ok
}
