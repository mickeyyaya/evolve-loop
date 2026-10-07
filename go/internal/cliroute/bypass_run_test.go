package cliroute_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func bypassRunRouter(t *testing.T, bypass bool) *cliroute.Router {
	t.Helper()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t),
		Host: cliroute.Host{LookPath: installed("claude", "agy", "codex")}, Bypass: bypass,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func TestBuild_ABypassRunAnswersEveryRequestFromTheBypassTable(t *testing.T) {
	r := bypassRunRouter(t, true)

	d, err := r.Resolve(cliroute.Request{Agent: "scout", Phase: "scout"})

	if err != nil || !d.Legacy() || d.Plan.Candidates[0] != "codex-tmux" {
		t.Fatalf("a bypass run ignores the table even when the request does not say so: %+v %v", d, err)
	}
	if r.Policy().CLIRouting != nil {
		t.Fatal("a bypass run reports the bypass policy, so the runner reads no operator overlays")
	}
}

func TestBuild_ANormalRunStillRoutesTheTable(t *testing.T) {
	r := bypassRunRouter(t, false)

	d, err := r.Resolve(cliroute.Request{Agent: "scout", Phase: "scout"})

	if err != nil || d.Legacy() || d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("the declared table routes a normal run: %+v %v", d, err)
	}
}
