package envchain

import "testing"

// TestResolveNoOS_PrecedenceNoOSFallback pins the SSOT contract for the global
// system-prompt key (added by the EVOLVE_SYSTEM_PROMPT flag-reduction): reqEnv
// wins, then profile, then default — and critically there is NO os.Getenv tier,
// so a process-env value can never override the profile SSOT. Also names
// SystemPromptReqEnvKey (apicover same-pkg coverage).
func TestResolveNoOS_PrecedenceNoOSFallback(t *testing.T) {
	if got := ResolveNoOS(SystemPromptReqEnvKey, map[string]string{SystemPromptReqEnvKey: "req"}, "prof", "def"); got != "req" {
		t.Errorf("reqEnv tier: got %q, want req", got)
	}
	if got := ResolveNoOS(SystemPromptReqEnvKey, nil, "prof", "def"); got != "prof" {
		t.Errorf("profile tier: got %q, want prof", got)
	}
	// Default when reqEnv + profile empty, and the process env is explicitly set —
	// ResolveNoOS must NOT fall back to os.Getenv (that's the whole point).
	t.Setenv(SystemPromptReqEnvKey, "fromOS")
	if got := ResolveNoOS(SystemPromptReqEnvKey, nil, "", "def"); got != "def" {
		t.Errorf("no-OS-fallback: got %q, want def (must ignore the process env)", got)
	}
}

// TestSystemPromptReqEnvKey pins the split-const key value.
func TestSystemPromptReqEnvKey(t *testing.T) {
	if SystemPromptReqEnvKey != "EVOLVE_SYSTEM_PROMPT" {
		t.Errorf("SystemPromptReqEnvKey = %q, want EVOLVE_SYSTEM_PROMPT", SystemPromptReqEnvKey)
	}
}
