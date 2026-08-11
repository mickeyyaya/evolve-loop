package phasespec

// clamp.go — the load-time registrar-equivalent clamp for DISCOVERED user
// specs (ADR-0073 security review, Finding 1 downstream trace; inbox
// loadtime-userspec-registrar-clamp).
//
// Two paths admit a user phase into the catalog. The registrar mint path
// (phaseregistrar.Register) normalizes and clamps: Optional forced true, and
// writes_source forces a sandbox-ENABLED dispatch profile into existence —
// the invariant `writes_source ⟹ sandboxed dispatch profile` holds by
// CONSTRUCTION. The discovery path (DiscoverUserSpecsFromRoots ← any on-disk
// .evolve/phases/*/phase.json: smuggled residue, git merge, operator typo)
// ran NO clamp: a spec claiming writes_source:true got worktree-write
// eligibility (core.worktreePhase reads spec.WritesSource) with no sandbox
// anywhere. This clamp closes that path by VERIFICATION: writes_source
// survives only when the spec's dispatch profile — the SAME on-disk profile
// the runner resolves at dispatch — exists with sandbox enabled.
//
// The two enforcement styles are pinned against each other by
// phaseregistrar's cross-binding test: a Register-minted writes_source phase
// must pass SandboxedProfilePredicate over its own persisted profile.

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// ClampDiscoveredSpecs applies the registrar-parity normalization to
// discovered specs: Optional is FORCED true (a user phase can never satisfy
// the build→audit→ship spine floor — NB this converts the previous
// skip-on-validation behavior for non-evaluate optional:false specs into
// admit-as-optional, exactly the registrar's own force-then-validate
// posture), and writes_source is kept only when sandboxedProfile verifies
// the spec's dispatch profile (runner convention: profile name = AgentName
// with the "evolve-" prefix stripped; nil predicate fails closed). Every
// change is loud — a silent strip would hide the one signal an operator
// needs to notice a smuggled writer.
func ClampDiscoveredSpecs(specs []PhaseSpec, sandboxedProfile func(profileName string) bool) ([]PhaseSpec, []string) {
	var warnings []string
	out := make([]PhaseSpec, len(specs))
	for i, s := range specs {
		if !s.Optional {
			warnings = append(warnings, fmt.Sprintf("user phase %s claims optional:false — FORCED optional:true (registrar parity; a user phase cannot displace the spine floor)", s.Name))
			s.Optional = true
		}
		if s.WritesSource {
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

// SandboxedProfilePredicate returns the writes_source verification predicate
// over a profiles directory: true iff <dir>/<name>.json parses as a profile
// with sandbox enabled. Reading the ON-DISK profile is correct here — it is
// the same file the runner resolves at dispatch (dispatch parity, not a
// repo-contract scan), and the registrar persists it sandbox-enabled for
// every writes_source mint.
func SandboxedProfilePredicate(profilesDir string) func(name string) bool {
	loader := profiles.NewFromDir(profilesDir)
	return func(name string) bool {
		p, err := loader.Get(name)
		if err != nil {
			return false
		}
		// Mirror the registrar's writer clamp exactly (Enabled=true AND
		// ReadOnlyRepo=false — "a writer needs to write"): a read-only sandbox
		// profile grants no write capability, so keeping eligibility for it
		// would only produce confusing mid-cycle write failures.
		return p.Sandbox != nil && p.Sandbox.Enabled && !p.Sandbox.ReadOnlyRepo
	}
}
