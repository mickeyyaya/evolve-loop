package triage

// Chronicle S3 RED contract (cycle-784, chronicle-s3-digest-wiring, task
// inject-recent-outcomes-prompts) — triage half; see the scout twin for the
// full contract text. Injected Context["recent_outcomes"] renders AFTER the
// existing stable lines (carryover_summary/fleet_scope); absent or empty key
// keeps the composed prompt byte-identical (shadow-stage regression pin).
//
// Builder implements; must NOT modify these tests.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// RENDER: a Context-carried digest appears in the composed prompt, after the
// existing stable lines.
func TestTriageComposePrompt_InjectsRecentOutcomes(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{
		"carryover_summary": "carryover-under-test",
		"recent_outcomes":   "cycle 42 PASS — harden the flux capacitor",
	}}
	out := hooks{}.ComposePrompt("BODY", req)
	if !strings.Contains(out, "recent_outcomes") {
		t.Fatalf("triage prompt has no recent_outcomes line:\n%s", out)
	}
	if !strings.Contains(out, "cycle 42 PASS — harden the flux capacitor") {
		t.Errorf("triage prompt does not carry the digest content:\n%s", out)
	}
	ro, co := strings.Index(out, "recent_outcomes"), strings.Index(out, "carryover-under-test")
	if co >= 0 && ro < co {
		t.Errorf("recent_outcomes must be appended AFTER the existing stable prefix lines (cache-friendly ordering):\n%s", out)
	}
}

// PIN: absent key and empty key produce byte-identical output with no
// recent_outcomes line at all — shadow/off stages keep today's prompt bytes.
func TestComposePrompt_NoDigestContextKeyIsByteIdentical(t *testing.T) {
	base := map[string]string{"carryover_summary": "c-1", "fleet_scope": "todo-lane-a"}
	withEmpty := map[string]string{"carryover_summary": "c-1", "fleet_scope": "todo-lane-a", "recent_outcomes": ""}

	a := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: base})
	b := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: withEmpty})
	if a != b {
		t.Errorf("absent vs empty recent_outcomes must be byte-identical:\n--- absent ---\n%s\n--- empty ---\n%s", a, b)
	}
	if strings.Contains(a, "recent_outcomes") {
		t.Errorf("prompt without a digest must not carry a recent_outcomes line:\n%s", a)
	}
}
