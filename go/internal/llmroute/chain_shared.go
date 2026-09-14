package llmroute

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// DefaultTriggers returns a copy of the conservative trigger set every profile
// without cli_fallback_on_exit gets (defaultFallbackOnExit) — the exits that
// advance a chain: 80 REPL boot timeout, 81 artifact timeout, 85 escalate
// (quota / rejected model / unanswerable prompt), 124 command timeout, 127
// missing binary. The bridge-chain decorator falls back to it when a resolver
// hands it a plan with no triggers.
func DefaultTriggers() []int {
	return append([]int(nil), defaultFallbackOnExit...)
}

// AllowedDiscovered filters host-discovered CLIs by the profile's allowed_clis
// families: a nil profile or an empty list allows all; the "all" wildcard too
// (policy.allowedBaseSet convention). ONE filter for the runner's universal
// fallback and the bridge-chain decorator's plan resolver.
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

// ExcludeFamilies drops every driver whose family is in families — the
// operator's ban on a family as a LAST RESORT (policy
// workflow.universal_fallback_exclude, default ["agy"]: the 2026-06-07
// judgment that an error-prone model is the worst rescue choice). A banned
// family may still be a configured primary; only the discovered tail is filtered.
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
