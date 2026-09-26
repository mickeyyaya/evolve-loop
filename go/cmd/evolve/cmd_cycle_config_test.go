package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// nonTestSourcesMatching lists the module's non-test Go files outside
// internal/config whose source matches re; `\b` excludes phaseconfig.New(.
func nonTestSourcesMatching(t *testing.T, re *regexp.Regexp) []string {
	t.Helper()
	moduleRoot := filepath.Join("..", "..")
	var hits []string
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || name == "testdata" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(rel, "internal/config/") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if re.Match(src) {
			hits = append(hits, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hits
}

func TestRoutingConfigLoader_OneConstructionSite(t *testing.T) {
	hits := nonTestSourcesMatching(t, regexp.MustCompile(`\bconfig\.New\(`))
	if len(hits) != 1 || hits[0] != "cmd/evolve/cmd_cycle_config.go" {
		t.Fatalf("config.New( belongs to cmd/evolve/cmd_cycle_config.go alone, found in %v", hits)
	}
}

func TestCenterlessConfigLoadSitesArePinned(t *testing.T) {
	want := map[string]bool{
		"internal/router/policy.go":             true, // per-phase self-skip policy (Q5)
		"internal/cli/phasecmd/phase_verify.go": true, // the agent's self-check
		"cmd/evolve/cmd_solution.go":            true, // hand-rendered, no Center (F7)
		"internal/kerneltest/fixture.go":        true, // test machinery
		"internal/dashboard/plan.go":            true, // the read-only board's registry read: no Center, registry warnings ride the snapshot's Warnings
	}
	hits := nonTestSourcesMatching(t, regexp.MustCompile(`\bconfig\.Load\(`))
	got := map[string]bool{}
	for _, h := range hits {
		got[h] = true
		if !want[h] {
			t.Errorf("%s calls the Center-less config.Load( facade: a new site must be declared here or wire a Loader", h)
		}
	}
	for site := range want {
		if !got[site] {
			t.Errorf("%s no longer calls config.Load( — update the pinned list", site)
		}
	}
}

func TestRegistryPathSpelling_HasOneNonTestHome(t *testing.T) {
	hits := nonTestSourcesMatching(t, regexp.MustCompile(`"docs",\s*"architecture",\s*"phase-registry\.json"`))
	if len(hits) != 0 {
		t.Fatalf("the docs/architecture/phase-registry.json join belongs to config.RegistryPath alone; re-inlined at %v", hits)
	}
}

func TestCmdCycle_NoHandWrittenConfigWarnLineAndLaddersLiveInTheSeam(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), `"[config] WARN`) {
			t.Errorf("%s still renders config warnings by hand; the Center's StderrSink renders config.warning", name)
		}
		defines := strings.Contains(string(src), "func parseGateStage(") || strings.Contains(string(src), "func parseRouterStage(")
		if defines && name != "cmd_cycle_config.go" {
			t.Errorf("%s defines a stage ladder; the forwarders live in cmd_cycle_config.go", name)
		}
	}
	seam, err := os.ReadFile("cmd_cycle_config.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"func parseGateStage(", "config.GateStage(", "func parseRouterStage(", "config.RouterStage("} {
		if !strings.Contains(string(seam), needle) {
			t.Errorf("cmd_cycle_config.go must forward onto the leaf's ladders: missing %s", needle)
		}
	}
}

// Keyed fields: vet's composites check applies to imported struct literals.
func TestPolicyStagesOf_ProjectsThePolicyAccessors(t *testing.T) {
	pol := policy.Policy{
		Gates:            &policy.GatesPolicy{ContractGate: "shadow", EvalGate: "off", TriageCapGate: "shadow", ReviewGate: "enforce", TopNGate: "off"},
		Recovery:         &policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "shadow", FatalPane: "shadow"},
		Router:           &policy.RouterPolicy{RouterReplan: "advisory", RoutingJudge: true, ReconDigest: true, ReplanDepth: 4},
		ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "shadow", Concurrency: 7},
	}
	got := policyStagesOf(pol.GatesConfig(), pol.RecoveryConfig(), pol.RouterConfig(), pol.ParallelEvaluateConfig())
	want := config.PolicyStages{
		ContractGate: "shadow", EvalGate: "off", TriageCapGate: "shadow", TopNGate: "off", ReviewGate: "enforce",
		PhaseRecovery: "enforce", SpineFloor: "shadow", FatalPane: "shadow", RouterReplan: "advisory", ParallelEvaluate: "shadow",
		ParallelEvaluateConcurrency: 7, RoutingJudge: true, ReconDigest: true, RePlanMaxDepth: 4,
	}
	if got != want {
		t.Fatalf("policyStagesOf:\n got %+v\nwant %+v", got, want)
	}
}

func TestWireOrchestratorDeps_ConfigWarningRendersAndIsDurable(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := config.RegistryPath(root)
	if err := os.MkdirAll(filepath.Dir(reg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	console := captureConsole(func(c io.Writer) { wireOrchestratorDeps(root, evolveDir, c) })
	for _, want := range []string{"[config] config.warning WARN CONFIG_REGISTRY_MALFORMED", "origin=Loader.Load", "step=registry", "phase registry malformed"} {
		if !strings.Contains(console, want) {
			t.Errorf("the root console must carry %q:\n%s", want, console)
		}
	}
	if strings.Contains(console, "[config] WARN registry-malformed:") || strings.Contains(console, "cycle=") {
		t.Errorf("the old hand-written line and a cycle stamp must not appear:\n%s", console)
	}
	data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson"))
	if err != nil || !strings.Contains(string(data), `"code":"CONFIG_REGISTRY_MALFORMED"`) {
		t.Errorf("the cycle-less signal is durable under <evolveDir>: %v %s", err, data)
	}
}

// --simulate resolves no RoutingConfig, so no CONFIG_* warning can originate
// there; this proves the topology a future Loader on that root would inherit.
func TestWireSimulateOrchestrator_CenterTopologyRendersConfigWarnings(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := config.RegistryPath(root)
	if err := os.MkdirAll(reg, 0o755); err != nil { // a directory at the path: a read fault, not absence
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	if out := console.String(); strings.Contains(out, "[config]") {
		t.Fatalf("wiring the --simulate root must not load the registry (it resolves no RoutingConfig); a config line rendered: %q", out)
	}
	l := config.New(config.WithSignals(func() *signalcenter.Center { return d.Signals }))
	if _, ws := l.Load(reg, nil); len(ws) != 1 || ws[0].Code != "registry-unreadable" {
		t.Fatalf("warnings: %+v", ws)
	}
	if out := console.String(); !strings.Contains(out, "[config]") || !strings.Contains(out, "CONFIG_REGISTRY_UNREADABLE") || !strings.Contains(out, "origin=Loader.Load") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"CONFIG_REGISTRY_UNREADABLE"`) {
		t.Errorf("the cycle-less signal is durable: %v %s", err, data)
	}
}

func TestSolutionCmd_MalformedRegistryPrintsTheRegistryWarning(t *testing.T) {
	root := t.TempDir()
	reg := config.RegistryPath(root)
	if err := os.MkdirAll(filepath.Dir(reg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := dispatch([]string{"solution", "check", "some-slug", "--project-root", root}, nil, &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "evolve solution: registry warning [registry-malformed]: phase registry malformed") || !strings.Contains(errb.String(), "declares no config.deliverable_kinds.document") {
		t.Fatalf("exit %d, stderr:\n%s", code, errb.String())
	}
}

// fakeBridgeStageSink records what the root forwards into the bridge adapter.
type fakeBridgeStageSink struct {
	phaseIO             config.Stage
	recovery, fatalPane string
}

func (f *fakeBridgeStageSink) SetPhaseIOStage(s config.Stage) { f.phaseIO = s }
func (f *fakeBridgeStageSink) SetRecoveryStage(s string)      { f.recovery = s }
func (f *fakeBridgeStageSink) SetFatalPaneStage(s string)     { f.fatalPane = s }

func TestWireBridgeStages_ForwardsEachDialOnItsOwnSetter(t *testing.T) {
	var cfg config.RoutingConfig
	cfg.PhaseIO, cfg.PhaseRecovery, cfg.FatalPane = config.StageAdvisory, config.StageShadow, config.StageEnforce
	var sink fakeBridgeStageSink
	wireBridgeStages(&sink, cfg)
	if sink.phaseIO != config.StageAdvisory || sink.recovery != "shadow" || sink.fatalPane != "enforce" {
		t.Fatalf("wireBridgeStages forwarded phaseIO=%v recovery=%q fatalPane=%q, want advisory/shadow/enforce",
			sink.phaseIO, sink.recovery, sink.fatalPane)
	}
}

func TestWireBridgeStages_IsTheRootsOnlyStageForwarding(t *testing.T) {
	for _, re := range []string{`\.SetPhaseIOStage\(`, `\.SetRecoveryStage\(`, `\.SetFatalPaneStage\(`} {
		hits := nonTestSourcesMatching(t, regexp.MustCompile(re))
		if len(hits) != 1 || hits[0] != "cmd/evolve/cmd_cycle_config.go" {
			t.Errorf("%s belongs to cmd/evolve/cmd_cycle_config.go alone, found in %v", re, hits)
		}
	}
	// The cycle root must also call the forwarder: NewDefault's policy seeding
	// would mask a dropped call while losing the Loader's validation.
	hits := nonTestSourcesMatching(t, regexp.MustCompile(`\bwireBridgeStages\(`))
	want := []string{"cmd/evolve/cmd_cycle.go", "cmd/evolve/cmd_cycle_config.go"}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("wireBridgeStages( must be defined in cmd_cycle_config.go and called by cmd_cycle.go, found in %v", hits)
	}
}
