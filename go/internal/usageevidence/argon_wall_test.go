package usageevidence

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	argonScopeKey = "agy@Gemini 4 Argon"
	argonWallPane = "Individual quota reached for this model. Resets in 127h38m0s."
)

var agyFirstChain = []string{"agy-tmux", "agy-claude-tmux", "claude-tmux"}

func launchAgyDeepIntoTheWall(t *testing.T, usageScreen string) wallAttempt {
	t.Helper()
	a := wallAttempt{root: t.TempDir(), ws: t.TempDir(), launched: time.Now()}
	explain := New(Options{ProjectRoot: a.root, EvolveDir: filepath.Join(a.root, ".evolve"), Act: true, Probe: func(context.Context, string) (string, error) {
		return usageFixture(t, usageScreen), nil
	}})
	inner := wallingBridge{FakeBridge: &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 85}}, pane: argonWallPane}
	req := core.BridgeRequest{CLI: "agy-tmux", Model: "deep", Agent: "build", ProjectRoot: a.root, Workspace: a.ws}
	if res, _ := Wrap(inner, explain, nil).Launch(context.Background(), req); res.ExitCode != 85 {
		t.Fatalf("the decorator changed the attempt's exit to %d", res.ExitCode)
	}
	return a
}

func (a wallAttempt) chainAt(tier string) []string {
	plan := llmroute.Plan{Candidates: slices.Clone(agyFirstChain), Triggers: llmroute.DefaultTriggers(), Model: tier, Tiers: []string{tier}}
	return bridgechain.ApplyCLIHealthBench(a.root, "build", plan, nil, nil).Candidates
}

func TestBridge_Cycle1951AnArgonWallAtDeepBenchesOnlyTheArgonGroupAndKeepsTheBalancedAgySeatFirst(t *testing.T) {
	a := launchAgyDeepIntoTheWall(t, "agy_usage_argon_drained.txt")

	active := a.active()
	if _, family := active["agy"]; family {
		t.Fatalf("benches %v; an Argon wall at deep benched all of agy, so balanced Gemini 3.8 Flash seats lose their place (cycle 1951)", active)
	}
	if _, scoped := active[argonScopeKey]; !scoped {
		t.Fatalf("benches %v; want the wall benched on the Argon model group %q", active, argonScopeKey)
	}
	if got := a.chainAt("balanced"); got[0] != "agy-tmux" {
		t.Errorf("balanced chain after an Argon wall = %v; the Flash seat must keep agy-tmux first", got)
	}
}

func TestBridge_Cycle1951AnArgonWallAtDeepWalksTheDeepSeatPastArgon(t *testing.T) {
	a := launchAgyDeepIntoTheWall(t, "agy_usage_argon_drained.txt")

	if got, want := a.chainAt("deep"), []string{"agy-claude-tmux", "claude-tmux", "agy-tmux"}; !slices.Equal(got, want) {
		t.Errorf("deep chain after an Argon wall = %v, want %v: the deep seat walks past Argon", got, want)
	}
}

func TestBridge_Cycle1951AGeminiGroupWallStillBenchesTheWholeAgyFamily(t *testing.T) {
	a := launchAgyDeepIntoTheWall(t, "agy_usage_gemini_drained.txt")

	if _, family := a.active()["agy"]; !family {
		t.Fatalf("benches %v; a drained Gemini group exhausts the whole agy family, so the family is benched", a.active())
	}
	if got := a.chainAt("balanced"); got[0] == "agy-tmux" {
		t.Errorf("balanced chain after a family-wide wall = %v; agy-tmux must leave the front", got)
	}
}
