package cycleoutcome

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestBuildStage_OnlyAPhaseRecordProvesWhereTheCycleStopped(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if pre, past := buildStage(ws, retryladder.Unknown); pre || past {
		t.Errorf("no phase record = (%v, %v), want (false, false): the stage is unknown", pre, past)
	}
	writeJSON(t, phasetiming.Path(ws), []phasetiming.Entry{{Phase: "scout", Verdict: cyclestate.VerdictPASS}, {Phase: "triage", Verdict: cyclestate.VerdictFAIL}})
	for class, want := range map[retryladder.Class]bool{retryladder.Unknown: true, retryladder.Spec: true, retryladder.Infra: false, retryladder.Quota: false} {
		if pre, past := buildStage(ws, class); pre != want || past {
			t.Errorf("buildStage(%s) = (%v, %v), want (%v, false)", class, pre, past, want)
		}
	}
	writeJSON(t, phasetiming.Path(ws), []phasetiming.Entry{{Phase: "triage", Verdict: cyclestate.VerdictPASS}, {Phase: "build", Verdict: cyclestate.VerdictFAIL}})
	if pre, past := buildStage(ws, retryladder.Infra); pre || !past {
		t.Errorf("a cycle that reached build = (%v, %v), want (false, true)", pre, past)
	}
}

func TestFailureInputsFor_CarriesThePrebuildCeilingAndTheBeforeBuildFact(t *testing.T) {
	t.Parallel()
	_, _, ws := seedProject(t, "p", []string{"p"})
	writeJSON(t, phasetiming.Path(ws), []phasetiming.Entry{{Phase: "scout", Verdict: cyclestate.VerdictFAIL}})
	in := FailureInputsFor(t.TempDir(), t.TempDir(), ws, 7, io.Discard)
	if !in.Prebuild || in.PastBuild || in.PrebuildCeiling != 2 {
		t.Errorf("Prebuild = %v, PrebuildCeiling = %d; want true, 2", in.Prebuild, in.PrebuildCeiling)
	}
}

func TestRouteProtectedAnswers_RoutesEachPresentUnroutedItemTriageAnsweredAsProtected(t *testing.T) {
	t.Parallel()
	root, inbox, ws := seedProject(t, "guarded", nil)
	writeJSON(t, filepath.Join(inbox, "routed.json"), map[string]any{"id": "routed", "route": "console-manual"})
	writeJSON(t, filepath.Join(inbox, "plain.json"), map[string]any{"id": "plain"})
	writeJSON(t, filepath.Join(ws, "triage-decision.json"), map[string]any{
		"top_n":          []any{},
		"escalate_block": []map[string]string{{"task_id": "guarded", "reason": "protected-surface: go/internal/guards/role.go"}, {"task_id": "routed", "reason": "protected-surface: x"}, {"task_id": "gone", "reason": "protected-surface: y"}},
		"dropped":        []map[string]string{{"id": "plain", "reason": "duplicate of another item"}},
	})
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	got := routeProtectedAnswers(opts, FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 7}, io.Discard)
	if len(got) != 1 || got[0] != "guarded" {
		t.Fatalf("routed = %v, want [guarded]", got)
	}
	if rec := itemRecord(t, inbox, "guarded"); rec["route"] != "console-manual" || !strings.HasPrefix(rec["routed_reason"].(string), "protected-surface:") {
		t.Errorf("item = %v", rec)
	}
	if rec := itemRecord(t, inbox, "plain"); rec["route"] != nil {
		t.Errorf("an answer with no protected reason is not routed: %v", rec)
	}
}

func TestRouteProtectedAnswers_AnItemHeldByAnotherCycleIsReportedAndNotCounted(t *testing.T) {
	t.Parallel()
	root, inbox, ws := seedProject(t, "guarded", nil)
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "guarded", "99"); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(ws, "triage-decision.json"), map[string]any{
		"escalate_block": []map[string]string{{"task_id": "guarded", "reason": "protected-surface: go/internal/guards/role.go"}},
	})
	var stderr strings.Builder
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	got := routeProtectedAnswers(opts, FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 7}, &stderr)
	if len(got) != 0 || !strings.Contains(stderr.String(), "was not routed to the console") {
		t.Errorf("routed = %v, stderr = %q", got, stderr.String())
	}
	if rec := itemRecord(t, filepath.Join(inbox, "processing", "cycle-99"), "guarded"); rec["route"] != nil {
		t.Errorf("another cycle's claim is not stamped: %v", rec)
	}
}
