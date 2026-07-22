//go:build acs

// Package cycle1057 encodes the cycle-1057 acceptance criteria for
// `retro-artifact-budget-perphase` (retry of the cycle-1054 audit-FAIL):
// a per-phase bridge artifact-wait budget (`BridgePolicy.PhaseArtifactTimeoutS`,
// compiled default {"retrospective": 900, "retro": 900}) threaded
// policy → adapters/bridge.productionEngineDeps → bridge.Deps → Engine.Launch
// (arg vector `--artifact-timeout-s=N`) → parseLaunchArgs → Config.ArtifactTimeoutS
// → the existing tmux artifact-wait loop, while every other phase keeps the
// 300s builtin.
//
// Source incident: cycle-1048's retro was ctx-canceled at ~608s because the
// global 300s artifact deadline is too small for the grown retro contract
// (report + preventive_actions + disposition.json).
//
// Key correction over cycle-1054 (architecture-design.md Axis B): the live
// retro launch passes Agent: "retrospective" (internal/phases/retro/retro.go),
// NOT "retro" — a map keyed only on "retro" would be unit-green and live-dead.
// TestC1057_008 is the behavioral drift guard binding the compiled key to the
// label the real retro phase actually dispatches with.
//
// Every predicate here exercises the system under test (policy resolution, the
// real Engine.Launch dispatch path, the real production adapter root, the real
// retro phase). No source-grep assertions.
package cycle1057

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	adapterbridge "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// retroAgentLabel is the compiled key the fix must carry a 900s budget for.
// It is the label internal/phases/retro dispatches with; TestC1057_008 proves
// that binding behaviorally rather than by trusting this constant.
const retroAgentLabel = "retrospective"

// spyCLI is the --cli name of the recording driver registered below. Unique to
// this predicate package so it can never collide with a real driver.
const spyCLI = "acs-spy-cycle1057"

// spyDriver records the fully-resolved Config the Engine handed the driver.
// This is the observation point that makes the wiring proof honest: the value
// asserted on is the one that reached the driver AFTER Engine.Launch built the
// arg vector and parseLaunchArgs rebuilt the Config from it — not a hand-built
// Config literal, and not a Deps read (the cheapest gaming fakes, which these
// predicates must reject; the 950-vs-954 unit-green≠live-green lesson).
type spyDriver struct {
	mu   sync.Mutex
	seen []bridge.Config
}

func (d *spyDriver) Name() string { return spyCLI }

func (d *spyDriver) Launch(_ context.Context, cfg *bridge.Config, _ bridge.Deps) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen = append(d.seen, *cfg)
	return bridge.ExitOK, nil
}

// last returns the Config from the most recent dispatch.
func (d *spyDriver) last(t *testing.T) bridge.Config {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.seen) == 0 {
		t.Fatalf("driver was never dispatched — Engine.Launch did not reach the driver, so nothing was proven")
	}
	return d.seen[len(d.seen)-1]
}

var spy = &spyDriver{}

func init() { bridge.Register(spy) }

// launchFixture is a self-contained workspace + profile under t.TempDir():
// predicates never write to the live repo tree.
type launchFixture struct {
	ws       string
	profile  string
	artifact string
}

func newFixture(t *testing.T) launchFixture {
	t.Helper()
	ws := t.TempDir()
	profile := filepath.Join(ws, "profile.json")
	body := `{
  "name": "acs-cycle1057",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "permission_mode": "default",
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
`
	if err := os.WriteFile(profile, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return launchFixture{ws: ws, profile: profile, artifact: filepath.Join(ws, "artifact.md")}
}

func (fx launchFixture) request(agent string) core.BridgeRequest {
	return core.BridgeRequest{
		CLI:          spyCLI,
		Profile:      fx.profile,
		Model:        "auto",
		Prompt:       "acs cycle-1057 wiring probe",
		Workspace:    fx.ws,
		ArtifactPath: fx.artifact,
		Agent:        agent,
	}
}

// ---------------------------------------------------------------------------
// AC1 — compiled default + positive-override merge (R2/R4/R5)
// ---------------------------------------------------------------------------

func TestC1057_001_PolicyPhaseArtifactTimeoutDefaults(t *testing.T) {
	// A zero-value BridgePolicy is the REAL production state: the checked-in
	// .evolve/policy.json has no "bridge" block at all, so the compiled default
	// must resolve from the zero receiver (R4).
	got := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	if got[retroAgentLabel] != 900 {
		t.Errorf("compiled default %q budget = %d, want 900", retroAgentLabel, got[retroAgentLabel])
	}
	if got["retro"] != 900 {
		t.Errorf("compiled default \"retro\" alias budget = %d, want 900 — the phase/agent-label skew "+
			"(core/routing_dispatch.go) is permanent, so both vocabularies must carry the budget", got["retro"])
	}
	for _, phase := range []string{"build", "audit", "scout", "tdd-engineer", "builder"} {
		if v := got[phase]; v != 0 {
			t.Errorf("phase %q resolved %d, want 0 — an unlisted phase must fall through to the 300s "+
				"builtin; global hang detection must not be weakened to fix one phase", phase, v)
		}
	}
}

func TestC1057_002_PhaseArtifactTimeout_PositiveMergeFromJSON(t *testing.T) {
	// An operator block ADDS/RAISES entries; it never erases a compiled entry,
	// and it never touches the global artifact_timeout_s.
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	body := `{"bridge": {"phase_artifact_timeout_s": {"build": 600}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	pol, err := policy.Load(path)
	if err != nil {
		t.Fatalf("policy.Load: %v", err)
	}
	bc := pol.BridgeConfig()
	got := bc.PhaseArtifactTimeouts()
	if got["build"] != 600 {
		t.Errorf("override build = %d, want 600", got["build"])
	}
	if got[retroAgentLabel] != 900 {
		t.Errorf("compiled %q entry = %d after an unrelated override, want 900 (positive merge must not erase it)",
			retroAgentLabel, got[retroAgentLabel])
	}
	if bc.ArtifactTimeoutS != 0 {
		t.Errorf("global artifact_timeout_s = %d, want 0 — the per-phase map must never bleed into the global budget",
			bc.ArtifactTimeoutS)
	}

	// A raise of the compiled key is honored.
	raised := policy.BridgePolicy{PhaseArtifactTimeoutS: map[string]int{retroAgentLabel: 1200}}.PhaseArtifactTimeouts()
	if raised[retroAgentLabel] != 1200 {
		t.Errorf("operator raise of %q = %d, want 1200", retroAgentLabel, raised[retroAgentLabel])
	}

	// The resolver must return a FRESH map each call: a caller mutating the
	// result must not poison the next resolution (shared-state defect class).
	first := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	if first == nil {
		t.Fatalf("PhaseArtifactTimeouts returned a nil map — it must return a fresh, populated map")
	}
	first[retroAgentLabel] = 1
	if second := (policy.BridgePolicy{}).PhaseArtifactTimeouts(); second[retroAgentLabel] != 900 {
		t.Errorf("after caller mutation, a fresh resolve gave %q = %d, want 900 — the resolver must not "+
			"alias a package-level map", retroAgentLabel, second[retroAgentLabel])
	}
}

// ---------------------------------------------------------------------------
// AC2 — NEGATIVE: non-positive / malformed entries are rejected (R5)
// ---------------------------------------------------------------------------

func TestC1057_003_PhaseArtifactTimeout_InvalidRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   map[string]int
	}{
		{"zero", map[string]int{retroAgentLabel: 0}},
		{"negative", map[string]int{retroAgentLabel: -5}},
		{"large-negative", map[string]int{retroAgentLabel: -100000}},
	} {
		got := policy.BridgePolicy{PhaseArtifactTimeoutS: tc.in}.PhaseArtifactTimeouts()
		if got[retroAgentLabel] != 900 {
			t.Errorf("%s: %q resolved %d, want 900 — a non-positive override must not lower the compiled default",
				tc.name, retroAgentLabel, got[retroAgentLabel])
		}
	}

	// A non-positive entry for an unlisted phase resolves 0, never negative:
	// a negative deadline is never a valid budget.
	got := policy.BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"build": -30}}.PhaseArtifactTimeouts()
	if got["build"] != 0 {
		t.Errorf("negative build override resolved %d, want 0", got["build"])
	}

	// A non-integer JSON entry must not corrupt resolution: either Load errors,
	// or the compiled default survives intact. Silently accepting garbage fails.
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	bad := `{"bridge": {"phase_artifact_timeout_s": {"retrospective": "abc"}}}`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	if pol, err := policy.Load(path); err == nil {
		if v := pol.BridgeConfig().PhaseArtifactTimeouts()[retroAgentLabel]; v != 900 {
			t.Errorf("non-integer entry accepted: %q = %d, want 900", retroAgentLabel, v)
		}
	}
}

// ---------------------------------------------------------------------------
// AC3 — live-path wiring proof through the real Engine.Launch (R1/R3)
// ---------------------------------------------------------------------------

func TestC1057_004_ArtifactTimeout_RetroLaunchCarries900(t *testing.T) {
	fx := newFixture(t)
	eng := bridge.NewEngine(bridge.Deps{
		PhaseArtifactTimeoutS: policy.BridgePolicy{}.PhaseArtifactTimeouts(),
	})

	if _, err := eng.Launch(context.Background(), fx.request(retroAgentLabel)); err != nil {
		t.Logf("Launch returned err=%v (expected: the spy driver writes no artifact)", err)
	}
	if got := spy.last(t).ArtifactTimeoutS; got != 900 {
		t.Errorf("%s launch: Config.ArtifactTimeoutS observed by the driver = %d, want 900 — the per-phase "+
			"budget must survive Engine.Launch → arg vector → parseLaunchArgs", retroAgentLabel, got)
	}

	if _, err := eng.Launch(context.Background(), fx.request("build")); err != nil {
		t.Logf("Launch returned err=%v (expected)", err)
	}
	if got := spy.last(t).ArtifactTimeoutS; got != 0 {
		t.Errorf("build launch: Config.ArtifactTimeoutS = %d, want 0 — an unlisted phase must fall through "+
			"to the 300s builtin, not inherit retro's budget", got)
	}
}

// TestC1057_005 pins the PARSE side of the arg-vector transport independently
// of the emit side: the flag must be a known flag (an unknown flag is
// ExitBadFlags, which would kill every retro launch), in both = and space
// forms, and a garbage value must be permissive (0), matching --cycle.
func TestC1057_005_ArtifactTimeout_FlagParsedFromArgVector(t *testing.T) {
	fx := newFixture(t)
	promptFile := filepath.Join(fx.ws, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("acs cycle-1057 parse probe"), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	eng := bridge.NewEngine(bridge.Deps{})

	base := []string{
		"--cli", spyCLI,
		"--profile", fx.profile,
		"--model", "haiku",
		"--prompt-file", promptFile,
		"--workspace", fx.ws,
		"--stdout-log", filepath.Join(fx.ws, "stdout.log"),
		"--stderr-log", filepath.Join(fx.ws, "stderr.log"),
		"--artifact", fx.artifact,
		"--agent", retroAgentLabel,
	}

	for _, tc := range []struct {
		name  string
		extra []string
		want  int
	}{
		{"equals-form", []string{"--artifact-timeout-s=900"}, 900},
		{"space-form", []string{"--artifact-timeout-s", "900"}, 900},
		{"garbage-value", []string{"--artifact-timeout-s=not-a-number"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, base...), tc.extra...)
			rc := eng.LaunchArgs(context.Background(), args, nil, os.Stdout, os.Stderr)
			if rc == bridge.ExitBadFlags {
				t.Fatalf("LaunchArgs rejected %v as bad flags (rc=%d) — --artifact-timeout-s must be a KNOWN "+
					"flag; an unparsed flag makes every retro launch die ExitBadFlags", tc.extra, rc)
			}
			if got := spy.last(t).ArtifactTimeoutS; got != tc.want {
				t.Errorf("%s: Config.ArtifactTimeoutS = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestC1057_006_ArtifactTimeout_ProductionAdapterWiresPolicy(t *testing.T) {
	// Production-root proof (R8): adapters/bridge.NewDefault is the composition
	// root every phase launch goes through; its productionEngineDeps must feed
	// the policy-resolved map. A policy.json with NO bridge block mirrors the
	// real repo, so the compiled default has to survive the zero value.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	fx := newFixture(t)
	a := adapterbridge.NewDefault(root)
	if _, err := a.Launch(context.Background(), fx.request(retroAgentLabel)); err != nil {
		t.Logf("Launch returned err=%v (expected)", err)
	}
	if got := spy.last(t).ArtifactTimeoutS; got != 900 {
		t.Errorf("production adapter root: %s launch Config.ArtifactTimeoutS = %d, want 900 — "+
			"productionEngineDeps must thread BridgePolicy.PhaseArtifactTimeouts() into bridge.Deps",
			retroAgentLabel, got)
	}

	// And an operator override reaches the same live path.
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	body := `{"bridge": {"phase_artifact_timeout_s": {"` + retroAgentLabel + `": 1500}}}`
	if err := os.WriteFile(filepath.Join(root2, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	if _, err := adapterbridge.NewDefault(root2).Launch(context.Background(), fx.request(retroAgentLabel)); err != nil {
		t.Logf("Launch returned err=%v (expected)", err)
	}
	if got := spy.last(t).ArtifactTimeoutS; got != 1500 {
		t.Errorf("policy.json override through the production root: Config.ArtifactTimeoutS = %d, want 1500", got)
	}
}

// ---------------------------------------------------------------------------
// AC4 — EDGE/OOD: unknown phase, empty agent, nil map fail open (R6)
// ---------------------------------------------------------------------------

func TestC1057_007_ArtifactTimeout_UnknownPhaseFailsOpen(t *testing.T) {
	fx := newFixture(t)
	for _, tc := range []struct {
		name  string
		agent string
		deps  bridge.Deps
	}{
		{"empty-agent", "", bridge.Deps{PhaseArtifactTimeoutS: map[string]int{retroAgentLabel: 900}}},
		{"unknown-phase", "not-a-phase", bridge.Deps{PhaseArtifactTimeoutS: map[string]int{retroAgentLabel: 900}}},
		{"nil-map-retro", retroAgentLabel, bridge.Deps{}},
		{"empty-map-retro", retroAgentLabel, bridge.Deps{PhaseArtifactTimeoutS: map[string]int{}}},
		{"non-positive-entry", retroAgentLabel, bridge.Deps{PhaseArtifactTimeoutS: map[string]int{retroAgentLabel: -1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng := bridge.NewEngine(tc.deps) // must not panic on a nil/empty map
			if _, err := eng.Launch(context.Background(), fx.request(tc.agent)); err != nil {
				t.Logf("Launch returned err=%v (expected)", err)
			}
			if got := spy.last(t).ArtifactTimeoutS; got != 0 {
				t.Errorf("%s: Config.ArtifactTimeoutS = %d, want 0 (fail open to the 300s builtin)", tc.name, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC5 — DRIFT GUARD: the compiled key is the label retro actually launches with
// ---------------------------------------------------------------------------

// captureBridge records the BridgeRequest the retro phase dispatches, so the
// agent label under test is the one the REAL phase produces — not a constant
// copied from a report. This is the anti-recurrence proof for the cycle-1054
// defect class (compiled key "retro" vs live label "retrospective"): a rename
// on either side turns this predicate RED instead of silently restoring the
// 300s timeout in production.
type captureBridge struct {
	req core.BridgeRequest
	hit bool
}

func (c *captureBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	c.req = req
	c.hit = true
	return core.BridgeResponse{ExitCode: bridge.ExitOK}, nil
}

func (c *captureBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestC1057_008_RetroPhaseAgentLabelCarriesTheBudget(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ws := t.TempDir()
	cap := &captureBridge{}
	p := retro.New(retro.Config{
		Bridge:  cap,
		Prompts: prompts.NewForProject(root),
	})

	// previous_verdict must be FAIL/WARN or retro SKIPs without dispatching.
	_, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle:       1057,
		Workspace:   ws,
		Worktree:    ws,
		ProjectRoot: root,
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Logf("retro.Run returned err=%v (the capture bridge writes no artifact)", err)
	}
	if !cap.hit {
		t.Fatalf("retro phase never dispatched a BridgeRequest — the drift guard proved nothing")
	}

	agent := cap.req.Agent
	if agent == "" {
		t.Fatalf("retro dispatched with an empty Agent label")
	}
	budgets := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	if budgets[agent] != 900 {
		t.Errorf("the retro phase dispatches Agent=%q but the compiled per-phase budget for that key is %d "+
			"(want 900) — the budget is keyed on a label no real retro launch carries, so production would "+
			"silently keep the 300s deadline (the cycle-1054 unit-green/live-dead defect)", agent, budgets[agent])
	}
}
