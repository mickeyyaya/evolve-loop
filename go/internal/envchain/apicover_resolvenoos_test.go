package envchain

import "testing"

func TestResolveNoOS_PrecedenceNoOSFallback(t *testing.T) {
	if got := ResolveNoOS(SystemPromptReqEnvKey, map[string]string{SystemPromptReqEnvKey: "req"}, "prof", "def"); got != "req" {
		t.Errorf("reqEnv tier: got %q, want req", got)
	}
	if got := ResolveNoOS(SystemPromptReqEnvKey, nil, "prof", "def"); got != "prof" {
		t.Errorf("profile tier: got %q, want prof", got)
	}
	t.Setenv(SystemPromptReqEnvKey, "fromOS")
	if got := ResolveNoOS(SystemPromptReqEnvKey, nil, "", "def"); got != "def" {
		t.Errorf("no-OS-fallback: got %q, want def (must ignore the process env)", got)
	}
}

func TestSystemPromptReqEnvKey(t *testing.T) {
	if SystemPromptReqEnvKey != "EVOLVE_SYSTEM_PROMPT" {
		t.Errorf("SystemPromptReqEnvKey = %q, want EVOLVE_SYSTEM_PROMPT", SystemPromptReqEnvKey)
	}
}
