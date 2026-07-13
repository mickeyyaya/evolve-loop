package scout

// Chronicle S3 RED contract (cycle-784, chronicle-s3-digest-wiring, task
// inject-recent-outcomes-prompts). At digest stage=enforce the orchestrator
// injects Context["recent_outcomes"] (the seeded recent-outcomes.md bytes);
// scout's ComposePrompt must render it as a context line APPENDED AFTER the
// existing stable-prefix lines (strategy/goal/challenge_token — cache-friendly
// ordering). When the key is absent or empty the composed prompt is
// byte-identical to today's output (the shadow-stage regression pin).
//
// Builder implements; must NOT modify these tests.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// RENDER: a Context-carried digest appears in the composed prompt, after the
// existing stable lines.
func TestScoutComposePrompt_InjectsRecentOutcomes(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{
		"strategy":        "strategy-under-test",
		"recent_outcomes": "cycle 42 PASS — harden the flux capacitor",
	}}
	out := hooks{}.ComposePrompt("BODY", req)
	if !strings.Contains(out, "recent_outcomes") {
		t.Fatalf("scout prompt has no recent_outcomes line:\n%s", out)
	}
	if !strings.Contains(out, "cycle 42 PASS — harden the flux capacitor") {
		t.Errorf("scout prompt does not carry the digest content:\n%s", out)
	}
	ro, st := strings.Index(out, "recent_outcomes"), strings.Index(out, "strategy-under-test")
	if st >= 0 && ro < st {
		t.Errorf("recent_outcomes must be appended AFTER the existing stable prefix lines (cache-friendly ordering):\n%s", out)
	}
}

// PIN: absent key and empty key produce byte-identical output with no
// recent_outcomes line at all — shadow/off stages keep today's prompt bytes.
func TestComposePrompt_NoDigestContextKeyIsByteIdentical(t *testing.T) {
	base := map[string]string{"strategy": "s-1", "goal": "g-1", "challengeToken": "tok-1"}
	withEmpty := map[string]string{"strategy": "s-1", "goal": "g-1", "challengeToken": "tok-1", "recent_outcomes": ""}

	a := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: base})
	b := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: withEmpty})
	if a != b {
		t.Errorf("absent vs empty recent_outcomes must be byte-identical:\n--- absent ---\n%s\n--- empty ---\n%s", a, b)
	}
	if strings.Contains(a, "recent_outcomes") {
		t.Errorf("prompt without a digest must not carry a recent_outcomes line:\n%s", a)
	}
}
