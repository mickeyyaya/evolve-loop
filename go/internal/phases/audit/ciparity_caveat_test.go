package audit

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

func TestCIParityCaveat_HostWithoutTmuxSaysNothing(t *testing.T) {
	if got := ciParityCaveat(func(string) (string, error) { return "", errors.New("not found") }); got != "" {
		t.Fatalf("a host matching CI has no divergence to disclose; got %q", got)
	}
}

func TestIntegrationTierMessage_DoesNotAssertACIOutcome(t *testing.T) {
	msg := integrationTierFailTemplate
	if strings.Contains(msg, "CI's integration-tier test step would FAIL") {
		t.Fatalf("the gate must not assert a CI outcome it cannot know — that exact claim was false for cycle-1543: %q", msg)
	}
	if !strings.Contains(msg, "locally") {
		t.Fatalf("the message must say WHERE the offenders were observed; got %q", msg)
	}
}

func TestIntegrationTierMessage_KeepsCountAndOffenders(t *testing.T) {
	msg := integrationTierFailTemplate
	if strings.Count(msg, "%d") != 1 || strings.Count(msg, "%s") != 2 {
		t.Fatalf("template must take (count, caveat, offenders); got %q", msg)
	}
}

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

func TestRun_IntegrationTierGate_DiagnosticDoesNotAssertACIOutcome(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
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
	if strings.Contains(msg, "CI's integration-tier test step would FAIL") {
		t.Fatalf("the emitted diagnostic still asserts a CI outcome it cannot know: %q", msg)
	}
	if !strings.Contains(msg, "locally") {
		t.Fatalf("the emitted diagnostic must say where the offenders were observed: %q", msg)
	}
	if !strings.Contains(msg, "1 offender(s)") || !strings.Contains(msg, "exit = 80") {
		t.Fatalf("the emitted diagnostic lost its evidence: %q", msg)
	}
	if strings.Contains(msg, "%!") || strings.Contains(msg, "MISSING") {
		t.Fatalf("format corruption in the emitted diagnostic: %q", msg)
	}
}
