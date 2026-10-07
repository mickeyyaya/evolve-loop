package usageevidence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type wallingBridge struct {
	*fixtures.FakeBridge
	pane string
}

func (w wallingBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	report, _ := json.Marshal(map[string]any{"captured_at": time.Now(), "cli": req.CLI, "pattern_name": "exhausted", "pane_tail": w.pane})
	if err := os.WriteFile(filepath.Join(req.Workspace, "escalation-report.json"), report, 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	return w.FakeBridge.Launch(ctx, req)
}

func liveAgyClaudeWall(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "bridge", "testdata", "agy-claude-individual-quota-wall.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type wallAttempt struct {
	root, ws string
	launched time.Time
}

func (a wallAttempt) active() map[string]clihealth.Entry {
	return clihealth.NewStore(a.root, nil).Active()
}

func (a wallAttempt) records(t *testing.T) []Line { t.Helper(); return readRecords(t, a.ws) }

func launchIntoTheWall(t *testing.T, cli, pane string, probe func(context.Context, string) (string, error)) wallAttempt {
	t.Helper()
	a := wallAttempt{root: t.TempDir(), ws: t.TempDir()}
	explain := New(Options{ProjectRoot: a.root, EvolveDir: filepath.Join(a.root, ".evolve"), Act: true, Probe: probe})
	inner := wallingBridge{FakeBridge: &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 85}}, pane: pane}
	a.launched = time.Now()

	if res, _ := Wrap(inner, explain, nil).Launch(context.Background(), core.BridgeRequest{CLI: cli, Agent: "router", ProjectRoot: a.root, Workspace: a.ws}); res.ExitCode != 85 {
		t.Fatalf("the decorator changed the attempt's exit to %d", res.ExitCode)
	}
	return a
}

func launchAgyClaudeIntoTheWall(t *testing.T, probe func(context.Context, string) (string, error)) (map[string]clihealth.Entry, time.Time) {
	t.Helper()
	a := launchIntoTheWall(t, "agy-claude-tmux", liveAgyClaudeWall(t), probe)
	return a.active(), a.launched
}

func TestBridge_TheLiveAgyClaudeWallWithADrainedClaudeGroupBenchesAgyClaudeUntilTheScreensReset(t *testing.T) {
	active, launched := launchAgyClaudeIntoTheWall(t, func(context.Context, string) (string, error) {
		return usageFixture(t, "agy_usage_claude_drained.txt"), nil
	})

	bench, ok := active["agy-claude"]
	if !ok {
		t.Fatalf("benches %v; the drained Claude group must bench agy-claude", active)
	}
	if lo, hi := launched.Add(2*time.Hour+15*time.Minute), time.Now().Add(2*time.Hour+20*time.Minute); bench.BenchedUntil.Before(lo) || bench.BenchedUntil.After(hi) {
		t.Errorf("benched until %v; want the screen's 2h15m reset, not the wall's own cooldown", bench.BenchedUntil)
	}
	if _, gemini := active["agy"]; gemini {
		t.Errorf("agy's Gemini family was benched for a Claude-group wall")
	}
}

func TestBridge_AnUnavailableUsageQueryFallsBackToTheAttemptsOwnWallAndBenchesTheFailingFamily(t *testing.T) {
	active, launched := launchAgyClaudeIntoTheWall(t, func(context.Context, string) (string, error) {
		return "", errors.New("REPL prompt never appeared after 60s")
	})

	bench, ok := active["agy-claude"]
	if !ok || bench.Reason != "exhausted" || bench.BenchedUntil.Before(launched.Add(clihealth.DefaultCooldown)) {
		t.Fatalf("benches %v; with no usage evidence the attempt's classified wall benches the failing agy-claude", active)
	}
	if _, gemini := active["agy"]; gemini {
		t.Errorf("the fallback benched agy's Gemini family, not the failing agy-claude")
	}
}

func TestBridge_AFreshHealthyUsageScreenIsRecordedAsEvidenceAndTheClassifiedWallIsStillBenchedOnce(t *testing.T) {
	a := launchIntoTheWall(t, "agy-claude-tmux", liveAgyClaudeWall(t), func(context.Context, string) (string, error) {
		return usageFixture(t, "agy_usage_groups.txt"), nil
	})

	bench, ok := a.active()["agy-claude"]
	if !ok || bench.Strikes != 1 || bench.Reason != "exhausted" {
		t.Fatalf("benches %v; a healthy family screen is evidence, not a veto: the classified wall benches agy-claude once, as on the runner's walk", a.active())
	}
	if records := a.records(t); len(records) != 1 || records[0].Evidence.Verdict != usageprobe.VerdictHealthy {
		t.Errorf("records %+v; want the healthy screen recorded as the attempt's usage evidence", records)
	}
}

func TestBridge_AWallTheFamilyScreenDoesNotShowBenchesAlikeOnTheAdvisorsWalkAndOnTheRunnersWalk(t *testing.T) {
	fableScreen := func(context.Context, string) (string, error) {
		return usageFixture(t, "claude_usage_fable_exhausted.txt"), nil
	}
	const fableWall = "You've hit your Fable limit · resets 9pm (Asia/Taipei)"

	advisor := launchIntoTheWall(t, "claude-tmux", fableWall, fableScreen)
	runner := launchIntoTheWall(t, "claude-tmux", fableWall, fableScreen)
	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: runner.root, Workspace: runner.ws, CLI: "claude-tmux", DispatchStart: runner.launched}, time.Now, nil)

	onAdvisor, onRunner := advisor.active()["claude"], runner.active()["claude"]
	if onAdvisor.Strikes != 1 || onRunner.Strikes != 1 || onAdvisor.Reason != onRunner.Reason {
		t.Fatalf("advisor's walk benched %+v, runner's walk benched %+v; one exit-85 wall with one usage screen must bench the same on every walk", onAdvisor, onRunner)
	}
}

func TestBridge_AHealthyReadTakenDuringTheAttemptIsReusedWithoutASecondQuery(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	queries := 0
	explain := New(Options{ProjectRoot: root, EvolveDir: evolveDir, Act: true, Probe: func(context.Context, string) (string, error) {
		queries++
		return usageFixture(t, "agy_usage_groups.txt"), nil
	}})
	inner := readDuringAttempt{wallingBridge: wallingBridge{FakeBridge: &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 85}}, pane: liveAgyClaudeWall(t)}, evolveDir: evolveDir}

	_, _ = Wrap(inner, explain, nil).Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Agent: "router", ProjectRoot: root, Workspace: ws})

	records := readRecords(t, ws)
	if queries != 0 || len(records) != 1 || !records[0].Evidence.Cached || records[0].Evidence.Verdict != usageprobe.VerdictHealthy {
		t.Fatalf("%d usage queries, records %+v; a read another lane took after this attempt started already describes it", queries, records)
	}
}

type readDuringAttempt struct {
	wallingBridge
	evolveDir string
}

func (r readDuringAttempt) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	obs := usageprobe.Observation{CLI: "agy", ObservedAt: time.Now(),
		Windows: []quotastate.UsageWindow{{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindFiveHour, PercentUsed: 40, Family: "agy-claude"}}}
	if err := usageprobe.RecordObservation(r.evolveDir, obs); err != nil {
		return core.BridgeResponse{}, err
	}
	return r.wallingBridge.Launch(ctx, req)
}

func TestBridge_AHealthyReadRecordedBeforeTheFailingAttemptIsQueriedAgainSoTheScreensResetSetsTheBench(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	preWave := usageprobe.Observation{CLI: "agy", ObservedAt: time.Now().Add(-5 * time.Minute),
		Windows: []quotastate.UsageWindow{{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindFiveHour, PercentUsed: 40, Family: "agy-claude"}}}
	if err := usageprobe.RecordObservation(evolveDir, preWave); err != nil {
		t.Fatal(err)
	}
	explain := New(Options{ProjectRoot: root, EvolveDir: evolveDir, Act: true, Probe: func(context.Context, string) (string, error) {
		return usageFixture(t, "agy_usage_claude_drained.txt"), nil
	}})
	inner := wallingBridge{FakeBridge: &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 85}}, pane: liveAgyClaudeWall(t)}

	launched := time.Now()
	_, _ = Wrap(inner, explain, nil).Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Agent: "router", ProjectRoot: root, Workspace: ws})

	bench, ok := clihealth.NewStore(root, nil).Active()["agy-claude"]
	if !ok || bench.BenchedUntil.Before(launched.Add(2*time.Hour+15*time.Minute)) {
		t.Fatalf("bench %+v; the pre-wave probe's healthy read predates the failing attempt, so the screen is read again and its drained Claude group benches until its 2h15m reset, not the wall's 1h4m hint", bench)
	}
}

func TestBridge_OnlyAnEscalatedExitBenchesItsWallAsOnTheRunnersWalk(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	explain := New(Options{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Act: true, Probe: func(context.Context, string) (string, error) {
		return "", errors.New("REPL prompt never appeared after 60s")
	}})
	inner := wallingBridge{FakeBridge: &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 80}}, pane: liveAgyClaudeWall(t)}

	_, _ = Wrap(inner, explain, nil).Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", ProjectRoot: root, Workspace: ws})

	if active := clihealth.NewStore(root, nil).Active(); len(active) != 0 {
		t.Fatalf("benches %v; a boot timeout is not an escalation, so its report benches nothing, as in the runner's walk", active)
	}
}
