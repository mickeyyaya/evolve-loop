package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

func wiringRoot(t *testing.T, policyJSON string) (string, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, evolveDir
}

func TestWireOrchestratorDeps_AnErrorFindingRefusesBeforeAnyDispatch(t *testing.T) {
	root, evolveDir := wiringRoot(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)
	var console bytes.Buffer
	d := wireOrchestratorDeps(root, evolveDir, &console, routingRun{})
	if d.RoutingErr == nil || d.Orchestrator != nil || !strings.Contains(d.RoutingErr.Error(), "cli_routing.clis") {
		t.Fatalf("a table with an error finding is refused at the composition root: err=%v orch=%v", d.RoutingErr, d.Orchestrator)
	}
	if !strings.Contains(console.String(), "[cli-routing] error cli_routing.clis") {
		t.Fatalf("every finding reaches the console: %q", console.String())
	}
}

func TestWireOrchestratorDeps_AMalformedPolicyIsRefusedBeforeAnyDispatch(t *testing.T) {
	root, evolveDir := wiringRoot(t, `{"pins": `)
	if d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}); d.RoutingErr == nil {
		t.Fatal("a malformed policy.json exits at start instead of failing each phase")
	}
}

func TestWireOrchestratorDeps_OneRouterReachesEveryLaunchPath(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root, evolveDir := wiringRoot(t, "")
	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	if d.RoutingErr != nil || d.Router == nil {
		t.Fatalf("a legacy tree routes: %v", d.RoutingErr)
	}
	if runner.DefaultRouter != d.Router || !d.Orchestrator.CLIRouterWired() {
		t.Fatal("the runners and the orchestrator share the one router the root compiled")
	}
}

func TestRunCycleRun_ARefusedRoutingTableExitsTwo(t *testing.T) {
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{RoutingErr: errors.New("the CLI routing table refuses to route: cli_routing.clis")}
	}
	var stdout, stderr bytes.Buffer
	rc := runCycleRun([]string{"--project-root", t.TempDir(), "--goal-hash", "abcd1234"}, &stdout, &stderr)
	if rc != exitRoutingRefused || !strings.Contains(stderr.String(), "cli_routing.clis") {
		t.Fatalf("rc=%d stderr=%q, want exit 2 naming the finding", rc, stderr.String())
	}
}

func TestRunLoop_ARefusedRoutingTableExitsTwo(t *testing.T) {
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{RoutingErr: errors.New("the CLI routing table refuses to route: cli_routing.clis")}
	}
	root, evolveDir := wiringRoot(t, "")
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--project-root", root, "--evolve-dir", evolveDir, "--goal-text", "x", "--cycles", "1", "--force-fresh", "--skip-preflight"}, nil, &stdout, &stderr)
	if rc != exitRoutingRefused || !strings.Contains(stderr.String(), "cli_routing.clis") {
		t.Fatalf("rc=%d stderr=%q, want exit 2 before the first cycle", rc, stderr.String())
	}
}

func TestCatalogPublisher_RecompilesTheRouterOnAMint(t *testing.T) {
	dir := t.TempDir()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r, _, err := cliroute.Build(cliroute.Setup{Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(dir),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "minted.json"), []byte(`{"name":"minted","cli":"claude-tmux","allowed_clis":["claude"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	catalogPublisher(&resolverSinkSpy{}, r)(phasespec.Catalog{})
	d, err := r.Resolve(cliroute.Request{Agent: "minted"})
	if err != nil || d.Plan.Candidates[0] != "claude-tmux" {
		t.Fatalf("a mint published mid-cycle swaps the router onto the new profile: %+v %v", d.Plan, err)
	}
}

func TestRecompileRouter_ARefusedMintWarnsAndKeepsTheTable(t *testing.T) {
	dir := t.TempDir()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy"}, AfterChain: "stop"}
	r, _, err := cliroute.Build(cliroute.Setup{Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "minted.json"), []byte(`{"name":"minted","cli":"codex-tmux","allowed_clis":["codex"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	recompileRouter(r, phasespec.Catalog{}, &warn)
	if !strings.Contains(warn.String(), "left the routing table unchanged") {
		t.Fatalf("a refused recompile is loud: %q", warn.String())
	}
	recompileRouter(nil, phasespec.Catalog{}, &warn)
}

func TestMemoized_RunsDiscoveryOnce(t *testing.T) {
	calls := 0
	discover := memoized(func() []string { calls++; return []string{"claude-tmux"} })
	discover()
	if got := discover(); calls != 1 || len(got) != 1 {
		t.Fatalf("discovery runs once per process: calls=%d got=%v", calls, got)
	}
}

func TestBuildCLIRouter_TheRootRouterCarriesTheProductionHost(t *testing.T) {
	fakeRoutingDoctor(t, doctorResult("claude-tmux", true, "ready"), doctorResult("agy-tmux", true, "ready"))
	root, evolveDir := wiringRoot(t, `{"workflow":{"universal_fallback_exclude":[]}}`)
	if err := os.WriteFile(filepath.Join(evolveDir, "profiles", "scout.json"), []byte(`{"name":"scout","cli":"codex-tmux"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, findings, err := loadCLIRouter(root, routingHost(io.Discard, time.Now))
	if err != nil || len(findings) != 0 {
		t.Fatalf("loadCLIRouter: %v %+v", err, findings)
	}
	d, err := r.Resolve(cliroute.Request{Agent: "scout", Phase: "scout", ProjectRoot: root})
	if err != nil || !strings.Contains(strings.Join(d.Plan.Candidates, " "), "agy-tmux") {
		t.Fatalf("the root's raw discovery feeds the tail and the router applies the workflow's ban: %v %v", d.Plan.Candidates, err)
	}
	role, err := roleResolver(r)("scout")
	if err != nil || role.CLI != "codex-tmux" {
		t.Fatalf("roleResolver keeps resolvellm's answer with no block: %+v %v", role, err)
	}
	reportRoutingFindings(io.Discard, []cliroute.Finding{{Severity: cliroute.SeverityWarn, Key: "k", Message: "m"}})
}

func TestPreflightRouting_BootsOnlyWhatTheResolvedChainsLaunch(t *testing.T) {
	root, evolveDir := wiringRoot(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)
	if err := os.WriteFile(filepath.Join(evolveDir, "profiles", "scout.json"), []byte(`{"name":"scout","cli":"codex-tmux","cli_fallback":["claude-tmux"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	routing, err := preflightRouting(root)
	if err != nil || strings.Join(routing.Drivers, " ") != "agy-tmux claude-tmux" {
		t.Fatalf("the preflight drivers are the table's, never the profile's codex: %v %v", routing.Drivers, err)
	}
	legacyRoot, legacyEvolve := wiringRoot(t, "")
	if err := os.WriteFile(filepath.Join(legacyEvolve, "profiles", "scout.json"), []byte(`{"name":"scout","cli":"codex-tmux","cli_fallback":["claude-tmux"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if legacy, err := preflightRouting(legacyRoot); err != nil || strings.Join(legacy.Drivers, " ") != "claude-tmux codex-tmux" {
		t.Fatalf("with no block the drivers stay the profile chains: %v %v", legacy.Drivers, err)
	}
}

func TestPreflightRouting_ARefusedTableCarriesItsFindings(t *testing.T) {
	root, _ := wiringRoot(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)
	routing, err := preflightRouting(root)
	if err == nil || len(routing.Findings) == 0 {
		t.Fatalf("the preflight check sees the refusal and every finding: %+v %v", routing, err)
	}
}

func TestDetectRouter_TheSetupDetectRootRendersThroughTheTable(t *testing.T) {
	root := routingFixture(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)
	var warn bytes.Buffer
	r := detectRouter(root, &warn)
	if r == nil || warn.Len() != 0 {
		t.Fatalf("a compiling table is injected: %v %q", r, warn.String())
	}
	if res, err := r.ResolveRole("scout", resolvellm.Options{}); err != nil || res.CLI != "agy-tmux" {
		t.Fatalf("detect renders scout from the table: %+v %v", res, err)
	}
	refused := routingFixture(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)
	if r := detectRouter(refused, &warn); r != nil || !strings.Contains(warn.String(), "refuses to route") {
		t.Fatalf("a refused table warns and leaves detect unresolved: %v %q", r, warn.String())
	}
}

func TestWireOrchestratorDeps_AProjectWithNoProfilesDirectoryRoutesAsBefore(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}); d.RoutingErr != nil || d.Orchestrator == nil {
		t.Fatalf("a fresh project with no cli_routing block and no profiles directory wires as before L1b: %v", d.RoutingErr)
	}
}

func fakeRoutingDoctor(t *testing.T, results ...gobridge.DoctorResult) {
	t.Helper()
	old := routingDoctor
	t.Cleanup(func() { routingDoctor = old })
	routingDoctor = func(context.Context) gobridge.DoctorReport { return gobridge.DoctorReport{Results: results} }
}

func TestOneProfilesDirectory_ThePreflightAndTheCycleCompileTheSameTable(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	block := `{"cli_routing":{"clis":["agy","codex","claude"],"after_chain":"stop"}}`
	root, plugin := routingFixture(t, block), routingFixture(t, block)
	for _, dir := range []string{root, plugin} {
		writeRoutingProfile(t, dir, "router", `{"name":"router","cli":"claude-tmux"}`)
	}
	writeRoutingProfile(t, root, "projectonly", `{"name":"projectonly","cli":"agy-tmux"}`)
	t.Setenv("EVOLVE_PLUGIN_ROOT", plugin)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	otherEvolve := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(filepath.Join(otherEvolve, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, otherEvolve, io.Discard, routingRun{})
	if d.RoutingErr != nil {
		t.Fatalf("the cycle compiles the project's profiles, not --evolve-dir's: %v", d.RoutingErr)
	}
	cycle, err := d.Router.Resolve(cliroute.Request{Agent: "projectonly"})
	if err != nil || strings.Join(cycle.Plan.Candidates, " ") != "agy-tmux" {
		t.Fatalf("cycle routes projectonly from the project's profile: %v %v", cycle.Plan.Candidates, err)
	}
	routing, err := loopPreflightOptions(loopConfig{ProjectRoot: root, EvolveDir: otherEvolve}, io.Discard).Routing()
	if err != nil || !slices.Contains(routing.Drivers, "agy-tmux") {
		t.Fatalf("the preflight compiles the same profiles as the cycle, not the plugin root's: %v %v", routing.Drivers, err)
	}
}

func writeRoutingProfile(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".evolve", "profiles", name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
