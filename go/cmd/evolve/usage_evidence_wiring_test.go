package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func installUsageEvidence(t *testing.T, fixture string, probeErr error) *[]string {
	t.Helper()
	prev := usageEvidenceFn
	var probed []string
	usageEvidenceFn = func(projectRoot, evolveDir string, log io.Writer) usageevidence.Explain {
		return usageevidence.New(usageevidence.Options{
			ProjectRoot: projectRoot, EvolveDir: evolveDir, Act: true, Log: log,
			Probe: func(_ context.Context, family string) (string, error) {
				probed = append(probed, family)
				if probeErr != nil {
					return "", probeErr
				}
				return usageFixture(t, fixture), nil
			},
		})
	}
	t.Cleanup(func() { usageEvidenceFn = prev })
	return &probed
}

func TestCLIUpdateWiring_ABootTimeoutWithADrainedWindowIsAVerifiedQuotaCauseAndBenchesUntilTheReset(t *testing.T) {
	root := t.TempDir()
	probed := installUsageEvidence(t, "agy_usage_gemini_drained.txt", nil)
	seams := productionCLIUpdateWiring(root, io.Discard).seams
	seams.Probe = func(context.Context, string) error {
		return fmt.Errorf("doctor live agy-tmux: %w after 2 attempt(s); final pane: signing in", cliupdate.ErrBootTimeout)
	}
	seams.Version = func(string) (string, error) { return "1.3.0", nil }

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{{Name: "agy", UpdateArgv: []string{"agy", "update"}}}, seams, nil)

	if res := rep.Results[0]; res.Status != cliupdate.StatusQuotaExhausted || !strings.Contains(res.Detail, "GEMINI MODELS") {
		t.Fatalf("result %+v after probing %v; want quota-exhausted from the production usage evidence", res, *probed)
	}
	bench, ok := clihealth.NewStore(root, nil).Active()["agy"]
	if !ok || !bench.BenchedUntil.After(time.Now().Add(3*time.Hour)) {
		t.Errorf("bench = %+v (present=%v); want agy benched until the drained windows reset (capped at 24h)", bench, ok)
	}
}

func TestCLIUpdateWiring_ABootTimeoutWithHealthyUsageStaysABootTimeout(t *testing.T) {
	installUsageEvidence(t, "agy_usage_groups.txt", nil)
	seams := productionCLIUpdateWiring(t.TempDir(), io.Discard).seams
	seams.Probe = func(context.Context, string) error {
		return fmt.Errorf("doctor live agy-tmux: %w after 2 attempt(s); final pane: signing in", cliupdate.ErrBootTimeout)
	}
	seams.Version = func(string) (string, error) { return "1.3.0", nil }

	res := cliupdate.Update(context.Background(), []cliupdate.Family{{Name: "agy", UpdateArgv: []string{"agy", "update"}}}, seams, nil).Results[0]

	if res.Status != cliupdate.StatusBootTimeout || !strings.Contains(res.Detail, "quota ruled out") || strings.Contains(res.Detail, "unsubscribed") {
		t.Fatalf("result %+v; want boot-timeout with quota ruled out", res)
	}
}

func TestLoopPreflightOptions_AFailedBootIsExplainedByTheUsageEvidence(t *testing.T) {
	root := t.TempDir()
	probed := installUsageEvidence(t, "", errors.New("REPL prompt never appeared after 60s"))

	opts := loopPreflightOptions(loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}, io.Discard)

	if opts.UsageEvidence == nil {
		t.Fatal("the readiness gate has no usage evidence seam")
	}
	if got := opts.UsageEvidence("agy-claude-tmux"); !strings.Contains(got, "auth, install or network") || len(*probed) != 1 || (*probed)[0] != "agy" {
		t.Errorf("summary %q after probing %v", got, *probed)
	}
}

func TestWithUsageEvidence_WrapsTheBridgeOnlyWhenThereIsAnExplainer(t *testing.T) {
	inner := &fixtures.FakeBridge{}
	if got := withUsageEvidence(inner, nil, signalcenter.New()); got != core.Bridge(inner) {
		t.Errorf("no explainer must leave the bridge as it is, got %T", got)
	}
	explain := func(context.Context, string, time.Time) usageprobe.Evidence { return usageprobe.Evidence{} }
	if _, ok := withUsageEvidence(inner, explain, nil).(*usageevidence.Bridge); !ok {
		t.Errorf("an explainer must wrap the bridge in the usage evidence decorator")
	}
}

func TestWireOrchestratorDeps_TheObserverAndTheBridgeShareOneUsageEvidence(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"explainUsage := usageEvidenceFn(projectRoot, evolveDir, console)", "withUsageEvidence(br, explainUsage, signals)", "observerDeps{signals: signals, usage: explainUsage}"} {
		if strings.Count(string(src), want) != 1 {
			t.Errorf("cmd_cycle.go must contain %q exactly once", want)
		}
	}
	explain := func(context.Context, string, time.Time) usageprobe.Evidence { return usageprobe.Evidence{} }
	autospawn := true
	opts := phaseObserverOptions(policy.ObserverPolicy{Autospawn: &autospawn}, "shadow", observerDeps{signals: signalcenter.New(), usage: explain})
	if len(opts) != 1 {
		t.Fatalf("an autospawned observer yields one option, got %d", len(opts))
	}
	off := false
	if opts := phaseObserverOptions(policy.ObserverPolicy{Autospawn: &off}, "shadow", observerDeps{}); opts != nil {
		t.Errorf("a disabled observer yields no option")
	}
}
