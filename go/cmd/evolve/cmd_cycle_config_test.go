package main

// cmd_cycle_config_test.go — ADR-0103 unit 08: the routing-config Loader's
// one wired construction, the policy-stages projection, the Center-less
// facade sites pinned by name, and the render proofs at both roots.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// nonTestSourcesMatching lists the module's non-test Go files (outside the
// config leaf, which spells its own New without the package prefix) whose
// source matches re — a regexp so `\b` excludes phaseconfig.New( / .Load(.
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

// Test 38 — config.New is exported; ONE non-test file constructs the wired
// Loader (the seam), so a second construction can never bypass the Center.
func TestRoutingConfigLoader_OneConstructionSite(t *testing.T) {
	hits := nonTestSourcesMatching(t, regexp.MustCompile(`\bconfig\.New\(`))
	if len(hits) != 1 || hits[0] != "cmd/evolve/cmd_cycle_config.go" {
		t.Fatalf("config.New( belongs to cmd/evolve/cmd_cycle_config.go alone, found in %v", hits)
	}
}

// Test 41 — the silent facade config.Load( is kept by exactly the four
// Center-less sites; the cycle/loop root is NOT one of them.
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

// Test 45 (architecture-review fold) — the registry's location has ONE
// non-test spelling, config.RegistryPath: the walker skips internal/config/,
// so any hit is a re-inlined `"docs", "architecture", "phase-registry.json"`
// join (the six the review found: cmd_cycle_outputs.go, phaseinventory.go,
// phases_create.go, phase_order.go, skillcheck.go, kerneltest/fixture.go).
// phasespec's relative registryRelPath constant is the sibling parser's own
// belief and goes with the reader unification (unit doc F4).
func TestRegistryPathSpelling_HasOneNonTestHome(t *testing.T) {
	hits := nonTestSourcesMatching(t, regexp.MustCompile(`"docs",\s*"architecture",\s*"phase-registry\.json"`))
	if len(hits) != 0 {
		t.Fatalf("the docs/architecture/phase-registry.json join belongs to config.RegistryPath alone; re-inlined at %v", hits)
	}
}

// Test 42 — the hand-written [config] WARN line is gone from the root, and the
// two stage ladders live only in the seam as forwarders onto the leaf's.
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

// Test 40 — the projection onto the leaf's Parameter Object carries every
// accessor value, field for field (keyed: vet's composites check applies to
// imported struct literals).
func TestPolicyStagesOf_ProjectsThePolicyAccessors(t *testing.T) {
	pol := policy.Policy{
		Gates:            &policy.GatesPolicy{ContractGate: "shadow", EvalGate: "off", TriageCapGate: "shadow", ReviewGate: "enforce", TopNGate: "off"},
		Recovery:         &policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "shadow"},
		Router:           &policy.RouterPolicy{RouterReplan: "advisory", RoutingJudge: true, ReconDigest: true, ReplanDepth: 4},
		ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "shadow", Concurrency: 7},
	}
	got := policyStagesOf(pol.GatesConfig(), pol.RecoveryConfig(), pol.RouterConfig(), pol.ParallelEvaluateConfig())
	want := config.PolicyStages{
		ContractGate: "shadow", EvalGate: "off", TriageCapGate: "shadow", TopNGate: "off", ReviewGate: "enforce",
		PhaseRecovery: "enforce", SpineFloor: "shadow", RouterReplan: "advisory", ParallelEvaluate: "shadow",
		ParallelEvaluateConcurrency: 7, RoutingJudge: true, ReconDigest: true, RePlanMaxDepth: 4,
	}
	if got != want {
		t.Fatalf("policyStagesOf:\n got %+v\nwant %+v", got, want)
	}
}

// Test 39 — the production root: a malformed registry renders the module tag
// on the root's console (through the Center, not a hand-written line) and is
// durable in <evolveDir>/signals.ndjson (cycle-less). The sibling parser's
// `[phases] WARN builtin registry load failed` still goes to real stderr (F4).
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

// Test 39 (twin) — CENTER-TOPOLOGY parity only (Go-review fold): the
// --simulate root (wireSimulateOrchestrator, cmd_cycle_simulate.go) never
// resolves a RoutingConfig, so it never loads the registry and no CONFIG_*
// warning can originate from --simulate today — the first assertion pins
// that. What the test proves is that a Loader handed the simulate Center
// (the 02/03/03b twin shape) renders on its console and is durable under
// <evolveDir>, so a future Loader on that root inherits a working topology.
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

// Test 43 — `evolve solution` stays on the silent facade and hand-renders the
// slice, so the registry codes now reach its stderr too (D2b).
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
