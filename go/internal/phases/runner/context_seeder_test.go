package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type seedingHooks struct {
	fakeHooks
	seen int
}

func (h *seedingHooks) SeedContext(req core.PhaseRequest) core.PhaseRequest {
	h.seen++
	req.Context = map[string]string{"seeded": "yes"}
	return req
}

func TestRun_AContextSeederHookSeedsTheRequestOnceBeforeComposition(t *testing.T) {
	t.Parallel()
	hooks := &seedingHooks{fakeHooks: fakeHooks{phase: "audit", agent: "evolve-auditor", model: "sonnet", prompt: "p", verdict: core.VerdictPASS}}
	var _ ContextSeeder = hooks
	r := New(Options{Hooks: hooks, Bridge: &fakeBridge{writeArtifact: "# a\n"}, Prompts: fakePromptsFS("evolve-auditor", "body")})

	if _, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if hooks.seen != 1 || hooks.gotComposeReq.Context["seeded"] != "yes" {
		t.Errorf("seeder calls = %d, compose context = %v, want one seed visible to ComposePrompt", hooks.seen, hooks.gotComposeReq.Context)
	}
}

func TestBaseRunnerSeedContext_HooksWithoutASeederLeaveTheRequestAlone(t *testing.T) {
	t.Parallel()
	r := New(Options{Hooks: &fakeHooks{phase: "scout"}})
	req := core.PhaseRequest{Cycle: 3, Context: map[string]string{"k": "v"}}

	got := r.SeedContext(req)

	if got.Cycle != 3 || got.Context["k"] != "v" || len(got.Context) != 1 {
		t.Errorf("SeedContext = %+v, want the request unchanged", got)
	}
}
