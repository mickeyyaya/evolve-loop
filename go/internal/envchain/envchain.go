// Package envchain encodes the canonical four-tier env-precedence
// chain used across evolve-loop. It is the single source of truth for
// "where does this value come from?" so the runner's permission
// resolution, bridge policy resolution, and any future config-knob site
// stay aligned.
//
// Precedence (highest to lowest):
//
//  1. reqEnv[key]   — per-request env (e.g., orchestrator-set on a single dispatch)
//  2. os.Getenv(key) — process env (operator shell)
//  3. profile        — value loaded from .evolve/profiles/<phase>.json
//  4. def            — package default
//
// Pattern: Chain of Responsibility (GoF). Each tier is consulted in
// order; the first non-empty value wins. An explicit empty string in
// reqEnv counts as absent (falls through) rather than masking lower
// tiers — operators who want to clear a value pin it explicitly to
// the canonical empty string for the relevant subsystem, never via the
// chain itself.
package envchain

import (
	"os"
	"strings"
)

// Resolve returns the first non-empty value from the four-tier chain.
// reqEnv may be nil (treated as empty); profile and def may be "".
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

// ResolveNoOS returns the first non-empty value from the three-tier chain
// (reqEnv → profile → def), skipping the os.Getenv tier. Use this for keys
// where the profile is the intended SSOT and process-env override is unwanted.
// reqEnv may be nil (treated as empty); profile and def may be "".
func ResolveNoOS(key string, reqEnv map[string]string, profile, def string) string {
	if v, ok := reqEnv[key]; ok && v != "" {
		return v
	}
	if profile != "" {
		return profile
	}
	return def
}

// PhaseEnvKey builds the canonical per-phase env-var name:
// "EVOLVE_<PHASE_UPPER>_<SUFFIX>". Hyphens in the phase name become
// underscores so "tdd-engineer" maps to "EVOLVE_TDD_ENGINEER_*".
//
// Examples:
//
//	PhaseEnvKey("build", "PERMISSION_MODE") → "EVOLVE_BUILD_PERMISSION_MODE"
//	PhaseEnvKey("tdd-engineer", "MODEL")    → "EVOLVE_TDD_ENGINEER_MODEL"
//	PhaseEnvKey("scout", "INTERACTIVE_POLICY") → "EVOLVE_SCOUT_INTERACTIVE_POLICY"
//
// Centralizing the transformation here prevents subtle drift between
// PERMISSION_MODE, MODEL, INTERACTIVE_POLICY, PLAN_INPUT, PLAN_OUTPUT
// and any future per-phase env knob.
func PhaseEnvKey(phase, suffix string) string {
	upper := strings.ReplaceAll(strings.ToUpper(phase), "-", "_")
	return "EVOLVE_" + upper + "_" + suffix
}
