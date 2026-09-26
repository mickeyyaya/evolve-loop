package phasespec

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// ClampDiscoveredSpecs forces discovered specs optional and keeps writes_source only if sandboxedProfile verifies it; nil fails closed.
func ClampDiscoveredSpecs(specs []PhaseSpec, sandboxedProfile func(profileName string) bool) ([]PhaseSpec, []string) {
	var warnings []string
	out := make([]PhaseSpec, len(specs))
	for i, s := range specs {
		if !s.Optional {
			warnings = append(warnings, fmt.Sprintf("user phase %s claims optional:false — FORCED optional:true (registrar parity; a user phase cannot displace the spine floor)", s.Name))
			s.Optional = true
		}
		if s.WritesSource {
			// The runner resolves the dispatch profile by this same name.
			profile := strings.TrimPrefix(s.AgentName(), "evolve-")
			if sandboxedProfile == nil || !sandboxedProfile(profile) {
				warnings = append(warnings, fmt.Sprintf("user phase %s claims writes_source:true but its dispatch profile %q is not a sandbox-enabled registrar-minted profile — writes_source STRIPPED (worktree-write eligibility denied; ADR-0073 on-disk-spec trust gap)", s.Name, profile))
				s.WritesSource = false
			}
		}
		out[i] = s
	}
	return out, warnings
}

// SandboxedProfilePredicate reports whether <profilesDir>/<name>.json is a sandbox-enabled, repo-writable profile.
func SandboxedProfilePredicate(profilesDir string) func(name string) bool {
	loader := profiles.NewFromDir(profilesDir)
	return func(name string) bool {
		p, err := loader.Get(name)
		if err != nil {
			return false
		}
		// Mirrors the registrar's writer clamp: a read-only sandbox grants eligibility without write capability.
		return p.Sandbox != nil && p.Sandbox.Enabled && !p.Sandbox.ReadOnlyRepo
	}
}
