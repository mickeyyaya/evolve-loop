package runner

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestStaticPrefix_RoundTripsBaseCycleContext(t *testing.T) {
	body := "PERSONA\n\nrules body"
	req := core.PhaseRequest{Cycle: 42, GoalHash: "abc123", ProjectRoot: "/proj", Workspace: "/ws"}

	got := StaticPrefix(BaseCycleContext(body, req))
	if got != body {
		t.Errorf("StaticPrefix(BaseCycleContext(...)) = %q, want the untouched body %q", got, body)
	}
	for _, dyn := range []string{"42", "abc123", "/proj", "/ws"} {
		if strings.Contains(got, dyn) {
			t.Errorf("per-cycle dynamic value %q leaked into the cache-stable prefix", dyn)
		}
	}
}

func TestStaticPrefix_NoBoundaryIsWholePrompt(t *testing.T) {
	prompt := "free-form prompt with no cycle context block"
	if got := StaticPrefix(prompt); got != prompt {
		t.Errorf("StaticPrefix(%q) = %q, want the whole prompt back", prompt, got)
	}
}

func TestBaseRunner_ComposePrompt_DelegatesToHooks(t *testing.T) {
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto",
		prompt: "hook-composed prompt"}
	br := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "agent body")})

	req := core.PhaseRequest{Cycle: 9, GoalHash: "gh9"}
	if got := br.ComposePrompt("inline body", req); got != "hook-composed prompt" {
		t.Errorf("ComposePrompt = %q, want the hooks' composition verbatim", got)
	}
	if hooks.gotComposeBody != "inline body" {
		t.Errorf("hooks received body %q, want the inline body passed through", hooks.gotComposeBody)
	}
	if hooks.gotComposeReq.Cycle != 9 || hooks.gotComposeReq.GoalHash != "gh9" {
		t.Errorf("hooks received req %+v, want the caller's request passed through", hooks.gotComposeReq)
	}
}
