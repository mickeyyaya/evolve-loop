package guards

import "testing"

// TestProtectedSurface_FableSkillOverlay locks the intent that the fable skill
// directory is integrity-protected: its SKILL.md is preloaded into deep/top-tier
// phase prompts (policy.ResolveOverlays → bridge skill-overlay injection), so a
// tampered fable persona would silently rewrite every deep-tier phase agent's
// operating discipline. Dropping the manifest entry must fail HERE, loudly.
func TestProtectedSurface_FableSkillOverlay(t *testing.T) {
	if !IsProtectedSurface("skills/fable/SKILL.md") {
		t.Error("skills/fable/SKILL.md must be a protected surface — it is injected into phase prompts as operating discipline (audit-F1)")
	}
	if !IsProtectedSurface("/skills/fable/") {
		t.Error("/skills/fable/ must be a protected surface")
	}
}
