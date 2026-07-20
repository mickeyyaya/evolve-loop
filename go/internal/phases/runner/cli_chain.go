package runner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// DefaultDiscoverCLIsFn / DefaultUniversalFallback are the composition-root
// seams for the universal-fallback last-resort tier, set ONCE at boot
// (cmd_cycle.go) — the set-once package-var pattern PhaseBoundaryCheckpointer
// already uses, so the ~10 per-phase runner constructors need not each thread a
// bridge.Doctor discovery closure. Per-instance runner.Options fields override
// them (test injection). Zero values ⇒ the feature is inert (byte-identical to
// the pre-feature dispatch). Set-once at boot, read-only thereafter.
var (
	DefaultDiscoverCLIsFn    func() []string
	DefaultUniversalFallback bool
)

// allowedDiscovered filters system-discovered driver names (e.g. "agy-tmux")
// to those the phase profile's allowlist permits, for the universal-fallback
// last-resort tier. An empty/absent allowlist permits every family (the profile
// imposes no CLI restriction); otherwise a discovered driver survives only when
// its FAMILY (llmroute.Family: "agy-tmux" → "agy") is in AllowedCLIs. This
// preserves per-phase security pins (e.g. tester allowed_clis=["claude"]) —
// discovery can never route a phase to a family its operator forbade.
func allowedDiscovered(discovered []string, prof *profiles.Profile) []string {
	if prof == nil || len(prof.AllowedCLIs) == 0 {
		return discovered
	}
	allow := make(map[string]struct{}, len(prof.AllowedCLIs))
	for _, a := range prof.AllowedCLIs {
		a = strings.TrimSpace(a)
		if a == "all" { // the wildcard permits every family (policy.allowedBaseSet convention)
			return discovered
		}
		allow[a] = struct{}{}
	}
	var out []string
	for _, d := range discovered {
		if _, ok := allow[llmroute.Family(d)]; ok {
			out = append(out, d)
		}
	}
	return out
}

// cli_chain.go — dispatch-log helpers for the per-phase CLI fallback chain.
//
// The chain RESOLUTION (primary + fallback + triggers) and the capability
// PROBE now live in internal/llmroute (llmroute.Resolve / llmroute.Probe /
// Plan.TriggersFallback), so the runner makes a single unified call for CLI +
// model. What remains here are the two presentation helpers the runner uses
// when logging that chain's execution.

// sameCandidates reports whether two ordered candidate lists are
// element-for-element equal. Used to suppress the "probe reordered" log line
// when the capability probe left the chain unchanged.
func sameCandidates(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// joinAttempts formats the per-attempt dispatch log — one "cli=exit" token per
// attempt, separated by " -> " arrows so the chain reads left-to-right in the
// order candidates were tried. Used only when fallback actually fired
// (>1 attempt) so single-CLI phases stay quiet.
func joinAttempts(attempts []string) string {
	if len(attempts) == 0 {
		return ""
	}
	out := attempts[0]
	for _, a := range attempts[1:] {
		out += " -> " + a
	}
	return out
}

// FormatSkillOverlayLog renders the operator-visible line announcing the
// skill-overlay set a dispatch resolved — e.g.
// `[runner] phase=audit skill-overlays=[fable] (tier=deep)`. It is a pure
// formatter (no I/O); the per-attempt DispatchTiered closure emits it via
// log.Diag().Infof so operators/graders can see the fable persona fired for a
// given (phase, tier) without diffing the prompt file. An empty skill set is
// rendered explicitly as `skill-overlays=[]` so "no overlay resolved" is
// distinguishable from "the line never ran".
func FormatSkillOverlayLog(phase string, skills []string, tier string) string {
	return "[runner] phase=" + phase +
		" skill-overlays=[" + strings.Join(skills, ",") + "]" +
		" (tier=" + tier + ")"
}
