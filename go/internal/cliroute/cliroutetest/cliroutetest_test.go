package cliroutetest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func variantNamed(t *testing.T, name string) Variant {
	t.Helper()
	for _, v := range Variants() {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("no variant %s", name)
	return Variant{}
}

func TestVariants_AreUniqueAndEachHasAHost(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range Variants() {
		if seen[v.Name] {
			t.Fatalf("duplicate variant %s", v.Name)
		}
		seen[v.Name] = true
		if len(v.Installed) == 0 || len(v.Discovered) == 0 {
			t.Errorf("variant %s has no host binaries or discovery", v.Name)
		}
	}
	for _, want := range []string{"none", "env_cli", "env_agent", "pin", "advisor", "benched", "missing_binary", "bypass"} {
		if !seen[want] {
			t.Errorf("the golden must cover variant %s", want)
		}
	}
}

func TestVariant_EnvForAddsThePerAgentKeyWithoutTouchingTheVariant(t *testing.T) {
	v := variantNamed(t, "env_agent")
	env := v.EnvFor("tdd-engineer")
	if env["EVOLVE_TDD_ENGINEER_CLI"] != "agy-tmux" {
		t.Fatalf("EnvFor = %v", env)
	}
	cli := variantNamed(t, "env_cli")
	env = cli.EnvFor("builder")
	env["EVOLVE_CLI"] = "changed"
	if cli.Env["EVOLVE_CLI"] != "claude-p" {
		t.Fatal("EnvFor must return a copy")
	}
}

func TestVariant_LookPathAndPathDirFollowTheInstalledSet(t *testing.T) {
	v := variantNamed(t, "missing_binary")
	if _, err := v.LookPath("codex"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("codex is not installed in missing_binary: %v", err)
	}
	if path, err := v.LookPath("claude"); err != nil || filepath.Base(path) != "claude" {
		t.Fatalf("claude is installed: %q %v", path, err)
	}
	dir := v.PathDir(t)
	for _, bin := range v.Installed {
		info, err := os.Stat(filepath.Join(dir, bin))
		if err != nil || info.Mode()&0o111 == 0 {
			t.Errorf("stub %s: %v (mode %v)", bin, err, info)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "codex")); err == nil {
		t.Error("a missing binary must have no stub")
	}
}

func TestVariant_ProductionDiscoverAppliesTheFamilyBanAndTheSwitch(t *testing.T) {
	v := variantNamed(t, "none")
	if got := v.ProductionDiscover(policy.Policy{})(); slices.Contains(got, "agy-tmux") {
		t.Fatalf("the compiled default bans agy from the tail: %v", got)
	}
	admit := policy.Policy{Workflow: &policy.WorkflowPolicy{UniversalFallbackExclude: []string{}}}
	if got := v.ProductionDiscover(admit)(); !slices.Contains(got, "agy-tmux") {
		t.Fatalf("an empty ban admits agy: %v", got)
	}
	off := false
	if fn := v.ProductionDiscover(policy.Policy{Workflow: &policy.WorkflowPolicy{UniversalFallback: &off}}); fn != nil {
		t.Fatal("the switch off means no discovery, as the composition root wires it")
	}
	if !reflect.DeepEqual(v.RawDiscover(), v.Discovered) {
		t.Fatal("RawDiscover is the unfiltered list")
	}
}

func TestVariant_ProjectRootWritesThePolicyAndTheBench(t *testing.T) {
	pinned := variantNamed(t, "pin")
	root := pinned.ProjectRoot(t, []string{"build", "audit"})
	if pin, ok := pinned.Policy(t, root).PinFor("audit"); !ok || pin != *pinned.Pin {
		t.Fatalf("the pin variant pins every phase it is given: %+v %v", pin, ok)
	}
	bypass := variantNamed(t, "bypass")
	if p := bypass.Policy(t, bypass.ProjectRoot(t, []string{"audit"})); !bypass.Bypass || len(p.Pins) == 0 || p.Workflow == nil || p.Workflow.UniversalFallbackExclude == nil {
		t.Fatalf("bypass writes pins and a workflow so the runner can show it ignores the one and keeps the other: %+v", p)
	}
	tail := variantNamed(t, "checked_in_tail")
	if got := tail.Policy(t, tail.ProjectRoot(t, nil)).WorkflowConfig().UniversalFallbackExclude; len(got) != 0 {
		t.Fatalf("checked_in_tail lifts the family ban: %v", got)
	}
	benched := variantNamed(t, "benched")
	active := clihealth.NewStore(benched.ProjectRoot(t, nil), Now).Active()
	if _, ok := active["codex"]; !ok {
		t.Fatalf("the benched variant benches codex at the fixed clock: %v", active)
	}
	none := variantNamed(t, "none")
	if p := none.Policy(t, none.ProjectRoot(t, nil)); p.Pins != nil || p.Workflow != nil {
		t.Fatalf("the none variant writes no policy: %+v", p)
	}
}

func TestNow_IsAFixedInstant(t *testing.T) {
	if !Now().Equal(time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)) || !Now().Equal(Now()) {
		t.Fatalf("Now() = %v", Now())
	}
}

func TestPhaseOf_MapsBuiltinAgentsToTheirPhase(t *testing.T) {
	want := map[string]string{
		"builder": "build", "auditor": "audit", "tdd-engineer": "tdd", "retrospective": "retro",
		"router": "router", "scout": "scout", "accessibility-audit": "accessibility-audit",
	}
	for agent, phase := range want {
		if got := PhaseOf(agent); got != phase {
			t.Errorf("PhaseOf(%s) = %s, want %s", agent, got, phase)
		}
	}
	if got := PhasesOf([]string{"builder", "memo"}); !reflect.DeepEqual(got, []string{"build", "memo"}) {
		t.Fatalf("PhasesOf = %v", got)
	}
}

func TestTrackedProfiles_BindsTheTrackedCorpus(t *testing.T) {
	if _, err := os.Stat(filepath.Join(RepoRoot(t), "go", "go.mod")); err != nil {
		t.Fatalf("RepoRoot is not the repository root: %v", err)
	}
	dir, names := TrackedProfiles(t)
	if len(names) < 50 || !slices.Contains(names, "builder") || filepath.Base(dir) != "profiles" {
		t.Fatalf("tracked profiles under %s: %d names", dir, len(names))
	}
}

func TestRecords_CarryThePlanOrTheError(t *testing.T) {
	plan := llmroute.Plan{
		Candidates: []string{"agy-tmux"}, Triggers: []int{85}, PrimarySource: "default",
		Model: DefaultModel, Tiers: []string{"balanced"}, TierCeiling: map[string][]string{"deep": {"claude"}},
	}
	r := PlanRecord(ResolverRunner, "none", "builder", "build", plan)
	if r.Resolver != "runner" || !reflect.DeepEqual(r.TierCeiling, plan.TierCeiling) || r.Model != "balanced" {
		t.Fatalf("PlanRecord = %+v", r)
	}
	e := ErrorRecord(ResolverBridgechain, "pin", "auditor", "", "refused")
	if e.Resolver != "bridgechain" || e.Error != "refused" || e.Candidates != nil {
		t.Fatalf("ErrorRecord = %+v", e)
	}
	raw, err := Encode([]Record{r, e})
	if err != nil || strings.Count(string(raw), "\n") != 2 || !strings.Contains(string(raw), `"tier_ceiling":{"deep":["claude"]}`) {
		t.Fatalf("Encode = %s, %v", raw, err)
	}
	if !strings.Contains(ErrDeclineAuto.Error(), "declined") {
		t.Fatalf("ErrDeclineAuto = %v", ErrDeclineAuto)
	}
}

func TestGolden_ReadAndEncodeRoundTripByteForByte(t *testing.T) {
	records := ReadGolden(t)
	if len(records) == 0 || filepath.Base(GoldenPath(t)) != "legacy-plans.golden.jsonl" {
		t.Fatalf("golden at %s has %d records", GoldenPath(t), len(records))
	}
	AssertGolden(t, records, false)
}

func TestFirstDifference_NamesTheFirstDifferingLine(t *testing.T) {
	if lines, differ := firstDifference([]byte("a\nb\n"), []byte("a\nc\n")); !differ || lines != [2]string{"b", "c"} {
		t.Fatalf("got %v %v", lines, differ)
	}
	if lines, differ := firstDifference([]byte("a\n"), []byte("a\nextra\n")); !differ || lines[0] != "" || lines[1] != "extra" {
		t.Fatalf("a shorter record set differs at its end: %v %v", lines, differ)
	}
	if lines, differ := firstDifference([]byte("a"), []byte("a\nb")); !differ || lines[0] != "<missing>" {
		t.Fatalf("a missing line is named: %v %v", lines, differ)
	}
	if _, differ := firstDifference([]byte("a\n"), []byte("a\n")); differ {
		t.Fatal("equal bytes do not differ")
	}
}

func TestAssertGoldenAt_WritesAndThenMatchesTheGivenPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "advisor.jsonl")
	healthy := false
	records := []Record{{Resolver: "advisor", Variant: "none/claude_benched", Agent: "router", Phase: "plan", Candidates: []string{"agy-tmux"}, Model: "deep", Healthy: &healthy}}
	AssertGoldenAt(t, path, records, true)
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), `"healthy":false`) {
		t.Fatalf("the written record keeps an explicit false: %s %v", raw, err)
	}
	AssertGoldenAt(t, path, records, false)
}

func TestAdvisorGoldenPath_SitsBesideThePlanGolden(t *testing.T) {
	if filepath.Dir(AdvisorGoldenPath(t)) != filepath.Dir(GoldenPath(t)) || filepath.Base(AdvisorGoldenPath(t)) != "legacy-advisor.golden.jsonl" {
		t.Fatalf("advisor golden at %s", AdvisorGoldenPath(t))
	}
	if _, err := os.Stat(AdvisorGoldenPath(t)); err != nil {
		t.Fatalf("the advisor golden is checked in: %v", err)
	}
}

func TestVariants_TheRouterKeysVariantWritesTheRouterBlock(t *testing.T) {
	v := variantNamed(t, "router_keys")
	rc := v.Policy(t, v.ProjectRoot(t, nil)).RouterConfig()
	if rc.CLI != "claude-tmux" || rc.Model != "balanced" || rc.PlanModel != "top" {
		t.Fatalf("router_keys policy router block = %+v", rc)
	}
}
