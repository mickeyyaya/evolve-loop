package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestPauseAtRouterWall_AnUnreadableOrNonQuotaArtifactDoesNotPause(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{"corrupt": "{not json", "no-quota": `{"ends":[]}`} {
		body := body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			path := filepath.Join(ws, routerQuotaWallFile)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			cr := &cycleRun{cs: CycleState{WorkspacePath: ws}}

			if err := cr.pauseAtRouterWall(PhaseScout); err != nil {
				t.Fatalf("err=%v, want no pause", err)
			}
			if _, serr := os.Stat(path); !errors.Is(serr, os.ErrNotExist) {
				t.Fatalf("artifact still present: %v", serr)
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

func TestRemoveRouterWall_WarnsOnAFailureAndIsQuietWhenTheWallIsAbsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	busy := filepath.Join(dir, "busy")
	if err := os.MkdirAll(filepath.Join(busy, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	removeRouterWall(&out, filepath.Join(dir, "absent"))
	if out.Len() != 0 {
		t.Fatalf("an absent wall warned: %q", out.String())
	}
	removeRouterWall(&out, busy)
	if !strings.Contains(out.String(), "WARN remove the quota wall") {
		t.Fatalf("a failed removal did not warn: %q", out.String())
	}
}

func TestWriteRouterWall_WarnsWhenTheWallCannotBeWritten(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	writeRouterWall(&out, filepath.Join(t.TempDir(), "missing", "dir", routerQuotaWallFile), []byte("{}"))
	if !strings.Contains(out.String(), "WARN write the quota wall") {
		t.Fatalf("a failed write did not warn: %q", out.String())
	}
	out.Reset()
	path := filepath.Join(t.TempDir(), routerQuotaWallFile)
	writeRouterWall(&out, path, []byte("{}"))
	if out.Len() != 0 {
		t.Fatalf("a good write warned: %q", out.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the wall was not written: %v", err)
	}
}

func TestBenchQuotaFamilies_AFailedBenchWriteStillReturns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolve, "cli-health.json", "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolve, "policy.json"), []byte(`{"cli_health":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := &cycleRun{o: &Orchestrator{now: time.Now}, req: CycleRequest{ProjectRoot: root}}
	cr.benchQuotaFamilies(Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"}))

	var out bytes.Buffer
	benchFamily(&out, clihealth.NewStore(root, time.Now), FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"})
	if !strings.Contains(out.String(), "WARN bench claude-tmux at its usage wall") {
		t.Fatalf("a failed bench write did not warn: %q", out.String())
	}
}

func TestBenchQuotaFamilies_BenchesEachQuotaFamilyWhenCLIHealthIsOn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_health":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := &cycleRun{o: &Orchestrator{now: time.Now}, req: CycleRequest{ProjectRoot: root}}

	cr.benchQuotaFamilies(Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"}))

	if got := clihealth.NewStore(root, time.Now).Active(); len(got) != 1 {
		t.Fatalf("benches = %+v, want the claude family benched at its usage wall", got)
	}
}
