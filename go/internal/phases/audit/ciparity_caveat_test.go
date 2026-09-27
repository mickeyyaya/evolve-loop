package audit

// A gate may not assert a CI outcome it cannot know. A gate that blocks real
// work citing an impossible CI failure teaches operators to bypass gates. The
// caveat is derived from the same predicate requireTmux uses — does this host
// have tmux — so it stays true if the guarded test set ever changes, rather
// than encoding today's file names.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestCIParityCaveat_HostWithTmuxDisclosesTheDivergence(t *testing.T) {
	got := ciParityCaveat(func(string) (string, error) { return "/opt/homebrew/bin/tmux", nil })
	if got == "" {
		t.Fatalf("a host WITH tmux runs tests CI skips; the divergence must be disclosed")
	}
	for _, want := range []string{"tmux", "SKIP"} {
		if !strings.Contains(got, want) {
			t.Fatalf("caveat must name the mechanism (%q); got %q", want, got)
		}
	}
}

// Parity holds when the host matches CI — no caveat, so a genuine offender is
// not diluted by a warning that does not apply.
func TestCIParityCaveat_HostWithoutTmuxSaysNothing(t *testing.T) {
	if got := ciParityCaveat(func(string) (string, error) { return "", errors.New("not found") }); got != "" {
		t.Fatalf("a host matching CI has no divergence to disclose; got %q", got)
	}
}

// THE headline regression: the message must never assert that CI would fail.
func TestIntegrationTierMessage_DoesNotAssertACIOutcome(t *testing.T) {
	msg := integrationTierFailTemplate
	if strings.Contains(msg, "CI's integration-tier test step would FAIL") {
		t.Fatalf("the gate must not assert a CI outcome it cannot know — that exact claim was false for cycle-1543: %q", msg)
	}
	if !strings.Contains(msg, "locally") {
		t.Fatalf("the message must say WHERE the offenders were observed; got %q", msg)
	}
}

// The template must still carry its evidence: how many, and which.
func TestIntegrationTierMessage_KeepsCountAndOffenders(t *testing.T) {
	msg := integrationTierFailTemplate
	if strings.Count(msg, "%d") != 1 || strings.Count(msg, "%s") != 2 {
		t.Fatalf("template must take (count, caveat, offenders); got %q", msg)
	}
}

// A '%' in the caveat must not become a format verb — the spliced result is
// itself Sprintf'd with (count, offenders), so an unescaped '%' would corrupt
// the very finding an operator has to act on.
func TestIntegrationTierTemplate_CaveatPercentIsEscaped(t *testing.T) {
	out := fmtSprintfLike(integrationTierTemplateWithCaveat(" 100% of lanes contended."), 3, "a; b")
	if strings.Contains(out, "MISSING") || strings.Contains(out, "%!") {
		t.Fatalf("a '%%' in the caveat corrupted the finding: %q", out)
	}
	if !strings.Contains(out, "100% of lanes") {
		t.Fatalf("the caveat text must survive verbatim; got %q", out)
	}
	if !strings.Contains(out, "3 offender(s)") || !strings.Contains(out, "a; b") {
		t.Fatalf("count and offenders must still render; got %q", out)
	}
}

// THE WIRING TEST: the template the gate actually uses must carry the caveat on
// a tmux-bearing host. A correct caveat that never reaches the finding is the
// defect this whole change exists to remove, one layer up.
func TestIntegrationTierTemplate_ProductionSpliceCarriesTheCaveat(t *testing.T) {
	withTmux := integrationTierTemplateWithCaveat(ciParityCaveat(func(string) (string, error) { return "/usr/bin/tmux", nil }))
	if !strings.Contains(withTmux, "parity gap") {
		t.Fatalf("the gate's own template must disclose the parity gap; got %q", withTmux)
	}
	if strings.Contains(withTmux, "would FAIL") {
		t.Fatalf("the spliced template must not assert a CI outcome; got %q", withTmux)
	}
	clean := integrationTierTemplateWithCaveat(ciParityCaveat(func(string) (string, error) { return "", errors.New("no tmux") }))
	if strings.Contains(clean, "parity gap") {
		t.Fatalf("a CI-matching host must not carry a caveat that does not apply; got %q", clean)
	}
}

func fmtSprintfLike(tmpl string, n int, offenders string) string {
	return fmt.Sprintf(tmpl, n, offenders)
}

// The real wiring test, through phase.Run, the path production uses: the
// composition tests above prove the caveat helper is correct, this proves the
// gate emits it. The assertion is on the diagnostic the orchestrator actually
// receives, not on a string built in the test.
func TestRun_IntegrationTierGate_DiagnosticDoesNotAssertACIOutcome(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0) // EGPS green → only the integration-tier gate can FAIL.
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts: fakePromptsFS("body"),
		CheckIntegrationTier: func(core.PhaseRequest) ([]string, error) {
			return []string{"tmux_repl_interactive_test.go:181: exit = 80, want ExitOK"}, nil
		},
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var msg string
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "integration tier") {
			msg = d.Message
		}
	}
	if msg == "" {
		t.Fatalf("expected an integration-tier diagnostic; got %+v", resp.Diagnostics)
	}
	// The gate must not assert an outcome it cannot know.
	if strings.Contains(msg, "CI's integration-tier test step would FAIL") {
		t.Fatalf("the emitted diagnostic still asserts a CI outcome it cannot know: %q", msg)
	}
	if !strings.Contains(msg, "locally") {
		t.Fatalf("the emitted diagnostic must say where the offenders were observed: %q", msg)
	}
	// The offenders and count must survive the template change.
	if !strings.Contains(msg, "1 offender(s)") || !strings.Contains(msg, "exit = 80") {
		t.Fatalf("the emitted diagnostic lost its evidence: %q", msg)
	}
	// No format-verb corruption reached the operator.
	if strings.Contains(msg, "%!") || strings.Contains(msg, "MISSING") {
		t.Fatalf("format corruption in the emitted diagnostic: %q", msg)
	}
}
