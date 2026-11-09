//go:build acs

package cycle2243

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type planWritingLauncher struct{ plan string }

func (l planWritingLauncher) Launch(_ context.Context, req advisor.LaunchRequest) (advisor.LaunchResponse, error) {
	if err := os.WriteFile(req.ArtifactPath, []byte(l.plan), 0o644); err != nil {
		return advisor.LaunchResponse{}, err
	}
	return advisor.LaunchResponse{Stdout: l.plan}, nil
}

type planner func(*advisor.Advisor, router.RouteInput) (*router.PhasePlan, error)

func viaPlan(a *advisor.Advisor, in router.RouteInput) (*router.PhasePlan, error) { return a.Plan(in) }

func viaRePlan(a *advisor.Advisor, in router.RouteInput) (*router.PhasePlan, error) {
	return a.RePlan(in)
}

func advisorProposing(buildTier string) *advisor.Advisor {
	plan := fmt.Sprintf(`[{"phase":"scout","run":true,"justification":"x"},{"phase":"build","run":true,"justification":"x","tier":%q}]`, buildTier)
	return advisor.New(planWritingLauncher{plan}, advisor.Identity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"}, nil)
}

func routeInput(t *testing.T, cfg config.RoutingConfig, goal string, items ...router.LaneItem) router.RouteInput {
	t.Helper()
	return router.RouteInput{
		Current:     "triage",
		Verdict:     "PASS",
		Workspace:   t.TempDir(),
		ProjectRoot: t.TempDir(),
		Cycle:       1,
		Cfg:         cfg,
		GoalText:    goal,
		LaneItems:   items,
	}
}

func buildTier(t *testing.T, run planner, cfg config.RoutingConfig, proposed, goal string, items ...router.LaneItem) string {
	t.Helper()
	plan, err := run(advisorProposing(proposed), routeInput(t, cfg, goal, items...))
	if err != nil {
		t.Fatalf("advisor plan failed: %v", err)
	}
	for _, e := range plan.Entries {
		if e.Phase == "build" {
			return e.Tier
		}
	}
	t.Fatalf("plan has no build entry: %+v", plan.Entries)
	return ""
}

func registryCarrying(t *testing.T, classes map[string][]string) config.RoutingConfig {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"config": map[string]any{"deep_build_risk": classes},
		"phases": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "phase-registry.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(path)
	return cfg
}

func registryWithoutRiskClasses(t *testing.T) config.RoutingConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "phase-registry.json")
	if err := os.WriteFile(path, []byte(`{"config":{},"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(path)
	return cfg
}

func shippedRegistry(t *testing.T) config.RoutingConfig {
	t.Helper()
	cfg, _ := config.Load(config.RegistryPath(acsassert.RepoRoot(t)))
	return cfg
}

func workItem(acceptance string, files ...string) router.LaneItem {
	return router.LaneItem{ID: "lane-item", Kind: "feature", DeliverableKind: "code", Acceptance: []string{acceptance}, Files: files}
}

var highRiskWork = map[string]router.LaneItem{
	"protected surface": workItem("tighten the ship gate refusal", "go/internal/guards/ship.go"),
	"concurrency":       workItem("guard the shared counter with a mutex lock to fix a data race between goroutines", "go/internal/fleet/counter.go"),
	"security":          workItem("reject path traversal and redact the secret credential token before it is logged", "go/internal/redact/redact.go"),
	"kernel":            workItem("change the state machine transition table of the cycle", "go/internal/core/statemachine.go"),
}

var lowRiskWork = map[string]router.LaneItem{
	"docs only":    workItem("fix a typo in the onboarding guide", "docs/guides/onboarding.md"),
	"small helper": workItem("add a rune truncation option to the text helper", "go/internal/textcap/textcap.go"),
}

func TestC2243_001_ClassesComeFromPolicyData_NovelClassRoutesDeep(t *testing.T) {
	cfg := registryCarrying(t, map[string][]string{"zz-novel-class": {"qwertyuiop-novel-marker"}})
	item := workItem("rework the module", "go/zz/qwertyuiop-novel-marker.go")
	if got := buildTier(t, viaPlan, cfg, "balanced", "", item); got != "deep" {
		t.Errorf("a build matching a class that only the policy data names routed %q, want deep", got)
	}
}

func TestC2243_002_ClassesComeFromPolicyData_NoGoLiteralClassSurvives(t *testing.T) {
	cfg := registryCarrying(t, map[string][]string{"zz-only-class": {"zz-only-marker"}})
	for name, item := range highRiskWork {
		if got := buildTier(t, viaPlan, cfg, "deep", "", item); got == "deep" {
			t.Errorf("%s work routed deep although the policy names none of its markers: a Go literal decides the class", name)
		}
	}
}

func TestC2243_003_ShippedRegistryRoutesEachHighRiskClassDeep(t *testing.T) {
	cfg := shippedRegistry(t)
	for name, item := range highRiskWork {
		for _, proposed := range []string{"balanced", ""} {
			if got := buildTier(t, viaPlan, cfg, proposed, "", item); got != "deep" {
				t.Errorf("%s build proposed %q routed %q, want deep", name, proposed, got)
			}
		}
	}
}

func TestC2243_004_ShippedRegistryKeepsLowRiskBuildOffDeep(t *testing.T) {
	cfg := shippedRegistry(t)
	for name, item := range lowRiskWork {
		for _, proposed := range []string{"deep", "top", "", "balanced"} {
			if got := buildTier(t, viaPlan, cfg, proposed, "", item); got != "balanced" {
				t.Errorf("%s build proposed %q routed %q, want balanced", name, proposed, got)
			}
		}
	}
}

func TestC2243_005_ReplanAppliesTheSameRule(t *testing.T) {
	cfg := shippedRegistry(t)
	if got := buildTier(t, viaRePlan, cfg, "balanced", "", highRiskWork["security"]); got != "deep" {
		t.Errorf("replan of a high-risk build routed %q, want deep", got)
	}
	if got := buildTier(t, viaRePlan, cfg, "deep", "", lowRiskWork["docs only"]); got != "balanced" {
		t.Errorf("replan of a low-risk build routed %q, want balanced", got)
	}
}

func TestC2243_006_NoRuleOrNoEvidenceLeavesTheProposalUnchanged(t *testing.T) {
	absent := registryWithoutRiskClasses(t)
	for _, proposed := range []string{"deep", "balanced"} {
		for name, item := range lowRiskWork {
			if got := buildTier(t, viaPlan, absent, proposed, "", item); got != proposed {
				t.Errorf("no policy classes, %s build proposed %q routed %q, want it unchanged", name, proposed, got)
			}
		}
	}
	ruled := shippedRegistry(t)
	unresolved := router.LaneItem{ID: "lane-item", Unresolved: "inbox record not resolved"}
	if got := buildTier(t, viaPlan, ruled, "deep", ""); got != "deep" {
		t.Errorf("no goal and no lane item routed %q, want the proposal deep unchanged", got)
	}
	if got := buildTier(t, viaPlan, ruled, "deep", "", unresolved); got != "deep" {
		t.Errorf("only an unresolved lane item routed %q, want the proposal deep unchanged", got)
	}
}

func TestC2243_007_PlanPromptNamesPolicyClassesAndDropsTheBlanketBuildRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Config struct {
			Classes map[string][]string `json:"deep_build_risk"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Config.Classes) < 4 {
		t.Fatalf("shipped registry names %d deep_build_risk classes, want the plan's four or more", len(doc.Config.Classes))
	}
	const blanketRule = "build, tdd, audit"
	a := advisorProposing("balanced")
	prompt := a.ComposePlanPrompt(routeInput(t, shippedRegistry(t), "goal"), "phase-plan.json")
	for class := range doc.Config.Classes {
		if !strings.Contains(prompt, class) {
			t.Errorf("plan prompt does not name the policy class %q", class)
		}
	}
	if strings.Contains(prompt, blanketRule) {
		t.Errorf("plan prompt still tells the advisor that every build is a deep phase (%q)", blanketRule)
	}
	legacy := a.ComposePlanPrompt(routeInput(t, registryWithoutRiskClasses(t), "goal"), "phase-plan.json")
	if !strings.Contains(legacy, blanketRule) {
		t.Errorf("with no policy classes the plan prompt must keep its existing tier policy line (%q)", blanketRule)
	}
}

func TestC2243_008_ChangedFunctionsOfTheRuleAreFullyCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	targets := []struct{ pkg, file string }{
		{"./internal/core/advisor", "deep_build_risk.go"},
		{"./internal/config", "deep_build_risk.go"},
	}
	for _, target := range targets {
		path := filepath.Join(root, "go", target.pkg, target.file)
		if !acsassert.FileExists(t, path) {
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", path); code == 0 {
			t.Errorf("%s is gitignored and would be dropped at ship", path)
		}
		profile := filepath.Join(t.TempDir(), "cover.out")
		run := exec.CommandContext(context.Background(), "go", "test", "-count=1", "-coverprofile="+profile, target.pkg)
		run.Dir = filepath.Join(root, "go")
		if out, err := run.CombinedOutput(); err != nil {
			t.Errorf("go test %s failed: %v\n%s", target.pkg, err, acsassert.OutputTail(string(out), 20))
			continue
		}
		report := exec.CommandContext(context.Background(), "go", "tool", "cover", "-func="+profile)
		report.Dir = filepath.Join(root, "go")
		out, err := report.Output()
		if err != nil {
			t.Errorf("go tool cover -func failed for %s: %v", target.pkg, err)
			continue
		}
		functions := 0
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "/"+target.file+":") {
				continue
			}
			functions++
			if !strings.HasSuffix(strings.TrimSpace(line), "100.0%") {
				t.Errorf("changed function is not fully covered: %s", strings.TrimSpace(line))
			}
		}
		if functions == 0 {
			t.Errorf("%s declares no function that the coverage report lists", path)
		}
	}
}
