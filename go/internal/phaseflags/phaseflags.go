// Package phaseflags centralizes the per-phase logic that assembles
// BridgeRequest.ExtraFlags from a profile JSON plus per-phase env-var
// overrides. v12.1 Capability 1 (plan-mode dispatch) introduced
// EVOLVE_<PHASE>_PERMISSION_MODE; all 6 phase runners (intent, scout,
// triage, tdd, build, audit) share this resolver instead of each
// copy-pasting the same five-line helper.
//
// Usage from a phase runner:
//
//	extraFlags := phaseflags.For("build").Resolve(profileDir, req.Env)
//
// Precedence for permission_mode:
//
//	reqEnv[EVOLVE_<PHASE>_PERMISSION_MODE] > profile.permission_mode > unset
//
// profile.extra_flags pass through unconditionally.
package phaseflags

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Phase is the resolver handle for a single phase. Construct with For.
// Phase values are cheap and safe to discard or reuse.
type Phase struct {
	name string
}

// For returns the resolver handle for the given phase name. Names use
// the canonical phase form ("build", "scout", "tdd-engineer"); hyphens
// are normalized to underscores when composing env-var keys so callers
// can pass agent names verbatim.
func For(name string) Phase { return Phase{name: name} }

// Name returns the phase name this resolver was constructed with.
func (p Phase) Name() string { return p.name }

// PermissionModeEnvKey returns the per-phase env-var key that overrides
// profile.permission_mode, e.g. EVOLVE_BUILD_PERMISSION_MODE for
// phase "build" and EVOLVE_TDD_ENGINEER_PERMISSION_MODE for
// phase "tdd-engineer".
func (p Phase) PermissionModeEnvKey() string {
	upper := strings.ReplaceAll(strings.ToUpper(p.name), "-", "_")
	return "EVOLVE_" + upper + "_PERMISSION_MODE"
}

// Resolve assembles BridgeRequest.ExtraFlags. It reads
// <profileDir>/<phase>.json for extra_flags + permission_mode, then
// applies the per-phase env override from reqEnv. Missing or malformed
// profile is non-fatal: the env-driven flag (if any) is still emitted.
//
// reqEnv may be nil. Lookups against a nil map return zero values per
// the standard Go map semantics, so the caller is not required to
// initialize an empty map before calling.
func (p Phase) Resolve(profileDir string, reqEnv map[string]string) []string {
	var profileFlags []string
	var profileMode string
	if loader := profiles.NewFromDir(profileDir); loader != nil {
		if prof, err := loader.Get(p.name); err == nil {
			profileFlags = prof.ExtraFlags
			profileMode = prof.PermissionMode
		}
	}
	mode := reqEnv[p.PermissionModeEnvKey()]
	if mode == "" {
		mode = profileMode
	}
	out := append([]string(nil), profileFlags...)
	if mode != "" {
		out = append(out, "--permission-mode", mode)
	}
	return out
}
