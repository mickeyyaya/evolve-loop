package triage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

const protectedCardsBody = "- innocent-task: unrelated fix — priority=M, files={go/internal/foo/foo.go}, source=scout\n" +
	"- binaryguard-bypass: edit the binary guard — priority=H, files={go/internal/binaryguard/guard.go}, source=scout\n" +
	"- role-gate-fix: touch the role gate — priority=H, files=go/internal/guards/role.go;go/internal/foo/foo.go, source=scout\n"

func TestProtectedTopNCards_NamesEveryOffendingCard(t *testing.T) {
	t.Parallel()
	if !guards.IsProtectedSurface("go/internal/binaryguard/guard.go") || !guards.IsProtectedSurface("go/internal/guards/role.go") {
		t.Fatal("pin moved: the manifest no longer protects binaryguard/ or guards/")
	}

	cards := protectedTopNCards(protectedCardsBody, nil)

	if len(cards) != 2 || cards[0].ID != "binaryguard-bypass" || cards[0].Path != "go/internal/binaryguard/guard.go" ||
		cards[1].ID != "role-gate-fix" || cards[1].Path != "go/internal/guards/role.go" {
		t.Fatalf("cards = %+v; want both offending cards in report order, never the innocent one", cards)
	}
}

func writeDecision(t *testing.T, ws, body string) string {
	t.Helper()
	path := filepath.Join(ws, "triage-decision.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readDecision(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(body, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestRouteProtectedCards_MovesTheCardOutOfTopNIntoEscalateBlock(t *testing.T) {
	t.Parallel()
	path := writeDecision(t, t.TempDir(), `{"cycle":1714,"top_n":[{"id":"binaryguard-bypass","files":["go/internal/binaryguard/guard.go"]},{"id":"innocent-task","files":["go/internal/foo/foo.go"]}],"deferred":[{"id":"later"}],"escalate_block":[{"task_id":"earlier","reason":"fail_count 3"}],"phase_skip":[]}`)
	cards := []protectedCard{{ID: "binaryguard-bypass", Path: "go/internal/binaryguard/guard.go"}}

	if err := routeProtectedCards(path, cards, nil); err != nil {
		t.Fatalf("routeProtectedCards: %v", err)
	}

	d := readDecision(t, path)
	topN := d["top_n"].([]any)
	if len(topN) != 1 || topN[0].(map[string]any)["id"] != "innocent-task" {
		t.Errorf("top_n keeps only the innocent card: %v", topN)
	}
	esc := d["escalate_block"].([]any)
	if len(esc) != 2 || esc[0].(map[string]any)["task_id"] != "earlier" {
		t.Fatalf("the earlier escalation stands first: %v", esc)
	}
	added := esc[1].(map[string]any)
	if added["task_id"] != "binaryguard-bypass" || !strings.HasPrefix(added["reason"].(string), "protected-surface: go/internal/binaryguard/guard.go") {
		t.Errorf("the routed card is an escalation with the console-route reason: %v", added)
	}
	if d["cycle"] != float64(1714) || len(d["deferred"].([]any)) != 1 || d["phase_skip"] == nil {
		t.Errorf("every other key is kept: %v", d)
	}

	if err := routeProtectedCards(path, cards, nil); err != nil {
		t.Fatal(err)
	}
	if again := readDecision(t, path); len(again["escalate_block"].([]any)) != 2 {
		t.Errorf("a second routing of the same card adds nothing: %v", again["escalate_block"])
	}
}

func TestRouteProtectedCards_RefusesAnAbsentOrMalformedDecision(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cards := []protectedCard{{ID: "x", Path: "go/internal/guards/role.go"}}
	if err := routeProtectedCards(filepath.Join(ws, "triage-decision.json"), cards, nil); err == nil {
		t.Error("no decision to record the route in is a fault")
	}
	if err := routeProtectedCards(writeDecision(t, ws, "not json"), cards, nil); err == nil {
		t.Error("a decision that cannot be read is a fault")
	}
}

func TestTriageClassify_ProtectedSurfaceCardIsRoutedNotRefused(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := writeDecision(t, ws, `{"cycle":1714,"top_n":[{"id":"innocent-task"},{"id":"binaryguard-bypass"},{"id":"role-gate-fix"}]}`)

	verdict, diags, next := hooks{}.Classify("## top_n\n"+protectedCardsBody, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS || next != string(core.PhaseTDD) {
		t.Fatalf("verdict = %s next = %s; a routed card is a host disposition, never the cycle's FAIL", verdict, next)
	}
	var warned []string
	for _, d := range diags {
		if d.Code == cyclestate.DiagCodeTriageProtectedSurface {
			if d.Severity != "warning" || d.Subject == "" || !strings.Contains(d.Message, "routed to the console") {
				t.Errorf("each routed card is a warning naming its subject and the route: %+v", d)
			}
			warned = append(warned, d.Subject)
		}
	}
	if strings.Join(warned, ",") != "binaryguard-bypass,role-gate-fix" {
		t.Errorf("warnings = %v", warned)
	}
	d := readDecision(t, path)
	if topN := d["top_n"].([]any); len(topN) != 1 || topN[0].(map[string]any)["id"] != "innocent-task" {
		t.Errorf("the decision commits only the innocent card: %v", topN)
	}
	if esc := d["escalate_block"].([]any); len(esc) != 2 {
		t.Errorf("both routed cards are escalations: %v", esc)
	}
}

func TestTriageClassify_ProtectedSurfaceCardWithNoDecisionFailsClosed(t *testing.T) {
	t.Parallel()
	verdict, diags, _ := hooks{}.Classify("## top_n\n"+protectedCardsBody, core.PhaseRequest{Workspace: t.TempDir()}, core.BridgeResponse{})

	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s; a route that cannot be recorded must not let the card reach the spine", verdict)
	}
	if codes := errorCodesOf(diags); len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageProtectedSurface {
		t.Errorf("codes = %v", codes)
	}
}

func TestTriageComposePrompt_NamesTheProtectedSurfaces(t *testing.T) {
	root := t.TempDir()
	if out := (hooks{}).ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root}); strings.Contains(out, "protected_surfaces") {
		t.Fatalf("an empty inbox renders nothing (the byte-identity pin):\n%s", out)
	}
	writeInboxItem(t, root, "a.json", `{"id":"lane-work","weight":0.9}`)

	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})

	if !strings.Contains(out, "- protected_surfaces:") || !strings.Contains(out, "/go/internal/guards/") || !strings.Contains(out, "`protected-surface: <path>`") {
		t.Fatalf("the prompt names the surfaces and the drop reason:\n%s", out)
	}
}

func TestRouteProtectedCards_RefusesACardWithoutAnIDAndKeepsIDLessEntries(t *testing.T) {
	t.Parallel()
	path := writeDecision(t, t.TempDir(), `{"top_n":[{"id":"innocent-task"},{"files":["go/internal/foo/foo.go"]},{"id":"binaryguard-bypass"}]}`)

	if err := routeProtectedCards(path, []protectedCard{{ID: "", Path: "go/internal/guards/role.go"}}, nil); err == nil {
		t.Fatal("a card with no id cannot be routed by id: a fault, never a sweep of every id-less entry")
	}
	if d := readDecision(t, path); len(d["top_n"].([]any)) != 3 {
		t.Errorf("a refused route writes nothing: %v", d["top_n"])
	}

	if err := routeProtectedCards(path, []protectedCard{{ID: "binaryguard-bypass", Path: "go/internal/binaryguard/guard.go"}}, nil); err != nil {
		t.Fatal(err)
	}
	d := readDecision(t, path)
	if topN := d["top_n"].([]any); len(topN) != 2 || topN[0].(map[string]any)["id"] != "innocent-task" || topN[1].(map[string]any)["id"] != nil {
		t.Errorf("an entry with no readable id is never matched by a routed id: %v", topN)
	}
}

func TestTriageClassify_ProtectedCardWithoutAnIDFailsClosed(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeDecision(t, ws, `{"top_n":[{"id":"innocent-task"}]}`)
	artifact := "## top_n\n- innocent-task: fine — priority=M, files={go/internal/foo/foo.go}, source=scout\n" +
		"- rewrite the role gate — priority=H, files={go/internal/guards/role.go}, source=scout\n"

	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s; a protected card the host cannot name must not reach the spine", verdict)
	}
	if codes := errorCodesOf(diags); len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageProtectedSurface || !diagsContain(diags, "go/internal/guards/role.go") {
		t.Errorf("the refusal carries the code and the path: %+v", diags)
	}
}

func TestRouteProtectedCards_RefusesACardTheDecisionDoesNotCommit(t *testing.T) {
	t.Parallel()
	path := writeDecision(t, t.TempDir(), `{"top_n":[{"id":"other"}],"escalate_block":[]}`)
	before, _ := os.ReadFile(path)

	err := routeProtectedCards(path, []protectedCard{{ID: "ghost", Path: "go/internal/guards/role.go"}}, nil)

	if err == nil {
		t.Fatal("a card the decision never committed cannot be moved out of it: the report and its decision disagree, a fault")
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Errorf("a refused route writes nothing: %s", after)
	}
}
