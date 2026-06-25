package sandbox

import "strings"

// confinement.go — the single source of truth for the inner-OS-sandbox
// confinement decision, consumed by BOTH the bridge launch path
// (internal/bridge) and the host preflight (internal/preflight).
//
// Before this file, the two decisions it owns were duplicated:
//
//   - "is this a nested-Claude session?" was detected by two different
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

// DetectNested reports whether we are running inside an outer Claude Code
// session. It is the single nested-Claude heuristic: any of the known signals
// counts, EXCEPT that CLAUDECODE_TYPE=host marks the top-level host process
// (which is not nested-under-another-sandbox and so can confine its children).
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
	return false
}

// ShouldWrap is the single wrap-policy decision: should a source-writing phase
// launch be wrapped in the inner OS sandbox, given the nested-Claude signal and
// the host probe? It returns (wrap, reason) where reason is always non-empty so
// callers can surface it (the bridge WARNs it for EVOLVE_SANDBOX=on; preflight
// records it).
//
// Policy: wrap IFF the OS is supported AND a sandbox binary is available AND we
// are NOT nested. The nested exclusion is universal (not OS-specific and not
// mode-specific): under an outer Claude Code session the inner sandbox is both
// redundant (the outer session already imposes OS sandbox + Tier-1 hooks) and,
// on macOS, non-functional (sandbox_apply() returns EPERM and the REPL never
// boots). When !nested, capability collapses to availability — the host can't
// be a working-but-nested case — so this single predicate matches preflight's
// richer ExpectedToWork report in every reachable cell.
func ShouldWrap(nested bool, probe ProbeResult) (bool, string) {
	switch probe.OS {
	case "darwin", "linux":
		// supported
	default:
		return false, "no sandbox impl for GOOS=" + probe.OS
	}
	if !probe.Available {
		reason := probe.Reason
		if reason == "" {
			reason = "sandbox binary not available"
		}
		return false, reason
	}
	if nested {
		return false, "nested-Claude: outer Claude Code OS sandbox + Tier-1 hooks already confine; inner sandbox redundant (and on macOS sandbox_apply() returns EPERM, hanging REPL boot)"
	}
	// Subtractive capability gate: a MEASURED-incapable sandbox (binary present
	// but sandbox_apply fails — e.g. a broken/SIP-weird standalone host) must NOT
	// be wrapped, because wrapping it hangs the REPL boot (exit 80). This is the
	// only behavioral delta vs the legacy guess, and it is strictly subtractive:
	// it can only demote a would-be wrap to skip (the nested skip above already
	// guarantees capability never PROMOTES a nested skip to a wrap), so
	// new_wrap ⟹ old_wrap. An UNCHECKED probe (CapabilityChecked=false) keeps
	// the legacy behavior byte-identical.
	if probe.CapabilityChecked && !probe.Capable {
		reason := probe.Reason
		if reason == "" {
			reason = "sandbox binary present but sandbox_apply failed"
		}
		return false, "measured: " + reason + " — inner sandbox not applicable here"
	}
	return true, "standalone host with available sandbox binary: inner confinement enabled"
}
