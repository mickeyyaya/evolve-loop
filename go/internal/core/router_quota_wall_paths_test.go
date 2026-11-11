package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
)

type causeBridge struct{ resp BridgeResponse }

func (b causeBridge) Launch(context.Context, BridgeRequest) (BridgeResponse, error) {
	return b.resp, wrapTransient(85)
}

func (causeBridge) Probe(context.Context) (BridgeProbe, error) { return BridgeProbe{}, nil }

func TestBridgeLauncher_AFailedLaunchWithACauseCodeCarriesTheWall(t *testing.T) {
	t.Parallel()
	b := causeBridge{resp: BridgeResponse{ExitCode: 85, CauseCode: "rate_limit"}}

	_, err := bridgeLauncher{b: b}.Launch(context.Background(), advisor.LaunchRequest{CLI: "claude-tmux", Agent: "router"})

	if wall := walkedWall(err); !wall.HasQuota() {
		t.Fatalf("err=%v wall=%+v, want a quota wall that survives the advisor error chain", err, wall)
	}
}

func TestPauseAtRouterWall_AWallWithNoQuotaRowDoesNotPause(t *testing.T) {
	t.Parallel()
	for name, wall := range map[string]Wall{
		"no wall":    {},
		"logged out": Wall{}.Observe(FamilyEnd{CLI: "agy-tmux", Pattern: "login_required"}),
	} {
		wall := wall
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cr := &cycleRun{routerWall: wall}

			if err := cr.pauseAtRouterWall(PhaseScout); err != nil {
				t.Fatalf("err=%v, want no pause", err)
			}
		})
	}
}

func TestBenchQuotaFamilies_SkipsTheStoreWhenCLIHealthIsOff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_health":{"enabled":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := &cycleRun{o: &Orchestrator{now: time.Now}, req: CycleRequest{ProjectRoot: root}}
	wall := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"})

	cr.benchQuotaFamilies(wall)

	if got := clihealth.NewStore(root, time.Now).Active(); len(got) != 0 {
		t.Fatalf("benches = %+v, want none while cli_health is off", got)
	}
}
