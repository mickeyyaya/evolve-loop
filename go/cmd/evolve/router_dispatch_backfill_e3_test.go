package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func e3AgyClaudeRouter(t *testing.T, router *policy.RouterPolicy) *cliroute.Router {
	t.Helper()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	return routerOverEvolveDir(t, t.TempDir(), policy.Policy{CLIRouting: &block, Router: router})
}

func e3Bench(t *testing.T, root string, families ...string) {
	t.Helper()
	store := clihealth.NewStore(root, nil)
	for _, f := range families {
		if err := store.Bench(clihealth.Entry{Family: f, Reason: "rate_limit", BenchedAt: time.Now(), BenchedUntil: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBenchedFamilies_ReadsOnlyActiveBenches(t *testing.T) {
	root := t.TempDir()
	if got := benchedFamilies(root); len(got) != 0 {
		t.Fatalf("an empty store benched %v", got)
	}
	e3Bench(t, root, "agy", "claude")
	store := clihealth.NewStore(root, nil)
	if err := store.Bench(clihealth.Entry{Family: "codex", BenchedAt: time.Now().Add(-2 * time.Hour), BenchedUntil: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	got := benchedFamilies(root)

	if !reflect.DeepEqual(got, map[string]bool{"agy": true, "claude": true}) {
		t.Errorf("benched = %v, want agy and claude only (codex expired)", got)
	}
}

func e3RefuseEveryWalk(t *testing.T) *[]string {
	t.Helper()
	var tiers []string
	prev := routerDispatchWalkAt
	t.Cleanup(func() { routerDispatchWalkAt = prev })
	routerDispatchWalkAt = func(d cliroute.Decision, tier string, benched func(string) bool) (llmroute.Plan, bool, error) {
		tiers = append(tiers, tier)
		return llmroute.Plan{}, false, fmt.Errorf("%w: no candidate of %v is permitted at tier %s", cliroute.ErrRefused, d.Plan.Candidates, tier)
	}
	return &tiers
}

func TestResolveRouterDispatchHealthy_ATierNoCandidatePermitsIsAnError(t *testing.T) {
	tiers := e3RefuseEveryWalk(t)
	r := e3AgyClaudeRouter(t, nil)

	rd, err := resolveRouterDispatchHealthy(r, "", decisionPlan, nil)

	if !errors.Is(err, cliroute.ErrRefused) || len(*tiers) != 1 {
		t.Fatalf("rd %+v err %v tiers %v, want the walk refusal", rd, err, *tiers)
	}
	if err == nil || !strings.Contains(err.Error(), "is permitted at tier") {
		t.Fatalf("rd %+v err %v, want the no-permitted-candidate refusal", rd, err)
	}
	if !reflect.DeepEqual(rd, routerDispatch{}) {
		t.Errorf("a refused dispatch returned %+v", rd)
	}
}

func e3CaptureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = prev }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestWireRouterAdvisor_ANoRouteTableIsAWrappedError(t *testing.T) {
	e3RefuseEveryWalk(t)
	r := e3AgyClaudeRouter(t, nil)

	adv, err := wireRouterAdvisor(routerAdvisorWiring{router: r, root: t.TempDir(), bridge: &fixtures.FakeBridge{}, prompts: prompts.NewFromDir(t.TempDir())})

	if adv != nil || err == nil || !strings.HasPrefix(err.Error(), "the routing advisor has no route: ") {
		t.Fatalf("advisor %v err %v", adv, err)
	}
}

func TestWireRouterAdvisor_EveryFamilyBenchedWarnsButStillWires(t *testing.T) {
	root := t.TempDir()
	e3Bench(t, root, "agy", "claude")
	r := e3AgyClaudeRouter(t, nil)
	var adv *core.PhaseAdvisor
	var err error

	stderr := e3CaptureStderr(t, func() {
		adv, err = wireRouterAdvisor(routerAdvisorWiring{router: r, root: root, bridge: &fixtures.FakeBridge{}, prompts: prompts.NewFromDir(t.TempDir()), signals: signalcenter.New()})
	})

	if err != nil || adv == nil {
		t.Fatalf("advisor %v err %v", adv, err)
	}
	if !strings.Contains(stderr, "[router] WARN router family and the claude fallback are both benched") {
		t.Errorf("stderr = %q, want the degraded-advisor warning", stderr)
	}
}

func TestWireRouterAdvisor_AHealthyFamilyDoesNotWarnAboutBenches(t *testing.T) {
	root := t.TempDir()
	e3Bench(t, root, "agy")
	r := e3AgyClaudeRouter(t, nil)

	stderr := e3CaptureStderr(t, func() {
		if adv, err := wireRouterAdvisor(routerAdvisorWiring{router: r, root: root, bridge: &fixtures.FakeBridge{}, prompts: prompts.NewFromDir(t.TempDir())}); err != nil || adv == nil {
			t.Errorf("advisor %v err %v", adv, err)
		}
	})

	if strings.Contains(stderr, "both benched") {
		t.Errorf("one benched family degraded the advisor: %q", stderr)
	}
}

func e3OverrideProfiles(t *testing.T, root string) {
	t.Helper()
	writeFileForPublishTest(t, filepath.Join(routingProfilesDir(root), "e3agent.json"), `{"name":"e3agent","cli":"agy-tmux","model_tier_default":"balanced"}`)
}

func TestOverrideRefusals_FaultsAreErrorsNotRefusals(t *testing.T) {
	t.Run("profiles path is a file", func(t *testing.T) {
		root := t.TempDir()
		writeFileForPublishTest(t, routingProfilesDir(root), "not a dir")
		refusals, err := overrideRefusals(e3AgyClaudeRouter(t, nil), root, map[string]string{"EVOLVE_CLI": "agy"})
		if err == nil || refusals != nil {
			t.Fatalf("refusals %v err %v, want the list error", refusals, err)
		}
	})
	t.Run("an overridden agent the table cannot route", func(t *testing.T) {
		root := t.TempDir()
		e3OverrideProfiles(t, root)
		block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Work: map[string][]string{"build": {"agy"}}}
		r, _, err := cliroute.Build(cliroute.Setup{Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(t.TempDir()),
			Host: cliroute.Host{LookPath: everyBinaryPresent}})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		refusals, err := overrideRefusals(r, root, map[string]string{"EVOLVE_CLI": "agy"})
		if err == nil || refusals != nil {
			t.Fatalf("refusals %v err %v, want the resolve error", refusals, err)
		}
	})
	t.Run("no override set means no resolve", func(t *testing.T) {
		root := t.TempDir()
		e3OverrideProfiles(t, root)
		refusals, err := overrideRefusals(e3AgyClaudeRouter(t, nil), root, map[string]string{})
		if err != nil || refusals != nil {
			t.Fatalf("refusals %v err %v", refusals, err)
		}
	})
}

func TestWireCLIRouter_AnOverrideCheckFaultIsReported(t *testing.T) {
	root := t.TempDir()
	boom := errors.New("profiles vanished mid-check")
	var gotEnv map[string]string
	prev := routingRunOverrideRefusals
	t.Cleanup(func() { routingRunOverrideRefusals = prev })
	routingRunOverrideRefusals = func(r *cliroute.Router, projectRoot string, env map[string]string) ([]string, error) {
		gotEnv = env
		if r == nil || projectRoot != root {
			t.Errorf("the check ran on router %v root %q", r, projectRoot)
		}
		return nil, boom
	}
	var console bytes.Buffer

	r, err := wireCLIRouter(routerSite{root: root}, routingRun{env: map[string]string{"EVOLVE_CLI": "agy"}}, &console)

	if r != nil || !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "checking the CLI overrides: ") {
		t.Fatalf("router %v err %v", r, err)
	}
	if gotEnv["EVOLVE_CLI"] != "agy" {
		t.Errorf("the check saw env %v, want the run's env", gotEnv)
	}
}

func TestProductionUsageEvidence_BuildsAnExplainerWithoutProbing(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	var log bytes.Buffer

	explain := productionUsageEvidence(root, evolveDir, &log)

	if explain == nil {
		t.Fatal("no explainer")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "usage-evidence")); !os.IsNotExist(err) {
		t.Errorf("building the explainer touched the probe workspace: %v", err)
	}
	if log.Len() != 0 {
		t.Errorf("building the explainer logged %q", log.String())
	}
}

func TestFamilyEvidence_AsksTheTmuxDriverOfTheFamily(t *testing.T) {
	if familyEvidence(nil) != nil {
		t.Fatal("no explainer must give no family evidence")
	}
	var asked []string
	explain := func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
		asked = append(asked, driver)
		return usageprobe.Evidence{Verdict: usageprobe.VerdictExhausted, Family: "agy"}
	}

	ev := familyEvidence(explain)(context.Background(), "agy")

	if ev.Verdict != usageprobe.VerdictExhausted || !reflect.DeepEqual(asked, []string{"agy-tmux"}) {
		t.Errorf("evidence %+v asked %v", ev, asked)
	}
}

func TestWithUsageEvidence_AFailedLaunchReportsToTheGivenSignalCenter(t *testing.T) {
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	inner := &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 1}}
	explain := func(context.Context, string, time.Time) usageprobe.Evidence {
		return usageprobe.Evidence{Verdict: usageprobe.VerdictExhausted, CLI: "agy", Family: "agy"}
	}

	res, _ := withUsageEvidence(inner, explain, center).Launch(context.Background(), core.BridgeRequest{CLI: "agy-tmux", Agent: "scout", Cycle: 3})
	center.Flush()

	if !res.UsageExhausted {
		t.Errorf("the response does not carry the exhausted verdict: %+v", res)
	}
	if len(events) != 1 || events[0].Code != usageevidence.CodeUsageEvidence || events[0].Cycle != 3 {
		t.Errorf("events = %+v, want one usage-evidence signal on the given center", events)
	}
}

func TestPhaseObserverAdapter_ReadsTheDepsItWasWiredWith(t *testing.T) {
	center := signalcenter.New()
	var asked string
	explain := func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
		asked = driver
		return usageprobe.Evidence{}
	}
	on := true

	built := phaseObserverAdapter(policy.ObserverPolicy{Autospawn: &on}, "enforce", observerDeps{signals: center, usage: explain})

	if built.Signals() != center || built.RecoveryStage != "enforce" || !*built.Config.Autospawn {
		t.Errorf("adapter = %+v, want the given center, stage and config", built)
	}
	built.UsageEvidence(context.Background(), "claude-tmux", time.Now())
	if asked != "claude-tmux" {
		t.Errorf("the adapter's usage evidence is not the given explainer (asked %q)", asked)
	}
	if other := phaseObserverAdapter(policy.ObserverPolicy{Autospawn: &on}, "shadow", observerDeps{}); other.Signals() != nil || other == built {
		t.Errorf("a second wiring shares state with the first: %+v", other)
	}
}
