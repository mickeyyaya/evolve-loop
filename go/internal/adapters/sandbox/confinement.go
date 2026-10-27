package sandbox

import "strings"

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

func ConfinementSatisfied(nested bool, mode string) (ok, optOut bool, reason string) {
	if strings.TrimSpace(mode) == "off" {
		return true, true, "--sandbox-off: host opt-out honoured — the phase runs UNCONFINED despite the sandbox requirement"
	}
	if nested {
		return false, false, "nested LLM-CLI session: inner sandbox unavailable; outer session is UNVERIFIED and does not satisfy mandatory profile restrictions"
	}
	return false, false, "inner sandbox required but unavailable"
}

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
