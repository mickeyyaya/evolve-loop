package deliverable

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const routeCycle = 7

type routeFixture struct {
	pin      []string
	items    map[string]string
	report   string
	decision string
	noRecord bool
}

type routeWorld struct {
	root, ws string
	calls    []claimCall
	fx       *HostEffects
	in       core.ReviewInput
}

func (f routeFixture) build(t *testing.T) *routeWorld {
	t.Helper()
	w := &routeWorld{root: t.TempDir()}
	w.ws = filepath.Join(w.root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(w.ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(w.root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	for id, body := range f.items {
		writeFile(t, filepath.Join(w.root, ".evolve", "inbox"), "2026-01-01T00-00-00Z-"+id+".json", body)
	}
	if !f.noRecord {
		if err := committedset.WriteRecord(w.root, committedset.Record{Cycle: routeCycle, RunID: "r", ItemIDs: f.pin, GoalHash: "g"}); err != nil {
			t.Fatal(err)
		}
	}
	if f.report != "" {
		writeFile(t, w.ws, "triage-report.md", f.report)
	}
	if f.decision != "" {
		writeFile(t, w.ws, "triage-decision.json", f.decision)
	}
	w.fx = NewHostEffects(routeCatalog(t), recordingClaimer(&w.calls), nil)
	w.in = core.ReviewInput{Cycle: routeCycle, Phase: "triage", Workspace: w.ws, ProjectRoot: w.root}
	return w
}

func routeCatalog(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{{
		Name: "triage", Role: "triage",
		Outputs: phasespec.IO{
			Files:       []string{".evolve/runs/cycle-{cycle}/triage-report.md", ".evolve/runs/cycle-{cycle}/triage-decision.json"},
			AgentOwed:   []string{"triage-decision.json"},
			DerivedFrom: map[string]string{"triage-decision.json": "triage-report.md"},
		},
		Effects: []string{"inbox-claim"},
	}})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	return cat
}

func (w *routeWorld) decisionTopN() []string {
	ids, _ := committedset.DecisionTopN(w.ws)
	return ids
}

func (w *routeWorld) escalated(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(w.ws, "triage-decision.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Escalate []struct {
			TaskID string `json:"task_id"`
			Reason string `json:"reason"`
		} `json:"escalate_block"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range d.Escalate {
		out = append(out, e.TaskID+"|"+e.Reason)
	}
	return out
}

const (
	routedX = `{"id":"x","title":"routed","weight":0.9,"route":"console-only"}`
	plainY  = `{"id":"y","title":"plain","weight":0.8}`
	xyCards = "# Triage\n\n## top_n\n- x: one — priority=H\n- y: two — priority=H\n"
	xyTopN  = `{"cycle":7,"top_n":[{"id":"x"},{"id":"y"}],"deferred":[]}`
)

func twoItems() map[string]string { return map[string]string{"x": routedX, "y": plainY} }

func TestRouteEffect_RecordsThePinnedRouteAndTakesItOutOfTopNBeforeTheClaim(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN}.build(t)

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatalf("Perform: %v", err)
	}

	if got := committedset.RoutedIDs(w.ws); len(got) != 1 || got[0] != "x" {
		t.Fatalf("RoutedIDs = %v", got)
	}
	if got := w.decisionTopN(); len(got) != 1 || got[0] != "y" {
		t.Fatalf("top_n = %v", got)
	}
	want := "x|console-routed: route:console-only — operator-owned work is never built by a lane"
	if got := w.escalated(t); len(got) != 1 || got[0] != want {
		t.Fatalf("escalate_block = %q, want %q", got, want)
	}
	if len(w.calls) != 1 || len(w.calls[0].ids) != 1 || w.calls[0].ids[0] != "y" {
		t.Fatalf("the claim runs after the route and takes only y: %+v", w.calls)
	}
}

func TestRouteEffect_RoutesAnUnpinnedTopNItemToo(t *testing.T) {
	w := routeFixture{pin: nil, items: twoItems(), decision: xyTopN}.build(t)

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	if got := committedset.RoutedIDs(w.ws); len(got) != 1 || got[0] != "x" {
		t.Fatalf("RoutedIDs = %v", got)
	}
	if got := w.decisionTopN(); len(got) != 1 || got[0] != "y" {
		t.Fatalf("top_n = %v", got)
	}
}

func TestRouteEffect_ADeferredRoutedItemIsNotRouted(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(),
		decision: `{"cycle":7,"top_n":[{"id":"x"},{"id":"y"}],"deferred":[{"id":"x"}]}`}.build(t)

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	if got := committedset.RoutedIDs(w.ws); len(got) != 0 {
		t.Fatalf("a deferred item is answered, so it is not routed: %v", got)
	}
}

func TestRouteEffect_ASecondRunReAppliesTheRecordedRouteToARewrittenDecision(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN}.build(t)
	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w.ws, "triage-decision.json", xyTopN)

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	if got := w.decisionTopN(); len(got) != 1 || got[0] != "y" {
		t.Fatalf("a rewritten decision cannot keep x in top_n: %v", got)
	}
	if got := w.escalated(t); len(got) != 1 {
		t.Fatalf("one escalation per route: %q", got)
	}
}

func TestRouteEffect_AReRoutedRecordWhoseResolverNoLongerRoutesKeepsTheRecordedFact(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: map[string]string{"x": plainY, "y": plainY}, decision: xyTopN}.build(t)
	if err := committedset.RecordWorkspaceRoutes(w.ws, []string{"x"}); err != nil {
		t.Fatal(err)
	}

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	got := w.escalated(t)
	if len(got) != 1 || got[0] != "x|console-routed: the host recorded this route earlier in the cycle — operator-owned work is never built by a lane" {
		t.Fatalf("escalate_block = %q", got)
	}
}

func TestRouteEffect_AReportOfNoneIsDerivedWhenTheWholePinWasRouted(t *testing.T) {
	w := routeFixture{pin: []string{"x"}, items: twoItems(), report: "# Triage\n\n## top_n\nnone\n"}.build(t)

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatalf("Perform: %v", err)
	}

	if _, decided := committedset.DecisionTopN(w.ws); !decided || len(w.decisionTopN()) != 0 {
		t.Fatalf("the host derives an empty decision: %v", w.decisionTopN())
	}
	if got := w.escalated(t); len(got) != 1 || !strings.HasPrefix(got[0], "x|") {
		t.Fatalf("x is escalated: %q", got)
	}
}

func TestRouteEffect_AnUnroutedPinStillRejectsANoneReport(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), report: "# Triage\n\n## top_n\nnone\n"}.build(t)

	err := w.fx.Perform(context.Background(), w.in)

	if err == nil || !strings.Contains(err.Error(), "derive triage-decision.json") {
		t.Fatalf("err = %v", err)
	}
	if strings.Count(err.Error(), "derive triage-decision.json") != 1 {
		t.Fatalf("the derive failure is reported once: %v", err)
	}
}

func TestRouteEffect_ARecordFailureIsALoudErrorAndLeavesTheCardInTopN(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN, noRecord: true}.build(t)
	writeFile(t, w.ws, "lane-scope.json", `{"todo_ids":["x","y"],"goal_hash":"g"}`)

	err := w.fx.Perform(context.Background(), w.in)

	if err == nil || !strings.Contains(err.Error(), `route "x" (route:console-only)`) {
		t.Fatalf("the error names the id and the reason: %v", err)
	}
	if got := w.decisionTopN(); len(got) != 2 {
		t.Fatalf("a failed record leaves the decision alone: %v", got)
	}
}

func TestRouteEffect_TheRecordIsWrittenBeforeTheDecisionIsEdited(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: `{"cycle":7,"top_n":[{"id":"x"},{"id":"y"}],"escalate_block":"none"}`}.build(t)

	err := w.fx.Perform(context.Background(), w.in)

	if err == nil || !strings.Contains(err.Error(), "route cards") {
		t.Fatalf("an unwritable decision edit is an error: %v", err)
	}
	if got := committedset.RoutedIDs(w.ws); len(got) != 1 || got[0] != "x" {
		t.Fatalf("the route was recorded before the edit failed: %v", got)
	}
}

func TestRouteEffect_ARelativeEvolveDirIsRefusedAndReadsNoInboxFromTheWorkingDirectory(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN}.build(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(w.root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	in := w.in
	in.ProjectRoot = ""

	err = w.fx.Perform(context.Background(), in)

	if err == nil || !strings.Contains(err.Error(), "no cycle or project root to route into") {
		t.Fatalf("err = %v", err)
	}
	if got := committedset.RoutedIDs(w.ws); len(got) != 0 {
		t.Fatalf("nothing is recorded: %v", got)
	}
}

func TestRouteEffect_APhaseWithNoCandidatesNeedsNoProjectRoot(t *testing.T) {
	w := routeFixture{pin: nil, items: nil, decision: `{"cycle":7,"top_n":[],"deferred":[]}`}.build(t)
	in := w.in
	in.ProjectRoot = ""

	if err := w.fx.Perform(context.Background(), in); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestRouteEffect_AnUnroutedPinIsLeftAlone(t *testing.T) {
	w := routeFixture{pin: []string{"y"}, items: twoItems(), decision: `{"cycle":7,"top_n":[{"id":"y"}],"deferred":[]}`}.build(t)

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	if got := committedset.RoutedIDs(w.ws); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	if got := w.decisionTopN(); len(got) != 1 {
		t.Fatalf("%v", got)
	}
}

func TestRouteEffect_TheProtectedFilesPredicateOfTheHostRoutesAnItem(t *testing.T) {
	w := routeFixture{pin: []string{"y"}, items: map[string]string{"y": `{"id":"y","title":"plain","weight":0.8,"files":["go/internal/guards/role.go"]}`},
		decision: `{"cycle":7,"top_n":[{"id":"y"}],"deferred":[]}`}.build(t)

	if err := NewHostEffects(routeCatalog(t), recordingClaimer(&w.calls), func(p string) bool { return p == "go/internal/guards/role.go" }).Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	if got := committedset.RoutedIDs(w.ws); len(got) != 1 || got[0] != "y" {
		t.Fatalf("RoutedIDs = %v", got)
	}
}

func TestFlagRoutedInTopN_ARoutedItemStillInTopNIsAMissingEffectNamingTheEdit(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN}.build(t)
	if err := committedset.RecordWorkspaceRoutes(w.ws, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	res := &Result{}

	flagRoutedInTopN(res, phasecontract.Roots{Workspace: w.ws, EvolveDir: filepath.Join(w.root, ".evolve"), Cycle: routeCycle})

	want := `review gate: console-routed item "x" is still in top_n of triage-decision.json. The host removes it from top_n and lists it under escalate_block. Next: the console reads the error with evolve signals tail --cycle 7 --code RUNNER_HOST_EFFECT_FAILED,ORCHESTRATOR_HOST_EFFECT_FAILED.`
	if len(res.Violations) != 1 || res.Violations[0].Code != CodeMissingEffect || res.Violations[0].Message != want {
		t.Fatalf("%+v", res.Violations)
	}
}

func TestFlagRoutedInTopN_APassingStateAddsNoViolation(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: `{"cycle":7,"top_n":[{"id":"y"}],"deferred":[]}`}.build(t)
	if err := committedset.RecordWorkspaceRoutes(w.ws, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	res := &Result{}

	flagRoutedInTopN(res, phasecontract.Roots{Workspace: w.ws, Cycle: routeCycle})

	if len(res.Violations) != 0 {
		t.Fatalf("%+v", res.Violations)
	}
}

func TestInboxClaimEntry_ChecksTheRouteBeforeTheClaim(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN}.build(t)
	if err := committedset.RecordWorkspaceRoutes(w.ws, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	roots := phasecontract.Roots{Workspace: w.ws, EvolveDir: filepath.Join(w.root, ".evolve"), Cycle: routeCycle}
	res := &Result{}

	if err := effects[phasecontract.EffectInboxClaim].check(res, roots); err != nil {
		t.Fatal(err)
	}

	if len(res.Violations) < 1 || !strings.Contains(res.Violations[0].Message, "still in top_n") {
		t.Fatalf("the route check runs first: %+v", res.Violations)
	}
}

func TestRouteFailureOnAConsoleOnlyItem_GivesTheTrueGateText(t *testing.T) {
	w := routeFixture{pin: []string{"x"}, items: map[string]string{"x": routedX}, decision: `{"cycle":7,"top_n":[{"id":"x"}],"deferred":[]}`, noRecord: true}.build(t)
	writeFile(t, w.ws, "lane-scope.json", `{"todo_ids":["x"],"goal_hash":"g"}`)

	if err := w.fx.Perform(context.Background(), w.in); err == nil {
		t.Fatal("the route cannot be recorded")
	}
	roots := RootsFor(w.in)
	res, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("inbox-claim")), config.StageOff)
	if err != nil {
		t.Fatal(err)
	}

	if len(w.calls) != 1 || len(w.calls[0].ids) != 1 {
		t.Fatalf("the claimer returned nil and was asked for x: %+v", w.calls)
	}
	want := `review gate: the host tried to route and claim committed item "x", and neither landed. If the item can be claimed, an answer of deferred ends this cycle as FAIL. Next: the console reads the error with evolve signals tail --cycle 7 --code RUNNER_HOST_EFFECT_FAILED,ORCHESTRATOR_HOST_EFFECT_FAILED.`
	if res.OK || len(res.Violations) == 0 || res.Violations[len(res.Violations)-1].Message != want {
		t.Fatalf("%+v", res.Violations)
	}
}

func TestRouteEffect_ARecordFailureOnATopNOnlyRouteIsALoudError(t *testing.T) {
	w := routeFixture{pin: nil, items: twoItems(), decision: xyTopN, noRecord: true}.build(t)

	err := w.fx.Perform(context.Background(), w.in)

	if err == nil || !strings.Contains(err.Error(), `route "x" (route:console-only)`) {
		t.Fatalf("err = %v", err)
	}
}

func TestRouteEffect_ARecordedRouteWithAnEmptyPinAndAnEmptyTopNIsWrittenToTheDecision(t *testing.T) {
	w := routeFixture{pin: nil, items: map[string]string{"x": plainY}, decision: `{"cycle":7,"top_n":[],"deferred":[]}`}.build(t)
	if err := committedset.RecordWorkspaceRoutes(w.ws, []string{"x"}); err != nil {
		t.Fatal(err)
	}

	if err := w.fx.Perform(context.Background(), w.in); err != nil {
		t.Fatal(err)
	}

	if got := w.escalated(t); len(got) != 1 || !strings.HasPrefix(got[0], "x|") {
		t.Fatalf("escalate_block = %q, want one line for x", got)
	}
}

func TestRouteEffect_ACycleOfZeroIsRefusedBeforeAnyRoute(t *testing.T) {
	w := routeFixture{pin: []string{"x", "y"}, items: twoItems(), decision: xyTopN}.build(t)
	in := w.in
	in.Cycle = 0

	err := w.fx.Perform(context.Background(), in)

	if err == nil || !strings.Contains(err.Error(), "no cycle or project root to route into") {
		t.Fatalf("err = %v", err)
	}
	if got := committedset.RoutedIDs(w.ws); len(got) != 0 {
		t.Fatalf("nothing is recorded: %v", got)
	}
}
