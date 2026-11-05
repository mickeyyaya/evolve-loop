package deliverable

import (
	"context"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const pinnedToX = `{"todo_ids":["x"],"goal_hash":"g"}`

const hostRoutedDecision = `{"cycle":7,"top_n":[],"deferred":[],"escalate_block":[{"task_id":"x","reason":"console-routed: route:console-only — operator-owned work is never built by a lane"}]}`

const (
	cellFormMessage      = `review gate: committed item "x" is not claimed yet. The host claims it after this phase ends.`
	claimAttemptedFormat = `review gate: the host tried to route and claim committed item "x", and neither landed. If the item can be claimed, an answer of deferred ends this cycle as FAIL. Next: the console reads the error with evolve signals tail --cycle %d --code RUNNER_HOST_EFFECT_FAILED,ORCHESTRATOR_HOST_EFFECT_FAILED.`
	dependencyWaitText   = `review gate: committed item "x" waits on the dependency "y", so the host did not claim it. Next: list "x" under deferred in the triage decision.`
)

func writeInboxItem(t *testing.T, evolveDir, dirName, body string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	if dirName != "" {
		dir = filepath.Join(dir, dirName)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "2026-01-01T00-00-00Z-x.json", body)
}

func verifyClaim(t *testing.T, ws, evolveDir string) Result {
	t.Helper()
	roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir, Cycle: 7}
	res, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("inbox-claim")), config.StageOff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res
}

func TestCheckInboxClaim_AHostRoutedItemHasLeftTheCommitmentAndOwesNoClaim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ws, "triage-report.md", validTriageReport)
	writeFile(t, ws, "triage-decision.json", hostRoutedDecision)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := committedset.RecordRoutes(root, 7, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, evolveDir, "", `{"id":"x","title":"fixture","route":"console-only"}`)
	if res := verifyClaim(t, ws, evolveDir); !res.OK {
		t.Fatalf("an escalated pending item is not committed, so it owes no claim: %+v", res.Violations)
	}
}

func TestCheckInboxClaim_AConsoleRoutedItemStillCommittedOwesTheClaim(t *testing.T) {
	t.Parallel()
	ws, evolveDir := claimFixture{decision: committedToX}.layout(t)
	writeInboxItem(t, evolveDir, "", `{"id":"x","title":"fixture","route":"console-only"}`)
	res := verifyClaim(t, ws, evolveDir)
	if res.OK || !hasCode(res, CodeMissingEffect) {
		t.Fatalf("a routed item that the host step left committed must not pass: %+v", res)
	}
}

func TestCheckInboxClaim_UnroutedPendingItemStillOwesClaim(t *testing.T) {
	t.Parallel()
	for _, route := range []string{``, `,"route":"lane"`} {
		ws, evolveDir := claimFixture{decision: `{"cycle":7,"top_n":[],"deferred":[]}`}.layout(t)
		writeFile(t, ws, "lane-scope.json", pinnedToX)
		writeInboxItem(t, evolveDir, "", `{"id":"x","title":"fixture"`+route+`}`)
		res := verifyClaim(t, ws, evolveDir)
		if res.OK || !hasCode(res, CodeMissingEffect) {
			t.Fatalf("an unrouted pending item must still owe a claim: %+v", res)
		}
	}
}

func TestCheckInboxClaim_ItemHeldByAnotherCycleStillOwesClaim(t *testing.T) {
	t.Parallel()
	ws, evolveDir := claimFixture{decision: `{"cycle":7,"top_n":[],"deferred":[]}`}.layout(t)
	writeFile(t, ws, "lane-scope.json", pinnedToX)
	writeInboxItem(t, evolveDir, "processing/cycle-3", `{"id":"x","title":"fixture","route":"console-only"}`)
	res := verifyClaim(t, ws, evolveDir)
	if res.OK || !violationMentions(res, "cycle-3") {
		t.Fatalf("an item held by another cycle must still be reported: %+v", res)
	}
}

func verifyClaimWith(t *testing.T, ws, evolveDir string, attempted bool) Result {
	t.Helper()
	roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir, Cycle: 7, ClaimAttempted: attempted}
	res, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("inbox-claim")), config.StageOff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res
}

func onlyViolation(t *testing.T, res Result) string {
	t.Helper()
	if res.OK || len(res.Violations) != 1 || res.Violations[0].Code != CodeMissingEffect {
		t.Fatalf("want one missing_effect violation: %+v", res)
	}
	return res.Violations[0].Message
}

func TestCheckInboxClaim_TheCellFormSaysTheHostClaimsAfterThePhase(t *testing.T) {
	t.Parallel()
	ws, evolveDir := claimFixture{decision: committedToX, itemIn: "inbox"}.layout(t)

	if got := onlyViolation(t, verifyClaimWith(t, ws, evolveDir, false)); got != cellFormMessage {
		t.Fatalf("message drift.\n got: %s\nwant: %s", got, cellFormMessage)
	}
}

func TestCheckInboxClaim_AnAttemptedClaimThatDidNotLandIsAHostFaultText(t *testing.T) {
	t.Parallel()
	ws, evolveDir := claimFixture{decision: committedToX, itemIn: "inbox"}.layout(t)

	want := fmt.Sprintf(claimAttemptedFormat, 7)
	if got := onlyViolation(t, verifyClaimWith(t, ws, evolveDir, true)); got != want {
		t.Fatalf("message drift.\n got: %s\nwant: %s", got, want)
	}
}

func TestCheckInboxClaim_AnItemThatWaitsOnADependencyNamesItWhetherOrNotTheClaimRan(t *testing.T) {
	t.Parallel()
	for _, attempted := range []bool{false, true} {
		ws, evolveDir := claimFixture{decision: committedToX}.layout(t)
		inbox := filepath.Join(evolveDir, "inbox")
		writeFile(t, inbox, "2026-01-01T00-00-00Z-x.json", `{"id":"x","title":"fixture","deps":["y"]}`)
		writeFile(t, inbox, "2026-01-01T00-00-00Z-y.json", `{"id":"y","title":"first"}`)

		if got := onlyViolation(t, verifyClaimWith(t, ws, evolveDir, attempted)); got != dependencyWaitText {
			t.Fatalf("attempted=%v message drift.\n got: %s\nwant: %s", attempted, got, dependencyWaitText)
		}
	}
}

func TestRootsFor_TheHostPathStatesThatTheClaimWasAttempted(t *testing.T) {
	t.Parallel()
	if roots := RootsFor(core.ReviewInput{Cycle: 7, Phase: "triage"}); !roots.ClaimAttempted {
		t.Fatalf("the host performs effects before its review, so RootsFor must say so: %+v", roots)
	}
}

func TestCheckInboxClaim_TheDeferAnswerTheMessageNamesPasses(t *testing.T) {
	t.Parallel()
	ws, evolveDir := claimFixture{decision: `{"cycle":7,"top_n":[{"id":"x"}],"deferred":[{"id":"x"}]}`, itemIn: "inbox"}.layout(t)
	writeFile(t, ws, "lane-scope.json", pinnedToX)
	if res := verifyClaim(t, ws, evolveDir); !res.OK {
		t.Fatalf("the literal answer in the message must pass: %+v", res.Violations)
	}
}

func TestDeclaredEffects_PinnedItemRoutedByTheHost_EndsAsPlannedNoWork(t *testing.T) {
	t.Parallel()
	root := gitRepoWithOneCommit(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, pendingItemFile), []byte(`{"id":"x","title":"fixture","weight":0.9,"route":"console-only"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runners, triage := effectRunners(t, 0, true)
	inner := triage.onRun
	triage.onRun = func(n int, req core.PhaseRequest) {
		inner(n, req)
		if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), []byte(hostRoutedDecision), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := committedset.WriteRecord(req.ProjectRoot, committedset.Record{Cycle: req.Cycle, ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
			t.Fatal(err)
		}
		if err := committedset.RecordRoutes(req.ProjectRoot, req.Cycle, []string{"x"}); err != nil {
			t.Fatal(err)
		}
	}
	o := effectOrchestrator(t, root, runners, hostEffects(t))

	result, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})

	if err != nil {
		t.Fatalf("a lane whose pinned item the host routed must not fail the cycle: %v", err)
	}
	if len(triage.requests) != 1 || correctionNaming(triage.requests, "[missing_effect]") != 0 {
		t.Fatalf("no correction is owed: %d run(s): %q", len(triage.requests), directives(triage.requests))
	}
	if !core.IsTriageNoWorkResult(result) {
		t.Fatalf("want planned no-work, got verdict=%q termination=%q phases=%v", result.FinalVerdict, result.TerminationReason, result.PhasesRun)
	}
	for _, p := range []core.Phase{core.PhaseBuild, core.PhaseShip} {
		if n := len(runners[p].(*stubPhase).requests); n != 0 {
			t.Errorf("a lane must never build or ship an operator-owned item: %s ran %d time(s)", p, n)
		}
	}
}

func TestCheckInboxClaim_UnreadableInboxFailsLoudWithTheItemID(t *testing.T) {
	t.Parallel()
	ws, evolveDir := claimFixture{decision: committedToX}.layout(t)
	if err := os.Remove(filepath.Join(evolveDir, "inbox")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, evolveDir, "inbox", "not a directory")
	roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir, Cycle: 7}
	_, err := VerifyWithStage("triage", roots, resolverFor(triageSpecWithClaim("inbox-claim")), config.StageOff)
	if err == nil || !strings.Contains(err.Error(), `locate inbox item "x"`) {
		t.Fatalf("an unreadable inbox must surface as an error naming the item, got %v", err)
	}
}

func TestCheckInboxClaim_ABareEscalateBlockOnAPinnedItemStillOwesTheClaim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ws, "triage-report.md", validTriageReport)
	writeFile(t, ws, "triage-decision.json", hostRoutedDecision)
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, ItemIDs: []string{"x"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, evolveDir, "", `{"id":"x","title":"fixture"}`)

	res := verifyClaim(t, ws, evolveDir)

	if res.OK || !hasCode(res, CodeMissingEffect) {
		t.Fatalf("an escalate_block with no host route must not release the pin: %+v", res)
	}
}
