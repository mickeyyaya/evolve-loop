package policy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureLegacyLabelWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := legacyLabelWarnings
	legacyLabelWarnings = &buf
	t.Cleanup(func() { legacyLabelWarnings = old })
	return &buf
}

func writeWorkflowPolicy(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInteractivePolicyFor_TheRetrospectiveKeyStillAppliesToTheRetroLabelWithAWarning(t *testing.T) {
	warnings := captureLegacyLabelWarnings(t)
	dir := writeWorkflowPolicy(t, `{"workflow":{"interactive_policy":"escalate","interactive_policies":{"retrospective":"auto_yes"}}}`)

	if got := InteractivePolicyFor(dir, "retro"); got != "auto_yes" {
		t.Errorf("InteractivePolicyFor(retro) = %q, want auto_yes from the retrospective key", got)
	}
	if !strings.Contains(warnings.String(), `interactive_policies.retrospective`) || !strings.Contains(warnings.String(), `"retro"`) {
		t.Errorf("the legacy key applied without a warning that names it and its new label: %q", warnings.String())
	}
}

func TestInteractivePolicyFor_TheRetroKeyWinsOverTheRetrospectiveKeyWithNoWarning(t *testing.T) {
	warnings := captureLegacyLabelWarnings(t)
	dir := writeWorkflowPolicy(t, `{"workflow":{"interactive_policies":{"retrospective":"auto_yes","retro":"escalate"}}}`)

	if got := InteractivePolicyFor(dir, "retro"); got != "escalate" {
		t.Errorf("InteractivePolicyFor(retro) = %q, want escalate from the retro key", got)
	}
	if got := InteractivePolicyFor(writeWorkflowPolicy(t, `{"workflow":{"interactive_policy":"escalate","interactive_policies":{"retrospective":""}}}`), "retro"); got != "escalate" {
		t.Errorf("InteractivePolicyFor(retro) = %q, want the global escalate: an empty legacy value never applies", got)
	}
	if warnings.Len() != 0 {
		t.Errorf("a warning for a key that did not apply: %q", warnings.String())
	}
}

func TestPhaseArtifactTimeouts_TheRetrospectiveOverrideStillAppliesToTheRetroLabelWithAWarning(t *testing.T) {
	warnings := captureLegacyLabelWarnings(t)

	got := BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"retrospective": 1500}}.PhaseArtifactTimeouts()

	if got["retro"] != 1500 {
		t.Errorf("PhaseArtifactTimeouts()[retro] = %d, want 1500 from the retrospective override", got["retro"])
	}
	if !strings.Contains(warnings.String(), `phase_artifact_timeout_s.retrospective`) {
		t.Errorf("the legacy override applied without a warning: %q", warnings.String())
	}
}

func TestPhaseArtifactTimeouts_TheRetroOverrideWinsAndTheDefaultNeedsNoWarning(t *testing.T) {
	warnings := captureLegacyLabelWarnings(t)

	if got := (BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"retrospective": 1500, "retro": 600}}).PhaseArtifactTimeouts(); got["retro"] != 600 {
		t.Errorf("PhaseArtifactTimeouts()[retro] = %d, want 600 from the retro override", got["retro"])
	}
	if got := (BridgePolicy{}).PhaseArtifactTimeouts(); got["retro"] != 900 {
		t.Errorf("PhaseArtifactTimeouts()[retro] = %d, want the 900 default", got["retro"])
	}
	if got := (BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"retrospective": -1}}).PhaseArtifactTimeouts(); got["retro"] != 900 {
		t.Errorf("PhaseArtifactTimeouts()[retro] = %d, want the 900 default: a non-positive override never applies", got["retro"])
	}
	if warnings.Len() != 0 {
		t.Errorf("a warning for a key that did not apply: %q", warnings.String())
	}
}
