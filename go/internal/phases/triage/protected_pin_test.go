package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func escalationsOf(t *testing.T, path string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, e := range readDecision(t, path)["escalate_block"].([]any) {
		m := e.(map[string]any)
		out[m["task_id"].(string)] = m["reason"].(string)
	}
	return out
}

func TestRouteProtectedCards_AnEmptiedSingleItemLaneAnswersForItsPin(t *testing.T) {
	t.Parallel()
	path := writeDecision(t, t.TempDir(), `{"top_n":[{"id":"selection-alias"}],"escalate_block":[]}`)
	cards := []protectedCard{{ID: "selection-alias", Path: "go/internal/loopwave/loopwave.go"}}

	if err := routeProtectedCards(path, cards, []string{"pinned-inbox-item"}); err != nil {
		t.Fatal(err)
	}

	esc := escalationsOf(t, path)
	if len(esc) != 2 || esc["selection-alias"] == "" {
		t.Fatalf("the routed card and the lane's pin are both escalations: %v", esc)
	}
	pinReason := esc["pinned-inbox-item"]
	if !strings.HasPrefix(pinReason, consoleRouteReason("go/internal/loopwave/loopwave.go")) || !strings.Contains(pinReason, `routed card "selection-alias"`) {
		t.Errorf("the pin carries the route of the card that emptied the lane and names that card: %q", pinReason)
	}
}

func TestRouteProtectedCards_ThePinIsAnsweredOnlyWhenTheRouteEmptiesASingleItemLane(t *testing.T) {
	t.Parallel()
	card := []protectedCard{{ID: "protected-part", Path: "go/internal/guards/role.go"}}
	cases := []struct {
		name     string
		decision string
		scope    []string
	}{
		{"another card of the lane stays committed", `{"top_n":[{"id":"protected-part"},{"id":"lane-part"}]}`, []string{"pin"}},
		{"a multi-item lane cannot tell which item the card was", `{"top_n":[{"id":"protected-part"}]}`, []string{"pin", "other-pin"}},
		{"a sequential cycle has no pin", `{"top_n":[{"id":"protected-part"}]}`, nil},
		{"triage already answered the pin", `{"top_n":[{"id":"protected-part"}],"dropped":[{"id":"pin","reason":"premise already landed"}]}`, []string{"pin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeDecision(t, t.TempDir(), tc.decision)

			if err := routeProtectedCards(path, card, tc.scope); err != nil {
				t.Fatal(err)
			}

			if esc := escalationsOf(t, path); len(esc) != 1 || esc["protected-part"] == "" {
				t.Errorf("only the routed card is escalated: %v", esc)
			}
		})
	}
}

func TestRouteProtectedCards_APinThatIsTheRoutedCardIsEscalatedOnce(t *testing.T) {
	t.Parallel()
	path := writeDecision(t, t.TempDir(), `{"top_n":[{"id":"pin"}]}`)

	if err := routeProtectedCards(path, []protectedCard{{ID: "pin", Path: "go/internal/guards/role.go"}}, []string{"pin"}); err != nil {
		t.Fatal(err)
	}

	if esc := readDecision(t, path)["escalate_block"].([]any); len(esc) != 1 {
		t.Errorf("the pin is escalated once: %v", esc)
	}
}

func TestTriageClassify_AnAliasCardOnAProtectedSurfaceAnswersTheLanesPin(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, committedset.LanePinFile), []byte(`{"todo_ids":["goal-text-has-no-selection-authority"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDecision(t, ws, `{"cycle":1757,"top_n":[{"id":"goal-text-selection-authority"}],"escalate_block":[]}`)
	artifact := "## top_n\n- goal-text-selection-authority: selection authority in PlanFn — priority=H, " +
		"files={go/internal/goalselect/goalselect.go;go/internal/loopwave/loopwave.go}, source={scout}\n"
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}

	verdict, _, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s; the host's route is a disposition, never the cycle's FAIL", verdict)
	}
	var pinReason string
	for _, d := range committedset.Dispositions(ws) {
		if d.ID == "goal-text-has-no-selection-authority" {
			pinReason = d.Reason
		}
	}
	if !strings.HasPrefix(pinReason, "protected-surface: go/internal/loopwave/loopwave.go") {
		t.Errorf("the lane's pinned id is answered with the console route (cycle 1757 answered only the alias): %q", pinReason)
	}
	signals, err := router.Digest(ws, []string{string(core.PhaseTriage)})
	if err != nil || !signals.HasEmptyTriageCommitment() {
		t.Errorf("the lane commits nothing, so the host ends it as planned no-work: err=%v signals=%+v", err, signals.Triage)
	}
}

func TestWithBoundItem_TheItemJoinsOnceWithTheFirstRoutedCard(t *testing.T) {
	t.Parallel()
	cards := make([]protectedCard, 2, 4)
	cards[0] = protectedCard{ID: "guard-part", Path: "go/internal/guards/role.go"}
	cards[1] = protectedCard{ID: "loop-part", Path: "go/internal/loopwave/loopwave.go"}

	routed := withBoundItem(cards, []string{"pin"}, []byte(`{"top_n":[]}`))

	if len(routed) != 3 || routed[2] != (protectedCard{ID: "pin", Path: "go/internal/guards/role.go", Via: "guard-part"}) {
		t.Errorf("routed = %+v; the item joins after the cards with the first card's path and id", routed)
	}
	if spare := cards[:3][2]; spare != (protectedCard{}) {
		t.Errorf("the caller's cards are never written through their spare capacity: %+v", spare)
	}
	if again := withBoundItem(cards, []string{"loop-part"}, []byte(`{"top_n":[]}`)); len(again) != 2 {
		t.Errorf("a pin that is already a routed card is not added twice: %+v", again)
	}
}

func TestTriageClassify_APinTriageDeferredStaysOwedWork(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, committedset.LanePinFile), []byte(`{"todo_ids":["pin"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeDecision(t, ws, `{"top_n":[{"id":"alias"}],"deferred":[{"id":"pin"}],"escalate_block":[]}`)
	artifact := "## top_n\n- alias: part of the item — priority=H, files={go/internal/guards/role.go}, source={scout}\n"

	verdict, _, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s", verdict)
	}
	if esc := escalationsOf(t, path); len(esc) != 1 || esc["alias"] == "" {
		t.Errorf("a deferred pin is still owed work, never routed for another card: %v", esc)
	}
}
