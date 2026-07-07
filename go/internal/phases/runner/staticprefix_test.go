package runner

// staticprefix_test.go — behavior + apicover naming tests for StaticPrefix,
// the read half of the cache-stable prompt-prefix contract (cycle-535 ship).
// CI's apicover -enforce flagged it UNCOVERED (no test named it); these tests
// close the gap by pinning the contract from both ends of the shared
// cycleContextBoundary literal, not by merely naming the identifier.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestStaticPrefix_RoundTripsBaseCycleContext: StaticPrefix over a
// BaseCycleContext-composed prompt must return EXACTLY the static body — if
// any per-cycle dynamic value (cycle, goal_hash, project_root, workspace)
// leaks above the boundary, the provider prompt-cache misses on every cycle.
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

// TestStaticPrefix_NoBoundaryIsWholePrompt: a prompt without the canonical
// "## Cycle Context" boundary is all prefix — StaticPrefix must never
// truncate a prompt it does not understand.
func TestStaticPrefix_NoBoundaryIsWholePrompt(t *testing.T) {
	prompt := "free-form prompt with no cycle context block"
	if got := StaticPrefix(prompt); got != prompt {
		t.Errorf("StaticPrefix(%q) = %q, want the whole prompt back", prompt, got)
	}
}

// TestBaseRunner_ComposePrompt_DelegatesToHooks executes the public
// ComposePrompt seam (apicover flagged it false-green: named but never run):
// it must return the hooks' composition verbatim and pass body+req through
// untouched — the cache-stable audit relies on this being the SAME assembly
// Run uses, not a parallel one.
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
