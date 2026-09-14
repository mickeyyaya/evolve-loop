package advisor

// prompt_test.go — the prompt composers and the routing context (ADR-0103
// unit 04 §6 tests 29, 33-35; the core rubric/failure/deliverable-kind/
// recall/clihealth/persona/absolute-path/mint-documentation tests moved
// verbatim in intent).

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Test 29 — the recon gather fails open: a reader fault is one
// ADVISOR_RECON_GIT_FAILED and the digest renders without file facts; the
// reader is never called with the recon off or an empty root; the Null
// Object default renders exactly the git-error path.
func TestComposePlanPrompt_ReconGitFaultWarnsOnceAndTheDigestDegrades(t *testing.T) {
	in := baseRouteInput()
	in.GoalText = "fix a security bug in the auth flow"
	in.CarryoverTodos = []router.CarryoverTodo{{ID: "t1", Action: "follow up"}}
	in.Cfg.ReconDigest = true
	calls := 0
	failing := WithRecentFiles(func(root string) ([]string, error) { calls++; return nil, errors.New("git: boom") })
	a, got := observed(t, nil, Identity{Persona: "PERSONA"}, failing)
	on := a.ComposePlanPrompt(in, "routing-plan.json")
	if !strings.Contains(on, "## Pre-plan recon (deterministic)") || !strings.Contains(on, "goal_keyword_hits: bug, fix, security") || !strings.Contains(on, "carryover_count: 1") || strings.Contains(on, "langs_touched") {
		t.Errorf("the digest degrades to the goal/carryover facts with no file facts:\n%s", on)
	}
	e := assertOneEvent(t, *got, CodeReconGitFailed, map[string]string{"step": "compose", "project_root": "/proj", "decision": "plan", "contract": "router"})
	if e.Reason != "git: boom" || e.Origin != "Advisor.Plan" || calls != 1 {
		t.Errorf("reason/origin/calls: %+v %d", e, calls)
	}
	*got = nil
	if re := a.ComposePlanPrompt(in, "routing-replan.json"); !strings.Contains(re, "Pre-plan recon") || (*got)[0].Fields["decision"] != "replan" || (*got)[0].Origin != "Advisor.RePlan" {
		t.Errorf("the re-plan artifact stamps the re-plan decision: %+v", *got)
	}
	*got, calls = nil, 0
	in.Cfg.ReconDigest = false
	if off := a.ComposePlanPrompt(in, "routing-plan.json"); strings.Contains(off, "Pre-plan recon") || calls != 0 || len(*got) != 0 {
		t.Errorf("recon off ⇒ no gather, no render, no event (%d calls, %+v)", calls, *got)
	}
	in.Cfg.ReconDigest = true
	in.ProjectRoot = ""
	if p := a.ComposePlanPrompt(in, "routing-plan.json"); calls != 0 || len(*got) != 0 || !strings.Contains(p, "Pre-plan recon") {
		t.Errorf("an empty project root reads nothing and is no fault (%d calls, %+v)", calls, *got)
	}
	in.ProjectRoot = "/proj"
	quiet := New(nil, Identity{Persona: "PERSONA"}, nil)
	if want, got := a.ComposePlanPrompt(in, "routing-plan.json"), quiet.ComposePlanPrompt(in, "routing-plan.json"); want != got {
		t.Error("the Null-Object reader renders exactly the git-error path")
	}
	// The prompt (and its recon) is composed BEFORE the launch guards run — the
	// pre-extraction argument order — so a nil launcher still reports the recon
	// fault first, then the preflight refusal.
	*got = nil
	if _, err := a.Plan(in); err == nil || len(*got) != 2 || (*got)[0].Code != CodeReconGitFailed || (*got)[1].Code != CodeLaunchFailed {
		t.Errorf("compose → preflight: %v %+v", err, *got)
	}
	// The PRODUCTION re-plan composes from its decision, never through the
	// artifact-string map the facade uses: its recon fault carries the re-plan
	// stamp (review fold — killed by decisionForArtifact ⇒ always Plan).
	*got = nil
	if _, err := a.RePlan(in); err == nil || len(*got) != 2 || (*got)[0].Code != CodeReconGitFailed || (*got)[0].Origin != "Advisor.RePlan" || (*got)[0].Fields["decision"] != "replan" {
		t.Errorf("RePlan's recon fault carries the re-plan stamp: %v %+v", err, *got)
	}
}

// Test 33 — the nine context sections render in order (ending with the
// FORBIDDEN line), equal the routing golden's context, and vanish when their
// input is empty; benches are family-sorted and a quota wall is WALLED.
func TestWriteRoutingContext_SectionsRenderInOrderAndVanishWhenEmpty(t *testing.T) {
	var b strings.Builder
	WriteRoutingContext(&b, richRouteInput())
	out := b.String()
	last := -1
	for _, h := range []string{"## Cycle\n", "## Goal\n", "## CLI health (environmental)\n", "## Objective signals (digested from handoff artifacts)\n",
		"## Carryover todos from previous cycles", "## Recall memory (learn from prior cycles", "## Optional phases available",
		"## Unavailable phases (persona doc missing", "## Decision rubric (justify each optional phase", "FORBIDDEN: never propose reaching ship without audit"} {
		i := strings.Index(out, h)
		if i <= last {
			t.Errorf("section %q at %d, must follow %d", h, i, last)
		}
		last = i
	}
	if golden := readGolden(t, "prompt-routing-happy.golden.txt"); !strings.Contains(golden, out) {
		t.Error("the context is the golden's context section verbatim")
	}
	if !strings.Contains(out, "- agy: WALLED/unavailable (quota_exhausted) until 01:00Z") || !strings.Contains(out, "- codex: benched (rate_limit) until 06:13Z") ||
		strings.Index(out, "- agy:") > strings.Index(out, "- codex:") {
		t.Errorf("benches: family-sorted, walled iff the reason mentions exhaustion, UTC clock:\n%s", out)
	}
	// Optional = trigger phases minus the unavailable ones, sorted; unavailable listed as given.
	if !strings.Contains(out, "- architecture-design\n- plan-review\n- tester\n") || strings.Contains(out, "- security-sweep\n- tester") {
		t.Errorf("optional phases sorted, persona-less ones removed:\n%s", out)
	}
	if !strings.Contains(out, "NOT selectable this cycle)\n- security-sweep\n- ghost-phase\n") {
		t.Errorf("unavailable phases named:\n%s", out)
	}
	var empty strings.Builder
	WriteRoutingContext(&empty, router.RouteInput{GoalText: "   "})
	e := empty.String()
	for _, absent := range []string{"## Goal", "CLI health", "Carryover todos", "Recall memory", "Optional phases", "Unavailable phases"} {
		if strings.Contains(e, absent) {
			t.Errorf("%q must vanish on an empty input:\n%s", absent, e)
		}
	}
	if !strings.HasPrefix(e, "## Cycle\n- cycle: 0\n") || !strings.Contains(e, "## Objective signals") || !strings.HasSuffix(e, "rejected by the kernel.\n") {
		t.Errorf("the header, the signals header and the rubric always render:\n%s", e)
	}
	var one strings.Builder
	WriteRoutingContext(&one, router.RouteInput{BenchedCLIs: []router.BenchedCLI{{Family: "codex", Reason: "rate_limit", Until: time.Date(2026, 6, 11, 6, 13, 0, 0, time.UTC)}}})
	if !strings.Contains(one.String(), "- codex: benched (rate_limit) until 06:13Z — its dispatch chains start at the fallback CLI") {
		t.Errorf("one bench:\n%s", one.String())
	}
}

// Test 34 — the rubric projections, the op tables, the signal lines, the
// recall section, the failure-transition alias and the failure vocabulary.
func TestWriteRubricLines_ProjectsTheRoutingConfig(t *testing.T) {
	in := router.RouteInput{Cfg: config.RoutingConfig{
		Triggers: map[string]config.RoutingBlock{
			"tester": {InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}, {Field: "build.severity_max", Op: "gte", Value: "HIGH"}}},
			"scout":  {RubricHint: []string{"scout.item_count == 0 → end cycle early (no-ship is legitimate)"}},
		},
		Conditional: map[string]config.CondRule{
			"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial", And: []config.CondRule{{Field: "deliverable_kind", Op: "!=", Value: "document"}}},
			"x":   {Field: "f", Op: "matches", Value: "v"},
		},
	}}
	got := buildRoutingPrompt(in)
	for _, want := range []string{
		"- build.acs_red > 0 OR build.severity_max >= HIGH → insert tester",
		"- cycle_size == trivial → skip tdd (conditional-mandatory exemption)",
		"- deliverable_kind == document → skip tdd (conditional-mandatory exemption)",
		"- scout.item_count == 0 → end cycle early (no-ship is legitimate)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rubric missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "skip x") {
		t.Error("an unknown op renders no exemption line rather than a wrong one")
	}
	if si, ti, xi := strings.Index(got, "→ end cycle early"), strings.Index(got, "→ skip tdd"), strings.Index(got, "→ insert tester"); si > ti || ti > xi {
		t.Error("phases sort: scout, tdd, tester")
	}
	if e := buildRoutingPrompt(router.RouteInput{}); strings.Contains(e, "→ skip") || strings.Contains(e, "→ insert") || !strings.Contains(e, "FORBIDDEN: never propose reaching ship without audit") {
		t.Errorf("a source-less cfg renders only the kernel rule:\n%s", e)
	}
}

func TestOpSymbolAndNegateOp_Tables(t *testing.T) {
	symbol := map[string]string{"eq": "==", "ne": "!=", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}
	negation := map[string]string{"==": "!=", "!=": "==", ">": "<=", ">=": "<", "<": ">=", "<=": ">"}
	for word, sym := range symbol {
		if opSymbol(word) != sym || opSymbol(sym) != sym {
			t.Errorf("opSymbol(%q/%q) = %q/%q, want %q", word, sym, opSymbol(word), opSymbol(sym), sym)
		}
		if got, ok := negateOp(word); !ok || got != negation[sym] {
			t.Errorf("negateOp(%q) = %q,%v", word, got, ok)
		}
		if got, ok := negateOp(sym); !ok || got != negation[sym] {
			t.Errorf("negateOp(%q) = %q,%v", sym, got, ok)
		}
	}
	if got, ok := negateOp("matches"); ok || got != "" {
		t.Error("negateOp must refuse unknown ops")
	}
	if opSymbol("matches") != "matches" {
		t.Error("opSymbol renders unknown ops raw")
	}
}

func TestWriteSignals_FourLinesOnlyWhenPresent(t *testing.T) {
	var b strings.Builder
	writeSignals(&b, router.RoutingSignals{
		Scout:  router.ScoutSignals{Present: true, CycleSizeEstimate: "medium", GoalType: "business-strategy", DeliverableKind: "document"},
		Triage: router.TriageSignals{Present: true, CycleSize: "medium", DeliverableKind: "document"},
	})
	out := b.String()
	if !strings.Contains(out, "goal_type=business-strategy") || strings.Count(out, "deliverable_kind=") != 2 || strings.Contains(out, "- build:") || strings.Contains(out, "- audit:") {
		t.Errorf("scout+triage only, both carrying deliverable_kind:\n%s", out)
	}
	b.Reset()
	writeSignals(&b, router.RoutingSignals{Build: router.BuildSignals{Present: true, Verdict: "PASS", SeverityMax: router.SevHigh}, Audit: router.AuditSignals{Present: true, Verdict: "FAIL", Confidence: 0.5, RedCount: 2}})
	if got := b.String(); !strings.Contains(got, "- build: verdict=PASS acs_green=0 acs_red=0 acs_regression=0 severity_max=HIGH") || !strings.Contains(got, "- audit: verdict=FAIL confidence=0.50 red_count=2") {
		t.Errorf("build+audit lines:\n%s", got)
	}
	b.Reset()
	writeSignals(&b, router.RoutingSignals{})
	if b.Len() != 0 {
		t.Errorf("absent signals render nothing: %q", b.String())
	}
}

func TestWriteRecallMemory_EmitsNothingWithoutHistory(t *testing.T) {
	var b strings.Builder
	writeRecallMemory(&b, router.RouteInput{})
	if b.Len() != 0 {
		t.Errorf("empty recall must render nothing, got %q", b.String())
	}
	writeRecallMemory(&b, router.RouteInput{LastReason: "EGPS red_count=3", Lessons: []string{"inst-L001 (egps-red): Run the suite first"}})
	if out := b.String(); !strings.Contains(out, "## Recall memory") || !strings.Contains(out, "- why the last cycle failed: EGPS red_count=3") || !strings.Contains(out, "- lesson: inst-L001") {
		t.Errorf("recall output:\n%s", out)
	}
	b.Reset()
	writeRecallMemory(&b, router.RouteInput{Lessons: []string{"l"}})
	if out := b.String(); strings.Contains(out, "why the last cycle failed") || !strings.Contains(out, "- lesson: l") {
		t.Errorf("lessons without a reason:\n%s", out)
	}
}

func TestIsFailureTransition_AliasesAndNormalisation(t *testing.T) {
	for _, c := range []struct {
		current, verdict string
		want             bool
	}{
		{"retro", "PASS", true}, {" Retrospective ", "", true}, {"audit", "FAIL", true},
		{"audit", "PASS", false}, {"build", "FAIL", false}, {"", "FAIL", false},
	} {
		if got := isFailureTransition(router.RouteInput{Current: c.current, Verdict: c.verdict}); got != c.want {
			t.Errorf("isFailureTransition(%q, %q) = %v", c.current, c.verdict, got)
		}
	}
	p := buildRoutingPrompt(router.RouteInput{Current: "retrospective", Verdict: "FAIL"})
	for _, want := range []string{"recovery_action", "learning_richness", "non-overridable", `"recovery_action":"retry|end"`} {
		if !strings.Contains(p, want) {
			t.Errorf("failure-transition prompt missing %q", want)
		}
	}
	if pa := buildRoutingPrompt(router.RouteInput{Current: "audit", Verdict: "FAIL"}); !strings.Contains(pa, "learning_richness") {
		t.Error("audit-FAIL prompt must carry the failure vocabulary")
	}
	for _, in := range []router.RouteInput{{Current: "build", Verdict: "PASS"}, {Current: "audit", Verdict: "PASS"}} {
		if p := buildRoutingPrompt(in); strings.Contains(p, "recovery_action") || !strings.Contains(p, `{"next_phase":"<phase>","insert_phases":["<phase>",...],"justification":"<one sentence>"}`) {
			t.Errorf("happy-path prompt for %s must not carry failure vocabulary", in.Current)
		}
	}
}

func TestWriteFailureVocabulary_NamesEveryFailureInsertPhase(t *testing.T) {
	got := buildRoutingPrompt(router.RouteInput{Current: "retrospective", Verdict: "FAIL"})
	for _, p := range router.FailureInsertPhases() {
		if !strings.Contains(got, p) {
			t.Errorf("failure vocabulary missing kernel insert phase %q", p)
		}
	}
	if !strings.Contains(got, strings.Join(router.FailureInsertPhases(), " or ")) {
		t.Error("the insert phases are joined by OR from the kernel map")
	}
}

// Test 35 — the persona composition, the legacy fallback, the absolute
// artifact path equal to the launch's, and the mint schema in both prompts.
func TestComposePlanPrompt_PersonaCompositionAndAbsoluteArtifactPath(t *testing.T) {
	const ws = "/tmp/ws-abs-artifact-test"
	fl := &fakeLauncher{stdout: planJSON()}
	a := New(fl, Identity{CLI: "claude-tmux", Persona: "PERSONA_MARKER_42", AgentLabel: "router"}, nil)
	if _, err := a.Plan(router.RouteInput{Workspace: ws, Cycle: 7}); err != nil {
		t.Fatal(err)
	}
	want := ws + "/routing-plan.json"
	if !strings.Contains(fl.gotReq.Prompt, "PERSONA_MARKER_42") || !strings.Contains(fl.gotReq.Prompt, "# This cycle") || !strings.Contains(fl.gotReq.Prompt, want) || fl.gotReq.ArtifactPath != want {
		t.Errorf("persona + dynamic context + the ABSOLUTE artifact path the bridge watches:\n%s", fl.gotReq.Prompt)
	}
	fl = &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, defaultIdentity(), nil).Plan(router.RouteInput{Workspace: ws, Cycle: 7}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fl.gotReq.Prompt, "PHASE ADVISOR") || strings.Contains(fl.gotReq.Prompt, "# This cycle") {
		t.Error("no persona ⇒ the legacy inline framing")
	}
	prompts := map[string]string{
		"legacy":  buildPlanPrompt(baseRouteInput()),
		"persona": a.ComposePlanPrompt(baseRouteInput(), "routing-plan.json"),
	}
	for name, got := range prompts {
		for _, want := range []string{`"mint":{`, "fast|balanced|deep", "never a raw model", "writes_source", `"description"`, `"when_to_use"`, `[{"phase":"<phase>","run":true`} {
			if !strings.Contains(got, want) {
				t.Errorf("%s plan prompt missing %q", name, want)
			}
		}
		if strings.Contains(got, `"next_phase":"<phase>"`) {
			t.Errorf("%s plan prompt carries the per-transition object spec", name)
		}
	}
	if got := buildPlanPrompt(router.RouteInput{Cycle: 5, GoalText: "redesign the auth subsystem"}); !strings.Contains(got, "## Goal\nredesign the auth subsystem\n") {
		t.Errorf("the plan prompt surfaces the goal text:\n%s", got)
	}
	if strings.Contains(buildPlanPrompt(router.RouteInput{Cycle: 5}), "## Goal") {
		t.Error("empty GoalText must not emit a Goal section")
	}
}

// TruncateGoal is textcap's rule at the advisor's bound.
func TestTruncateGoal_CapsAtTheAdvisorBound(t *testing.T) {
	long := strings.Repeat("g", MaxGoalTextRunes+7)
	got := TruncateGoal("  " + long + "  ")
	if !strings.HasSuffix(got, " …[truncated]") || len([]rune(got)) != MaxGoalTextRunes+len([]rune(" …[truncated]")) {
		t.Errorf("TruncateGoal caps at %d runes with the marker: %d runes", MaxGoalTextRunes, len([]rune(got)))
	}
	if TruncateGoal("  short  ") != "short" || TruncateGoal("   ") != "" {
		t.Error("TruncateGoal trims; whitespace-only ⇒ empty (no Goal section)")
	}
	if MaxGoalTextRunes != 4000 {
		t.Errorf("MaxGoalTextRunes = %d, want 4000", MaxGoalTextRunes)
	}
}

// The plan prompt shares the routing context (goal, carryover todos, the
// rubric, the four signal lines, the optional-triggers block) with the
// per-transition prompt but asks for the whole-cycle ARRAY shape — the two
// cadences diverge correctly (the core RendersGoal / RendersCarryoverTodos /
// WholeCycleArray / FullSignalsAndTriggers intents, moved).
func TestBuildPlanPrompt_SharesTheRoutingContextAndAsksForTheArray(t *testing.T) {
	in := router.RouteInput{
		Current: "start", Cycle: 3, Completed: []string{},
		GoalText:       "redesign the auth subsystem with a new token-rotation architecture",
		CarryoverTodos: []router.CarryoverTodo{{ID: "cycle-4-failed-build", Action: "Review failed build learning and fix missing audit binding", Priority: "P0", FirstSeenCycle: 4, CyclesUnpicked: 1}},
		Signals: router.RoutingSignals{
			Scout:  router.ScoutSignals{Present: true, CycleSizeEstimate: "medium", ItemCount: 3, CarryoverCount: 1},
			Triage: router.TriageSignals{Present: true, CycleSize: "small", PhaseSkip: []string{"plan-review"}},
			Build:  router.BuildSignals{Present: true, Verdict: "PASS", ACSGreen: 5, ACSRed: 1, FilesTouched: 4},
			Audit:  router.AuditSignals{Present: true, Verdict: "PASS", Confidence: 0.9, RedCount: 0},
		},
		Cfg: config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}, MaxInsertions: 4, Triggers: map[string]config.RoutingBlock{
			"scout":       {RubricHint: []string{"scout.carryover_count >= 3 → skip scout (work already queued)"}},
			"tester":      {InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}}},
			"plan-review": {},
		}},
	}
	got := buildPlanPrompt(in)
	for _, want := range []string{
		"## Goal\nredesign the auth subsystem", "## Carryover todos from previous cycles", "- [P0] cycle-4-failed-build: Review failed build learning and fix missing audit binding (first_seen_cycle=4, cycles_unpicked=1)",
		"scout: cycle_size_estimate=medium", "triage: cycle_size=small", "build: verdict=PASS", "audit: verdict=PASS",
		"- plan-review\n- scout\n- tester\n", "## Decision rubric", "skip scout (work already queued)", "insert tester",
		"FORBIDDEN: never propose reaching ship without audit", `[{"phase":"<phase>","run":true`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plan prompt missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, `"next_phase":"<phase>"`) {
		t.Error("the plan prompt must not carry the per-transition object spec")
	}
}
