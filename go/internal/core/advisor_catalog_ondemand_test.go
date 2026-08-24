package core

// advisor_catalog_ondemand_test.go — the advisor's SELECT menu must be a menu,
// not an inventory.
//
// Measured 2026-08-23 on the runtime plane: 65 non-control phases are projected
// as advisor SELECT cards against maxEnrichedCatalogCards = 12, so 53 render in
// the degraded overflow form. Of those 65, 47 have NEVER been selected in 120
// cycles. The 12 enriched slots are therefore allocated by registry order and
// Optional-ness — not by usefulness — and genuinely useful rare phases lose
// their metadata to phases the advisor has never once chosen.
//
// This is the bloated-tool-set failure mode with a number on it. The fix is NOT
// to delete the unused phases: a phase like migration-safety-check is exactly
// what you want available for the one cycle that needs it, and deleting it
// because it has not fired is deleting the fire extinguisher because there has
// been no fire. The measured harm is advisor CONTEXT, not execution.
//
// So a phase may decline a SELECT slot while staying installed, dispatchable by
// explicit plan, and mintable. It is HIDDEN FROM THE MENU, NOT REMOVED — the
// declined set is still indexed in one line so neither the advisor nor an
// operator loses discoverability.

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

// catalogOf builds a Catalog the same way the rest of the core tests do.
func catalogOf(t *testing.T, specs ...phasespec.PhaseSpec) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge(specs)
	if err != nil {
		t.Fatalf("merge catalog: %v", err)
	}
	return cat
}

// An on-demand phase must not consume a SELECT card.
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

// NO-REGRESSION: an absent catalog key means today's behavior exactly.
func TestPhaseCardsFromCatalog_AbsentKeyIsUnchanged(t *testing.T) {
	cat := catalogOf(t, specWith("scout", "plan", ""), specWith("audit", "evaluate", ""))
	if got := len(phaseCardsFromCatalog(cat)); got != 2 {
		t.Fatalf("phases without the key must all render; got %d cards", got)
	}
}

// Control phases were already excluded; that must not change.
func TestPhaseCardsFromCatalog_ControlStillExcluded(t *testing.T) {
	cat := catalogOf(t, specWith("ship", "control", ""), specWith("scout", "plan", ""))
	cards := phaseCardsFromCatalog(cat)
	if len(cards) != 1 || cards[0].Name != "scout" {
		t.Fatalf("control phases stay excluded; got %+v", cards)
	}
}

// HIDDEN, NOT REMOVED. The declined set must still be named in the prompt, or
// this fix trades one invisibility problem for another — the advisor would have
// no way to learn the phase exists, and neither would an operator reading the
// prompt.
func TestWriteCatalog_OnDemandPhasesAreStillIndexed(t *testing.T) {
	var b strings.Builder
	writeCatalogWithOnDemand(&b, []router.PhaseCard{{Name: "scout", Optional: true}},
		[]string{"market-sizing", "okr-draft"})
	out := b.String()

	if !strings.Contains(out, "market-sizing") || !strings.Contains(out, "okr-draft") {
		t.Fatalf("declined phases must still be discoverable by name; got:\n%s", out)
	}
	if !strings.Contains(out, "scout") {
		t.Fatalf("the SELECT menu itself must still render; got:\n%s", out)
	}
	// One line, not 53 cards — the whole point.
	idx := out[strings.Index(out, "market-sizing"):]
	if n := strings.Count(idx, "\n"); n > 3 {
		t.Fatalf("the on-demand index must be compact, not a second catalog (%d lines):\n%s", n, idx)
	}
}

// No on-demand phases ⇒ no index line at all, so the common repo is unchanged.
func TestWriteCatalog_NoIndexLineWhenNothingDeclined(t *testing.T) {
	var b strings.Builder
	writeCatalogWithOnDemand(&b, []router.PhaseCard{{Name: "scout", Optional: true}}, nil)
	// Case matters: the emitted marker is "ON REQUEST". An earlier version of
	// this assertion searched for lowercase "on request" and therefore could
	// never fail — it passed against an implementation that always printed the
	// line. Match the real token.
	if strings.Contains(b.String(), "ON REQUEST") {
		t.Fatalf("no declined phases means no index line; got:\n%s", b.String())
	}
}

// THE WIRING TEST. Everything above proves the pieces; this proves the ADVISOR
// PROMPT changes. Three separate fixes this week shipped a correct component
// that nothing called, so the assertion is on the rendered prompt built from a
// real catalog — not on a helper invoked directly by the test.
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
	// The index must list ONLY what declined. An index that names every phase
	// re-creates the crowding it exists to remove, one line lower down.
	// Slice from the START of the index line: the count precedes the ON REQUEST
	// marker, so anchoring on the marker would cut it off.
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

// The repo catalog must not carry a typo'd membership word: an unrecognized
// value silently leaves the phase ON the menu, which is the failure this key
// exists to prevent. Same loud-failure rule the classifier applies elsewhere.
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

// M9's kill: through composePlanPrompt / buildPlanPrompt — the functions that
// actually render the advisor prompt in production. The earlier test called the
// catalog writer directly and so could not see a call site passing nil, which is
// exactly the mutation that survived.
func TestBuildPlanPrompt_CarriesTheOnDemandIndex(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
	)
	in := router.RouteInput{
		Catalog:        phaseCardsFromCatalog(cat),
		OnDemandPhases: onDemandCatalogNames(cat),
	}

	prompt := buildPlanPrompt(in)

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

// The typo guard must actually reject a bad word. The repo-catalog test walks
// real phase.json files, none of which carry one, so it never exercises this.
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

// The OTHER production render path. composePlanPrompt is the one the advisor
// actually uses when a persona is configured; buildPlanPrompt is its
// persona-less fallback. Both call the catalog writer, so both need the index —
// covering only one left a mutation alive that nulled the other, which is how
// this test came to exist.
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

// THE OUTERMOST WIRING. advisorPlanInput is where the orchestrator assembles the
// advisor's RouteInput from its own catalog. Every test above constructs that
// input by hand, so all of them pass even if the orchestrator populates nil —
// which is exactly the mutation that survived them. If this layer is dead the
// feature is dead, however well the renderers behave.
//
// Sixth wiring layer chased on this change: a test at layer N never proves
// layer N+1.
func TestAdvisorPlanInput_PopulatesOnDemandFromTheOrchestratorCatalog(t *testing.T) {
	cat := catalogOf(t,
		specWith("scout", "plan", ""),
		specWith("market-sizing", "plan", phasespec.CatalogOnDemand),
		specWith("okr-draft", "plan", phasespec.CatalogOnDemand),
	)
	// o.now() is dereferenced during assembly; supply it (and nothing else — the
	// recall lookup and cfg both no-op on zero values).
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

// A tracked phase with NO resolvable persona is undispatchable by construction
// — the runner's load-agent step fails before any work happens. It must not
// hold a SELECT slot: cycle-1551 (soak-20260824a) had the advisor insert
// defect-disposition-preflight, whose persona exists nowhere, and the load
// failure killed the whole lane rc=4. Four catalog phases carried the defect;
// two were on the menu. The fail-soft skip (optionalInfraSkip +
// ErrAgentDocMissing) contains the blast radius when one is dispatched anyway;
// this guard keeps them off the menu in the first place. The cure for a phase
// caught here: write agents/evolve-<agent>.md, add a phase-local agent.md, or
// mark it catalog:"on-demand" until someone does.
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
		// Control-role detection uses the PRODUCTION inference (RoleOrDefault:
		// explicit archetype, else the built-in name table) — never a private
		// allowlist that drifts from phasespec.inferredRoles.
		if (phasespec.PhaseSpec{Name: name, Role: cfg.Archetype}).RoleOrDefault() == phasespec.RoleControl ||
			cfg.Catalog == phasespec.CatalogOnDemand {
			continue
		}
		checked++
		agent := cfg.Agent
		if agent == "" {
			agent = "evolve-" + name
		}
		// The ONLY persona source the dispatch path reads for a disk-loaded
		// spec is agents/<agent>.md (prompts.Loader.Agent is single-rooted;
		// specrunner's inline PromptBody is mint-only). A phase-local agent.md
		// is a scaffold for humans and must NOT count — accepting it would
		// bless exactly the phase shape `evolve phases add` produces while the
		// runner still dies at load-agent.
		if _, err := os.Stat(filepath.Join(root, "agents", agent+".md")); err != nil {
			t.Errorf("menu phase %q resolves NO persona (agents/%s.md absent) — dispatching it kills a lane at load-agent (cycle-1551). Write agents/%s.md or mark the phase catalog:\"on-demand\".", name, agent, agent)
		}
	}
	if checked == 0 {
		t.Skip("no menu phases found")
	}
}
