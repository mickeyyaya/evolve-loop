package trustkernel_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func repoRootOrSkip(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not inside a git checkout: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func writeGoPredFixture(t *testing.T, root string, cycle int, pass bool) {
	t.Helper()
	pkg := "cycle" + strconv.Itoa(cycle)
	dir := filepath.Join(root, "go", "acs", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module acsfixture\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "//go:build acs\n\npackage " + pkg + "\n\nimport \"testing\"\n\nfunc TestC" + strconv.Itoa(cycle) + "_001_Fixture(t *testing.T) {\n"
	if !pass {
		body += "\tt.Fatal(\"fixture RED\")\n"
	}
	body += "}\n"
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShipGate_ShipEligibleOnlyWhenRedCountZero(t *testing.T) {
	root := t.TempDir()
	writeGoPredFixture(t, root, 1, true)

	v, err := acssuite.Run(acssuite.Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatalf("acssuite.Run: %v", err)
	}
	if v.RedCount != 0 {
		t.Fatalf("all-green suite: RedCount = %d, want 0", v.RedCount)
	}
	if v.Verdict != "PASS" || !v.ShipEligible {
		t.Errorf("all-green suite: Verdict=%q ShipEligible=%v, want PASS/true", v.Verdict, v.ShipEligible)
	}
}

func TestShipGate_BlocksWhenRedCountNonZero(t *testing.T) {
	root := t.TempDir()
	writeGoPredFixture(t, root, 1, false)

	v, err := acssuite.Run(acssuite.Options{Root: root, Cycle: 1})
	if err != nil {
		t.Fatalf("acssuite.Run: %v", err)
	}
	if v.RedCount == 0 {
		t.Fatal("suite with a failing predicate: RedCount = 0, want > 0")
	}
	if v.Verdict != "FAIL" {
		t.Errorf("Verdict = %q, want FAIL when a predicate is RED", v.Verdict)
	}
	if v.ShipEligible {
		t.Error("ShipEligible = true with a RED predicate; ship must be blocked")
	}
}

func tddPinCfg() config.RoutingConfig {
	return config.RoutingConfig{Conditional: map[string]config.CondRule{
		"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"},
	}}
}

func planRunsPhase(p *router.PhasePlan, phase string) bool {
	for _, e := range p.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

func TestRoutingFloor_ShipRequiresBuildAndAudit(t *testing.T) {
	in := router.RouteInput{
		Cfg:     tddPinCfg(),
		Signals: router.RoutingSignals{Scout: router.ScoutSignals{CycleSizeEstimate: "medium", Present: true}},
	}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "ship", Run: true},
	}}
	out, clamps := router.ClampPlanToFloor(in, plan)
	for _, required := range []string{"tdd", "build", "audit"} {
		if !planRunsPhase(out, required) {
			t.Errorf("ship plan must force %s to run; got plan %+v", required, out.Entries)
		}
	}
	if len(clamps) == 0 {
		t.Error("expected clamps recording the forced phases, got none")
	}
}

func TestRoutingFloor_NoShipCycleIsUnconstrained(t *testing.T) {
	in := router.RouteInput{
		Cfg:     tddPinCfg(),
		Signals: router.RoutingSignals{Scout: router.ScoutSignals{CycleSizeEstimate: "medium", Present: true}},
	}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "build", Run: false},
		{Phase: "ship", Run: false},
	}}
	_, clamps := router.ClampPlanToFloor(in, plan)
	if len(clamps) != 0 {
		t.Errorf("no-ship cycle must not be clamped, got %+v", clamps)
	}
}

func TestRoutingFloor_TrivialCycleExemptsTDDNotBuildAudit(t *testing.T) {
	in := router.RouteInput{
		Cfg:     tddPinCfg(),
		Signals: router.RoutingSignals{Scout: router.ScoutSignals{CycleSizeEstimate: "trivial", Present: true}},
	}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "ship", Run: true},
	}}
	out, _ := router.ClampPlanToFloor(in, plan)
	if planRunsPhase(out, "tdd") {
		t.Error("trivial cycle: tdd must NOT be forced")
	}
	if !planRunsPhase(out, "build") || !planRunsPhase(out, "audit") {
		t.Error("trivial cycle: build+audit must still be forced for a ship")
	}
}

func TestStateMachine_ShipFollowsAuditOnlyViaShippableVerdict(t *testing.T) {
	sm := core.NewStateMachine()
	if !sm.CanTransition(core.PhaseAudit, core.PhaseShip) {
		t.Error("audit → ship must be a legal transition")
	}
	if sm.CanTransition(core.PhaseScout, core.PhaseShip) {
		t.Error("scout → ship must be illegal (no bypass of build/audit)")
	}
	if sm.CanTransition(core.PhaseBuild, core.PhaseShip) {
		t.Error("build → ship must be illegal (audit must run first)")
	}
}

func TestStateMachine_AuditVerdictRoutesShipOrRetro(t *testing.T) {
	sm := core.NewStateMachine()
	for _, v := range []string{core.VerdictPASS, core.VerdictWARN} {
		next, err := sm.Next(core.PhaseAudit, v)
		if err != nil {
			t.Fatalf("audit verdict %q: %v", v, err)
		}
		if next != core.PhaseShip {
			t.Errorf("audit verdict %q → %q, want ship", v, next)
		}
	}
	next, err := sm.Next(core.PhaseAudit, core.VerdictFAIL)
	if err != nil {
		t.Fatalf("audit FAIL: %v", err)
	}
	if next != core.PhaseRetro {
		t.Errorf("audit FAIL → %q, want retro", next)
	}
}

func TestProfile_AllPhaseProfilesValid(t *testing.T) {
	profilesDir := filepath.Join(repoRootOrSkip(t), ".evolve", "profiles")
	entries, err := filepath.Glob(filepath.Join(profilesDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("no profiles found under %s", profilesDir)
	}
	for _, path := range entries {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: read: %v", filepath.Base(path), err)
			continue
		}
		var p struct {
			Name string `json:"name"`
			CLI  string `json:"cli"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Errorf("%s: invalid JSON: %v", filepath.Base(path), err)
			continue
		}
		isPolicyConfigNotProfile := p.Name == ""
		if isPolicyConfigNotProfile {
			continue
		}
		if p.CLI == "" {
			t.Errorf("%s: missing required \"cli\" field", filepath.Base(path))
		}
	}
}
