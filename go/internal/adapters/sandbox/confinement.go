package sandbox

import "strings"

// confinement.go — the single source of truth for the inner-OS-sandbox
// confinement decision, consumed by BOTH the bridge launch path
// (internal/bridge) and the host preflight (internal/preflight).
//
// Before this file, the two decisions it owns were duplicated:
//
//   - "is this a nested LLM-CLI sandbox session?" was detected by two different
//     heuristics — preflight read CLAUDECODE; the bridge read
//     CLAUDE_CODE_ENTRYPOINT / CLAUDE_CODE_SESSION_ID.
//   - "should the inner OS sandbox wrap this launch?" was decided twice — the
//     bridge gated it on binary-presence with a nested skip wired ONLY for
//     auto mode (so EVOLVE_SANDBOX=on still wrapped under nested macOS and hung
//     the claude REPL boot, exit=80), while preflight computed InnerSandbox
//     from its own capability signal.
//
// Centralizing both here removes the duplication and makes the two consumers
// agree by construction.

// DetectNested reports a possible nested LLM CLI session, not measured sandbox
// capability or proof of outer confinement. Known session signals count, EXCEPT that
// CLAUDECODE_TYPE=host marks the top-level host process (which is not
// nested-under-another-sandbox and so can confine its children).
//
// getenv is injected (os.Getenv in production, a map lookup in tests / on the
// bridge's request-local env chain).
func DetectNested(getenv func(string) string) bool {
	if strings.Contains(strings.ToLower(getenv("CLAUDECODE_TYPE")), "host") {
		return false
	}
	for _, k := range []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SESSION_ID"} {
		if getenv(k) != "" {
			return true
		}
	}
	path := getenv("PATH")
	if strings.Contains(path, "/codex.system/bootstrap/") || strings.Contains(path, "/.codex/tmp/arg0/") {
		return true
	}
	return false
}

// ConfinementSatisfied is the single home of the fail-closed confinement
// decision for a source-writing phase whose profile REQUIRES sandboxing but
// whose inner wrap did not apply (the Specification the bridge evaluates and
// preflight displays — docs/architecture/sandbox-confinement-ssot.md chose
// this package as the seam). Three cells:
//
//	mode == "off" → explicit host opt-out; optOut=true, never verified confinement.
//	otherwise     → NOT satisfied, including unverified nested environments.
//
// A sampled parent-directory write denial cannot prove the requested profile's
// read and write policy; the optional preflight canary does not waive this gate.
func ConfinementSatisfied(nested bool, mode string) (ok, optOut bool, reason string) {
	if strings.TrimSpace(mode) == "off" {
		return true, true, "EVOLVE_SANDBOX=off: host opt-out honoured — the phase runs UNCONFINED despite the sandbox requirement"
	}
	if nested {
		return false, false, "nested LLM-CLI session: inner sandbox unavailable; outer session is UNVERIFIED and does not satisfy mandatory profile restrictions"
	}
	return false, false, "inner sandbox required but unavailable"
}

// ShouldWrap decides whether to attempt the actual profile wrapper. A measured
// capability outranks session hints; an unchecked nested hint remains refused.
// Success here does not authorize an unwrapped launch or attest an outer sandbox.
func ShouldWrap(nested bool, probe ProbeResult) (bool, string) {
	switch probe.OS {
	case "darwin", "linux":
	default:
		return false, "Unsupported OS: " + probe.OS + " — no sandbox implementation"
	}
	if !probe.Available {
		reason := probe.Reason
		if reason == "" {
			reason = "sandbox binary not available"
		}
		return false, reason
	}
	if probe.CapabilityChecked {
		if probe.Capable {
			return true, "measured: sandbox applies; inner profile confinement enabled"
		}
		reason := probe.Reason
		if reason == "" {
			reason = "sandbox application failed"
		}
		return false, "measured: " + reason + " — inner sandbox unavailable"
	}
	if nested {
		return false, "nested LLM-CLI hint with unmeasured capability; outer confinement UNVERIFIED"
	}
	return true, "standalone host with available sandbox binary: inner confinement enabled"
}
