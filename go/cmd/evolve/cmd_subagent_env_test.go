package main

import "testing"

func TestReadSubagentRunFlags_Defaults(t *testing.T) {
	for _, k := range []string{"ADVERSARIAL_AUDIT", "LEGACY_AGENT_DISPATCH"} {
		t.Setenv(k, "")
	}
	f := readSubagentRunFlags()
	if !f.adversarialAudit {
		t.Error("adversarialAudit default = false, want true (`!= \"0\"`)")
	}
	if f.legacyAgentDispatch {
		t.Error("legacyAgentDispatch default = true, want false (`== \"1\"`)")
	}
}

func TestReadSubagentRunFlags_Toggle(t *testing.T) {
	t.Setenv("ADVERSARIAL_AUDIT", "0")
	t.Setenv("LEGACY_AGENT_DISPATCH", "1")
	f := readSubagentRunFlags()
	if f.adversarialAudit {
		t.Error(`"0" should disable the default-on flags`)
	}
	if !f.legacyAgentDispatch {
		t.Error(`"1" should enable the default-off flag`)
	}
}
