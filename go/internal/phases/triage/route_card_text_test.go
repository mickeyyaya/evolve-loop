package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestRoutedCardDiagnostics_ACommittedRouteCardNamesItsReasonNotAProtectedSurface(t *testing.T) {
	t.Parallel()
	cards := []protectedCard{{ID: "x", Reason: "route:console-only", Committed: true}}

	diags := routedCardDiagnostics(cards)

	want := `committed item "x" is console-routed (route:console-only): routed to the console by the host (escalate_block), not committed. Next: the console builds it. This lane has no work for it.`
	if len(diags) != 1 || diags[0].Message != want {
		t.Fatalf("message = %+v\nwant %q", diags, want)
	}
	if strings.Contains(diags[0].Message, "protected surface") {
		t.Fatalf("a route card has no protected surface: %q", diags[0].Message)
	}
	if diags[0].Subject != "x" || diags[0].Severity != "warning" || diags[0].Code != cyclestate.DiagCodeTriageProtectedSurface {
		t.Fatalf("subject and severity: %+v", diags[0])
	}
}

func TestRoutedCardDiagnostics_ATopNCardStillNamesItsProtectedSurface(t *testing.T) {
	t.Parallel()
	diags := routedCardDiagnostics([]protectedCard{{ID: "y", Path: "go/internal/guards/role.go"}})

	want := `top_n card "y" names protected surface "go/internal/guards/role.go" — routed to the console by the host (escalate_block), not committed`
	if len(diags) != 1 || diags[0].Message != want {
		t.Fatalf("message = %+v\nwant %q", diags, want)
	}
}

func classifyRouted(t *testing.T, decision, artifact string) (string, []core.Diagnostic) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, root, "a-x.json", routedItemBody)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, RunID: "r", ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := committedset.RecordRoutes(root, 7, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if decision != "" {
		writeDecision(t, ws, decision)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Cycle: 7, ProjectRoot: root, Workspace: ws}, core.BridgeResponse{})
	return verdict, diags
}

const routeCardWarning = `committed item "x" is console-routed (route:console-only): routed to the console by the host (escalate_block), not committed. Next: the console builds it. This lane has no work for it.`

const routedItemBody = `{"id":"x","title":"fixture","weight":0.9,"route":"console-only"}`

func TestTriageClassify_TheEmptyTopNBranchReturnsTheRouteDiagnostics(t *testing.T) {
	t.Parallel()

	verdict, diags := classifyRouted(t, `{"cycle":7,"top_n":[],"deferred":[]}`, "## top_n\n")

	if verdict != core.VerdictPASS || len(diags) != 1 || diags[0].Message != routeCardWarning || diags[0].Subject != "x" {
		t.Fatalf("a routed pin whose only item left passes and says why: %s %+v", verdict, diags)
	}
}

func TestTriageClassify_ANoneTopNBranchReturnsTheRouteDiagnostics(t *testing.T) {
	t.Parallel()

	verdict, diags := classifyRouted(t, `{"cycle":7,"top_n":[],"deferred":[]}`, "## top_n\nnone\n")

	if verdict != core.VerdictPASS || len(diags) != 1 || diags[0].Message != routeCardWarning {
		t.Fatalf("none is the same empty branch: %s %+v", verdict, diags)
	}
}

func TestTriageClassify_ARecordedRouteNeedsTheProjectRootToNameItsReason(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, root, "a-x.json", routedItemBody)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, RunID: "r", ItemIDs: []string{"x"}, GoalHash: "g", RoutedIDs: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	diags := routedCommittedDiagnostics(core.PhaseRequest{Cycle: 7, Workspace: ws}, nil)

	if len(diags) != 0 {
		t.Fatalf("a request with no project root reads no inbox from the working directory: %+v", diags)
	}
}

func TestTriageClassify_AnUnroutedCommittedItemHasNoRouteDiagnostic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, root, "a-x.json", `{"id":"x","title":"fixture","weight":0.9}`)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, RunID: "r", ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}

	diags := routedCommittedDiagnostics(core.PhaseRequest{Cycle: 7, ProjectRoot: root, Workspace: ws}, nil)

	if len(diags) != 0 {
		t.Fatalf("%+v", diags)
	}
}

func TestTriageClassify_ARecordedRouteWhoseItemNoLongerRoutesHasNoRouteDiagnostic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, root, "a-x.json", `{"id":"x","title":"fixture","weight":0.9}`)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, RunID: "r", ItemIDs: []string{"x"}, GoalHash: "g", RoutedIDs: []string{"x"}}); err != nil {
		t.Fatal(err)
	}

	diags := routedCommittedDiagnostics(core.PhaseRequest{Cycle: 7, ProjectRoot: root, Workspace: ws}, nil)

	if len(diags) != 0 {
		t.Fatalf("%+v", diags)
	}
}
