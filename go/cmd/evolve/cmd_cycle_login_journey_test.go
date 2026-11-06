package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const quotaWallFrame = "⏺ working\n\nClaude usage limit reached. Your limit will reset at 9pm.\n\n❯ \n"

type journeyHooks struct{}

func (journeyHooks) PhaseName() string       { return "scout" }
func (journeyHooks) AgentPromptName() string { return "evolve-scout" }
func (journeyHooks) ArtifactFilename(core.PhaseRequest) string {
	return "scout-report.md"
}
func (journeyHooks) DefaultModel() string { return "sonnet" }
func (journeyHooks) ComposePrompt(body string, _ core.PhaseRequest) string {
	return body
}
func (journeyHooks) Classify(string, core.PhaseRequest, core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, "triage"
}

type paneStubBridge struct {
	t     *testing.T
	frame string
	calls []string
}

func (b *paneStubBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req.CLI)
	m, err := bridge.LoadManifest("claude-tmux")
	if err != nil {
		b.t.Fatalf("manifest claude-tmux: %v", err)
	}
	for _, p := range m.InteractivePrompts {
		if p.Policy != "escalate" || p.Regex == "" {
			continue
		}
		subject := b.frame
		if p.TailLines > 0 {
			lines := strings.Split(strings.TrimRight(subject, "\n"), "\n")
			if len(lines) > p.TailLines {
				lines = lines[len(lines)-p.TailLines:]
			}
			subject = strings.Join(lines, "\n")
		}
		if regexp.MustCompile(p.Regex).MatchString(subject) {
			return core.BridgeResponse{ExitCode: 85, CauseCode: p.Name, Stderr: "escalate:" + p.Name},
				fmt.Errorf("bridge: launch exit=85: %w", core.ErrTransientBridgeFailure)
		}
	}
	b.t.Fatalf("the stub CLI frame matched no escalate rule of claude-tmux: %q", b.frame)
	return core.BridgeResponse{}, nil
}

func (b *paneStubBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func realLoginFrame(t *testing.T) string {
	t.Helper()
	frame, err := os.ReadFile(filepath.Join("..", "..", "internal", "bridge", "testdata", "claude-login-expired-frame.txt"))
	if err != nil {
		t.Fatalf("the recorded login frame is missing: %v", err)
	}
	return string(frame)
}

type journeySetup struct {
	bridge  core.Bridge
	profile string
	fleet   bool
}

type journeyRun struct {
	rc        int
	stdout    string
	stderr    string
	events    []signalcenter.Event
	root      string
	evolveDir string
	claim     string
	pending   string
}

func runJourney(t *testing.T, j journeySetup) journeyRun {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	origRouter := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = origRouter })
	runner.DefaultRouter = nil
	if j.fleet {
		t.Setenv(ipcenv.FleetKey, "1")
	}
	root := t.TempDir()
	initLoopContractRepo(t, root)
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 1)
	claim, pending := claimedInboxItem(t, root, 1, "journey-item")
	profiles := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profiles, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "scout.json"), []byte(j.profile), 0o644); err != nil {
		t.Fatal(err)
	}
	st := storage.New(evolveDir)
	if err := st.WriteState(context.Background(), core.State{}); err != nil {
		t.Fatal(err)
	}
	center, got := recordedCenter()
	fake := newFakeLedger()
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		agents := fstest.MapFS{"agents/evolve-scout.md": &fstest.MapFile{Data: []byte("---\nname: evolve-scout\n---\nscout")}}
		scout := runner.New(runner.Options{
			Hooks:               journeyHooks{},
			Bridge:              j.bridge,
			Prompts:             prompts.NewFromFS(agents),
			SleepFn:             func(time.Duration) {},
			NowFn:               time.Now,
			Signals:             func() *signalcenter.Center { return center },
			DisableStdoutFilter: true,
		})
		runners := map[core.Phase]core.PhaseRunner{}
		for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
			runners[p] = noopRunner{name: string(p)}
		}
		runners[core.PhaseScout] = scout
		orch := core.NewOrchestrator(st, fake, runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}), core.WithSignalCenter(center))
		return orchDeps{Storage: st, Ledger: fake, Orchestrator: orch, Signals: center}
	}
	var out, errBuf bytes.Buffer
	rc := runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234"}, &out, &errBuf)
	return journeyRun{rc: rc, stdout: out.String(), stderr: errBuf.String(), events: *got, root: root, evolveDir: evolveDir, claim: claim, pending: pending}
}

func runCycleRunThroughStubCLI(t *testing.T, frame string) (rc int, stdout, stderr string, events []signalcenter.Event, stub *paneStubBridge) {
	t.Helper()
	stub = &paneStubBridge{t: t, frame: frame}
	run := runJourney(t, journeySetup{bridge: stub, profile: `{"name":"evolve-scout","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"sonnet"}`})
	return run.rc, run.stdout, run.stderr, run.events, stub
}

type mixedFamilyBridge struct{ calls []string }

func (b *mixedFamilyBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req.CLI)
	cause := "rate_limit"
	if strings.HasPrefix(req.CLI, "claude") {
		cause = "auth_recheck"
	}
	return core.BridgeResponse{ExitCode: 85, CauseCode: cause, Stderr: "escalate:" + cause},
		fmt.Errorf("bridge: launch exit=85: %w", core.ErrTransientBridgeFailure)
}

func (b *mixedFamilyBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func eventsOfKind(events []signalcenter.Event, kind signalcenter.Kind) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func TestCycleRunJourney_ALogoutThroughTheRealEntryEndsWithTheLoginSignal(t *testing.T) {
	rc, stdout, stderr, events, stub := runCycleRunThroughStubCLI(t, realLoginFrame(t))

	if len(stub.calls) == 0 {
		t.Fatalf("the stub CLI was never launched; stderr=%s", stderr)
	}
	if rc != 5 {
		t.Errorf("rc=%d, want the wall exit 5; stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, `"login-required"`) {
		t.Errorf("stdout lacks the stop reason login-required: %s", stdout)
	}
	login := eventsOfKind(events, signalcenter.KindLoginRequired)
	if len(login) != 1 || !strings.Contains(login[0].Reason, "/login") {
		t.Fatalf("login events=%+v, want one naming /login", login)
	}
	if paused := eventsOfKind(events, signalcenter.KindQuotaPaused); len(paused) != 0 {
		t.Errorf("a logout must not read as quota: %+v", paused)
	}
}

func TestCycleRunJourney_ARealQuotaWallThroughTheSamePathKeepsItsClass(t *testing.T) {
	rc, stdout, stderr, events, stub := runCycleRunThroughStubCLI(t, quotaWallFrame)

	if len(stub.calls) == 0 {
		t.Fatalf("the stub CLI was never launched; stderr=%s", stderr)
	}
	if rc != 5 {
		t.Errorf("rc=%d, want the wall exit 5; stderr=%s", rc, stderr)
	}
	if strings.Contains(stdout, `"login-required"`) || !strings.Contains(stdout, `"quota-pause"`) {
		t.Errorf("a quota wall must keep quota-pause: %s", stdout)
	}
	if login := eventsOfKind(events, signalcenter.KindLoginRequired); len(login) != 0 {
		t.Errorf("a quota wall must not name a login: %+v", login)
	}
	if paused := eventsOfKind(events, signalcenter.KindQuotaPaused); len(paused) != 1 {
		t.Errorf("a quota wall must emit one quota.paused through the wired center: %+v", paused)
	}
	if !strings.Contains(stderr, "claude is on quota") {
		t.Errorf("a quota wall must read as quota on the pause line: %s", stderr)
	}
}

func TestCycleRunJourney_AFleetLaneWithAQuotaFamilyAndALoggedOutFamilyReleasesItsClaimAndNamesBoth(t *testing.T) {
	stub := &mixedFamilyBridge{}
	run := runJourney(t, journeySetup{
		bridge:  stub,
		profile: `{"name":"evolve-scout","cli":"agy-tmux","cli_fallback":["claude-tmux"],"model_tier_default":"sonnet"}`,
		fleet:   true,
	})

	if want := []string{"agy-tmux", "claude-tmux", "agy-tmux", "claude-tmux"}; !slices.Equal(stub.calls, want) {
		t.Errorf("calls = %v, want %v", stub.calls, want)
	}
	if run.rc != 5 || !strings.Contains(run.stdout, `"login-required"`) {
		t.Errorf("rc=%d stdout=%s, want the wall exit 5 and login-required", run.rc, run.stdout)
	}
	wantText := "cycle 1 phase scout: agy is on quota, reset unknown; claude needs a login. Impact: the cycle is paused; claims released. Next: open claude and run /login, or wait for agy, reset unknown. Then evolve loop --resume."
	login := eventsOfKind(run.events, signalcenter.KindLoginRequired)
	if len(login) != 1 || login[0].Reason != wantText {
		t.Fatalf("login events = %+v, want one with the reason %q", login, wantText)
	}
	if paused := eventsOfKind(run.events, signalcenter.KindQuotaPaused); len(paused) != 1 {
		t.Errorf("a wall that holds a quota row emits one quota.paused: %+v", paused)
	}
	if _, err := os.Stat(run.claim); err == nil {
		t.Errorf("the claim %s is still held after the deferral", run.claim)
	}
	if _, err := os.Stat(run.pending); err != nil {
		t.Errorf("the item is not pending again: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(run.evolveDir, "runs", "cycle-1", "cycle-state.json"))
	if err != nil {
		t.Fatalf("the per-run checkpoint is missing: %v", err)
	}
	if !strings.Contains(string(raw), `"quotaWall"`) {
		t.Errorf("the per-run checkpoint lacks quotaWall: %s", raw)
	}
}
