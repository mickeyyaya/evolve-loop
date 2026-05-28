package runner

// cli_chain.go — Workstream G1: per-agent CLI resolution + fallback chain.
//
// Pre-G, a phase pinned to ONE CLI via profile.cli; primary failure (REPL boot
// timeout / missing binary) = phase failure = cycle failure. Cycle 121's audit
// failure on codex-tmux exit=80 was the canonical signature.
//
// G1 adds two pieces to the resolution chain:
//
//  1. PER-AGENT env override — `EVOLVE_<AGENT>_CLI=claude-tmux` lets an operator
//     hot-swap a phase's CLI without editing the profile. Matches the existing
//     EVOLVE_<AGENT>_MODEL / EVOLVE_<AGENT>_PERMISSION_MODE / SYSTEM_PROMPT
//     conventions (see envchain.PhaseEnvKey).
//
//  2. FALLBACK CHAIN — profile.cli_fallback is the ordered list of alternates;
//     profile.cli_fallback_on_exit (default [80, 127]) enumerates which bridge
//     exit codes trigger fallback. A non-trigger exit (e.g. a real FAIL verdict)
//     still hard-fails the phase — the chain never silently routes a legitimate
//     model failure to a different CLI.
//
// Together: "any registered CLI can run any phase, and a CLI-level failure
// degrades to the next one instead of killing the cycle."

import (
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// cliBinaryFor maps a registered CLI driver name to the underlying binary
// the host needs on PATH. Used by the WS-G3 startup capability probe to
// skip candidates whose binary isn't installed — fast-fail in milliseconds
// instead of waiting for the bridge's ExitMissingBinary (127). Mirror of
// bridge.doctorBinaryFor (kept here so this leaf package doesn't depend
// on the bridge implementation).
var cliBinaryFor = map[string]string{
	"claude-p":    "claude",
	"claude-tmux": "claude",
	"codex":       "codex",
	"codex-tmux":  "codex",
	"agy":         "agy",
	"agy-tmux":    "agy",
	"ollama-tmux": "ollama",
}

// probeAvailableCLIChain returns a copy of chain with candidates whose
// binary isn't on PATH demoted to the end of the chain — that way an
// already-missing CLI doesn't block the cycle on its 60s boot timeout,
// but if ALL candidates are missing we still attempt the primary so the
// classifier sees a real ExitMissingBinary (not a silent skip).
//
// lookPath is the seam: production passes exec.LookPath; tests inject a
// closure that returns nil for "present" / error for "missing".
//
// The reorder is INTENTIONALLY not a hard drop: a user might have a CLI
// installed but its binary not yet on PATH at probe time (e.g., a freshly
// added Homebrew tap), and the bridge's later subprocess launch may still
// resolve the binary via a richer search. Demote rather than delete.
func probeAvailableCLIChain(chain cliChain, lookPath func(string) (string, error)) cliChain {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if len(chain.candidates) <= 1 {
		return chain // nothing to demote against
	}
	var available, missing []string
	for _, cli := range chain.candidates {
		bin := cliBinaryFor[cli]
		if bin == "" {
			// Unknown CLI name — keep its position (the chain.candidates
			// list was operator-authored; we don't second-guess unknown
			// names because LookupDriver might still resolve them).
			available = append(available, cli)
			continue
		}
		if _, err := lookPath(bin); err == nil {
			available = append(available, cli)
		} else {
			missing = append(missing, cli)
		}
	}
	if len(available) == 0 {
		return chain // all missing — fall back to the original order
	}
	return cliChain{
		primarySource: chain.primarySource,
		candidates:    append(available, missing...),
		triggers:      chain.triggers,
	}
}

// defaultFallbackOnExit is the conservative trigger set: ExitREPLBootTimeout
// (80) + ExitMissingBinary (127). Mirror of bridge/exitcodes.go (kept as
// integer literals here so this leaf package doesn't depend on bridge).
//
// Operators that want the more aggressive policy — add 81 (ExitArtifactTimeout),
// 2 (ExitSafetyGate), etc. — set profile.cli_fallback_on_exit per-agent.
var defaultFallbackOnExit = []int{80, 127}

// cliChain is the resolved per-phase dispatch plan: an ordered list of CLIs
// to try, the exit codes that promote to the next CLI, and a human label for
// the primary's source (env / profile / default) so logs can attribute it.
type cliChain struct {
	primarySource string // "env(EVOLVE_AUDITOR_CLI)" / "env(EVOLVE_CLI)" / "profile.auditor.cli" / "default"
	candidates    []string
	triggers      []int
}

// resolveCLIChain composes the dispatch plan for one phase invocation.
//
// Resolution order for the PRIMARY CLI (first candidate):
//
//  1. EVOLVE_<AGENT>_CLI (envchain.PhaseEnvKey) — per-agent, highest precedence.
//  2. EVOLVE_CLI                                — global fallback.
//  3. profile.CLI                               — on-disk per-agent config.
//  4. "claude-tmux"                             — final default.
//
// The fallback chain is the primary PLUS profile.CLIFallback (deduped against
// primary, preserving the operator's declared order). Triggers come from
// profile.CLIFallbackOnExit, defaulting to {80, 127} when unset.
//
// agentName is the canonical profile name (e.g. "auditor", "tdd-engineer") —
// same key used for per-agent env elsewhere in the runner.
func resolveCLIChain(agentName string, env map[string]string, prof *profiles.Profile) cliChain {
	// Step 1: pick the primary.
	perAgentKey := envchain.PhaseEnvKey(agentName, "CLI")
	primary := envchain.Resolve(perAgentKey, env, "", "")
	source := "env(" + perAgentKey + ")"
	if primary == "" {
		primary = envchain.Resolve("EVOLVE_CLI", env, "", "")
		source = "env(EVOLVE_CLI)"
	}
	if primary == "" && prof != nil {
		primary = prof.CLI
		source = "profile." + agentName + ".cli"
	}
	if primary == "" {
		primary = "claude-tmux"
		source = "default"
	}

	// Step 2: build the chain — primary first, then dedup'd fallback list.
	candidates := []string{primary}
	if prof != nil {
		seen := map[string]struct{}{primary: {}}
		for _, c := range prof.CLIFallback {
			c = trimSpace(c)
			if c == "" {
				continue
			}
			if _, dup := seen[c]; dup {
				continue
			}
			seen[c] = struct{}{}
			candidates = append(candidates, c)
		}
	}

	// Step 3: triggers — profile first, then conservative default.
	triggers := defaultFallbackOnExit
	if prof != nil && len(prof.CLIFallbackOnExit) > 0 {
		triggers = append([]int(nil), prof.CLIFallbackOnExit...)
	}

	return cliChain{
		primarySource: source,
		candidates:    candidates,
		triggers:      triggers,
	}
}

// triggersFallback reports whether exitCode should advance the chain. A
// non-trigger exit (or zero) breaks the loop — either the phase succeeded
// or it produced a legitimate FAIL that the dispatcher's classifier should
// see, not a CLI bug we should retry around.
func (c cliChain) triggersFallback(exitCode int) bool {
	for _, t := range c.triggers {
		if t == exitCode {
			return true
		}
	}
	return false
}

// sameCandidates reports whether two ordered candidate lists are
// element-for-element equal. Used to suppress the "probe reordered" log
// when the chain was unchanged.
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

// joinAttempts formats the per-attempt log line — one "cli=exit" token per
// attempt, separated by " -> " arrows so the chain reads as a left-to-right
// dispatch story (matches the order candidates were tried). Used only when
// fallback actually fired (>1 attempt) so single-CLI phases stay quiet.
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

// trimSpace is a tiny local helper so this leaf file doesn't import strings
// just for one call. Trims leading + trailing ASCII whitespace + newlines.
func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j {
		c := s[i]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		i++
	}
	for j > i {
		c := s[j-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		j--
	}
	return s[i:j]
}
