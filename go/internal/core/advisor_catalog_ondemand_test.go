package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func specWith(name, archetype, catalog string) phasespec.PhaseSpec {
	return phasespec.PhaseSpec{Name: name, Role: archetype, Catalog: catalog, Optional: true,
		Description: name + " does a thing", WhenToUse: "when " + name + " is needed"}
}

func catalogOf(t *testing.T, specs ...phasespec.PhaseSpec) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge(specs)
	if err != nil {
		t.Fatalf("merge catalog: %v", err)
	}
	return cat
}

func TestPhaseCardsFromCatalog_OnDemandPhaseTakesNoSelectSlot(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
		specWith("audit", "evaluate", ""),
	)
	cards := phaseCardsFromCatalog(cat)

	var names []string
	for _, c := range cards {
		names = append(names, c.Name)
	}
	if len(cards) != 2 {
		t.Fatalf("an on-demand phase must not appear as a SELECT card; got %v", names)
	}
	for _, c := range cards {
		if c.Name == "market-sizing" {
			t.Fatalf("market-sizing declined its slot and must be absent; got %v", names)
		}
	}
}

func TestPhaseCardsFromCatalog_AbsentKeyIsUnchanged(t *testing.T) {
	cat := catalogOf(t, specWith("scout", "plan", ""), specWith("audit", "evaluate", ""))
	if got := len(phaseCardsFromCatalog(cat)); got != 2 {
		t.Fatalf("phases without the key must all render; got %d cards", got)
	}
}

func TestPhaseCardsFromCatalog_ControlStillExcluded(t *testing.T) {
	cat := catalogOf(t, specWith("ship", "control", ""), specWith("scout", "plan", ""))
	cards := phaseCardsFromCatalog(cat)
	if len(cards) != 1 || cards[0].Name != "scout" {
		t.Fatalf("control phases stay excluded; got %+v", cards)
	}
}

func TestAdvisorPrompt_OnDemandPhaseLeavesTheMenuButStaysNamed(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("audit", "evaluate", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
	)

	in := router.RouteInput{
		Catalog:        phaseCardsFromCatalog(cat),
		OnDemandPhases: onDemandCatalogNames(cat),
	}
	var b strings.Builder
	writeCatalogWithOnDemand(&b, in.Catalog, in.OnDemandPhases)
	prompt := b.String()

	menu := prompt
	if i := strings.Index(prompt, "ON REQUEST"); i >= 0 {
		menu = prompt[:i]
	}
	if strings.Contains(menu, "market-sizing") {
		t.Fatalf("a declined phase must not appear in the SELECT menu:\n%s", menu)
	}
	if !strings.Contains(prompt, "market-sizing") {
		t.Fatalf("a declined phase must still be NAMED so it stays discoverable:\n%s", prompt)
	}
	if !strings.Contains(menu, "scout") || !strings.Contains(menu, "audit") {
		t.Fatalf("the real menu must still render its cards:\n%s", menu)
	}
	// Slice from the start of the index line: the count precedes the ON REQUEST marker.
	idx := prompt[strings.Index(prompt, "further phase"):]
	for _, onMenu := range []string{"scout", "audit"} {
		if strings.Contains(idx, onMenu) {
			t.Fatalf("%q is on the menu and must not also appear in the on-request index:\n%s", onMenu, idx)
		}
	}
	if !strings.Contains(prompt, "1 further phase") {
		t.Fatalf("the index must state how many declined; got:\n%s", prompt)
	}
}

func TestRepoPhaseCatalog_CatalogWordIsKnown(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	dir := filepath.Join(root, ".evolve", "phases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("phase catalog not present: %v", err)
	}
	tracked := trackedPhaseDirsForTest(t, root)
	checked, onDemand := 0, 0
	for _, e := range entries {
		if !e.IsDir() || (tracked != nil && !tracked[e.Name()]) {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name(), "phase.json"))
		if rerr != nil {
			continue
		}
		var cfg struct {
			Catalog string `json:"catalog"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			continue
		}
		checked++
		if !phasespec.KnownCatalogWord(cfg.Catalog) {
			t.Errorf(".evolve/phases/%s/phase.json declares catalog=%q — unknown, so the phase silently stays on the advisor menu. Use \"\" or %q.",
				e.Name(), cfg.Catalog, phasespec.CatalogOnDemand)
		}
		if cfg.Catalog == phasespec.CatalogOnDemand {
			onDemand++
		}
	}
	if checked == 0 {
		t.Skip("no phase.json files found")
	}
	t.Logf("tracked phases: %d, declined a SELECT slot: %d, on the menu: %d", checked, onDemand, checked-onDemand)
}

func TestBuildPlanPrompt_CarriesTheOnDemandIndex(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
	)
	in := router.RouteInput{
		Catalog:        phaseCardsFromCatalog(cat),
		OnDemandPhases: onDemandCatalogNames(cat),
	}

	prompt := NewPhaseAdvisor(nil).composePlanPrompt(in, "routing-plan.json")

	if !strings.Contains(prompt, "ON REQUEST") {
		t.Fatalf("the production plan prompt must carry the on-request index:\n%s", prompt)
	}
	if !strings.Contains(prompt, "market-sizing") {
		t.Fatalf("the declined phase must be named in the production prompt:\n%s", prompt)
	}
	menu := prompt[:strings.Index(prompt, "ON REQUEST")]
	if strings.Contains(menu, "market-sizing") {
		t.Fatalf("the declined phase must not be a SELECT card in the production prompt:\n%s", menu)
	}
}

func TestKnownCatalogWord_RejectsTypos(t *testing.T) {
	for _, ok := range []string{phasespec.CatalogSelect, phasespec.CatalogOnDemand} {
		if !phasespec.KnownCatalogWord(ok) {
			t.Fatalf("%q must be accepted", ok)
		}
	}
	for _, bad := range []string{"ondemand", "on demand", "On-Demand", "select", "off", "hidden"} {
		if phasespec.KnownCatalogWord(bad) {
			t.Fatalf("%q must be rejected — an unknown word silently leaves the phase on the menu", bad)
		}
	}
}

func TestComposePlanPrompt_CarriesTheOnDemandIndex(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
	)
	in := router.RouteInput{
		Catalog:        phaseCardsFromCatalog(cat),
		OnDemandPhases: onDemandCatalogNames(cat),
	}
	p := NewPhaseAdvisor(nil, WithPersona("# Persona\nyou plan cycles."))

	prompt := p.composePlanPrompt(in, "routing-plan.json")

	if !strings.Contains(prompt, "1 further phase") || !strings.Contains(prompt, "market-sizing") {
		t.Fatalf("the persona render path must carry the on-request index:\n%s", prompt)
	}
	menu := prompt[:strings.Index(prompt, "further phase")]
	if strings.Contains(menu, "market-sizing") {
		t.Fatalf("the declined phase must not be a SELECT card here either:\n%s", menu)
	}
}

func TestAdvisorPlanInput_PopulatesOnDemandFromTheOrchestratorCatalog(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
		specWith("okr-draft", "plan", phasespec.CatalogOnDemand),
	)
	// o.now is dereferenced during assembly; the recall lookup and cfg no-op on zero values.
	o := &Orchestrator{catalog: cat, now: func() time.Time { return time.Unix(0, 0).UTC() }}

	in := o.advisorPlanInput(context.Background(), "start", router.RoutingSignals{},
		CycleRequest{ProjectRoot: "/p"}, State{}, CycleState{}, 1, nil, nil)

	if len(in.OnDemandPhases) != 2 {
		t.Fatalf("the orchestrator must project its catalog's declined phases; got %v", in.OnDemandPhases)
	}
	got := strings.Join(in.OnDemandPhases, ",")
	for _, want := range []string{"market-sizing", "okr-draft"} {
		if !strings.Contains(got, want) {
			t.Fatalf("declined phase %q missing from the assembled input; got %v", want, in.OnDemandPhases)
		}
	}
	for _, c := range in.Catalog {
		if c.Name == "market-sizing" || c.Name == "okr-draft" {
			t.Fatalf("a declined phase must not also be a SELECT card: %+v", in.Catalog)
		}
	}
}

func TestRepoPhaseCatalog_MenuPhasesResolveAPersona(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	dir := filepath.Join(root, ".evolve", "phases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("phase catalog not present: %v", err)
	}
	tracked := trackedPhaseDirsForTest(t, root)
	checked := 0
	for _, e := range entries {
		if !e.IsDir() || (tracked != nil && !tracked[e.Name()]) {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name(), "phase.json"))
		if rerr != nil {
			continue
		}
		var cfg struct {
			Name      string `json:"name"`
			Agent     string `json:"agent"`
			Archetype string `json:"archetype"`
			Catalog   string `json:"catalog"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			continue
		}
		name := cfg.Name
		if name == "" {
			name = e.Name()
		}
		// Control-role detection uses the production inference, never a private allowlist.
		if (phasespec.PhaseSpec{Name: name, Role: cfg.Archetype}).RoleOrDefault() == phasespec.RoleControl ||
			cfg.Catalog == phasespec.CatalogOnDemand {
			continue
		}
		checked++
		agent := cfg.Agent
		if agent == "" {
			agent = "evolve-" + name
		}
		// Dispatch reads a disk-loaded spec's persona only from agents/<agent>.md; a phase-local agent.md must not count.
		if _, err := os.Stat(filepath.Join(root, "agents", agent+".md")); err != nil {
			t.Errorf("menu phase %q resolves NO persona (agents/%s.md absent) — dispatching it kills a lane at load-agent (cycle-1551). Write agents/%s.md or mark the phase catalog:\"on-demand\".", name, agent, agent)
		}
	}
	if checked == 0 {
		t.Skip("no menu phases found")
	}
}
