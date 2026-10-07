package cliroute_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

func deepCappedRouter(t *testing.T, agent string) (*cliroute.Router, string) {
	t.Helper()
	return deepCappedRouterFor(t, `{"name":"`+agent+`","cli":"agy-tmux","model_tier_default":"deep"}`, agent)
}

func deepCappedRouterFor(t *testing.T, profile, agent string) (*cliroute.Router, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".json"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}, Tiers: map[string][]string{"deep": {"claude"}}}
	r, _, err := cliroute.Build(cliroute.Setup{Policy: routingPolicy(block), Profiles: profiles.NewFromDir(dir), Host: cliroute.Host{LookPath: installed("agy", "claude")}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r, root
}

func TestDecision_Walk_IsWhatTheCeilingPermitsAtTheDecisionsOwnTier(t *testing.T) {
	r, root := deepCappedRouter(t, "failure-advisor")
	d, err := r.Resolve(cliroute.Request{Agent: "failure-advisor", ProjectRoot: root, DefaultModel: "balanced"})
	if err != nil {
		t.Fatal(err)
	}

	walk, err := d.Walk()

	if err != nil || !reflect.DeepEqual(walk.Candidates, []string{"claude-tmux"}) {
		t.Fatalf("walk = %v (err %v), want [claude-tmux]: the deep ceiling drops agy (decision chain %v)", walk.Candidates, err, d.Plan.Candidates)
	}
}

func TestResolveRole_ADeepRoleReportsTheFirstCLITheCeilingPermits(t *testing.T) {
	r, root := deepCappedRouter(t, "failure-advisor")

	res, err := r.ResolveRole("failure-advisor", resolvellm.Options{ProjectRoot: root, GitRoot: root})

	if err != nil || res.CLI != "claude-tmux" {
		t.Fatalf("ResolveRole = %+v (err %v), want claude-tmux: agy is not permitted at deep", res, err)
	}
}

func TestResolveRole_ADeepRoleTheCeilingEmptiesIsRefused(t *testing.T) {
	r, root := deepCappedRouterFor(t, `{"name":"scan","cli":"agy-tmux","allowed_clis":["agy"],"model_tier_default":"deep"}`, "scan")

	res, err := r.ResolveRole("scan", resolvellm.Options{ProjectRoot: root, GitRoot: root})

	if !errors.Is(err, cliroute.ErrRefused) || res.CLI != "" {
		t.Fatalf("ResolveRole = %+v (err %v): an agy-only role at deep has nothing the ceiling permits, so it is refused, never answered with agy", res, err)
	}
}
