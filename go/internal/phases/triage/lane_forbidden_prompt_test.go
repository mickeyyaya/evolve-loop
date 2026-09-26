package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func excludedIDs(prompt string) string {
	i := strings.Index(prompt, "console_routed_excluded")
	if i < 0 {
		return ""
	}
	return prompt[i:]
}

func TestTriageComposePrompt_ExcludesWhatTheInjectedLanePredicateForbids(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "a.json", `{"id":"lane-work","weight":0.9}`)
	writeInboxItem(t, root, "b.json", `{"id":"needs-a-profile","weight":0.95,"files":[".evolve/profiles/historian.json (new)"]}`)
	sandboxDenied := func(p string) bool { return strings.HasPrefix(p, ".evolve/profiles/") }

	out := hooks{forbidden: sandboxDenied}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root})
	if !strings.Contains(excludedIDs(out), "needs-a-profile") {
		t.Fatalf("an item the lane predicate forbids must be excluded loudly:\n%s", out)
	}
	if strings.Contains(excludedIDs(out), "lane-work") {
		t.Fatalf("an allowed item must stay selectable:\n%s", out)
	}
}

func TestTriageComposePrompt_WithoutAnInjectedPredicateJudgesProtectedSurfaceOnly(t *testing.T) {
	root := t.TempDir()
	writeInboxItem(t, root, "b.json", `{"id":"needs-a-profile","weight":0.95,"files":[".evolve/profiles/historian.json (new)"]}`)
	if out := (hooks{}).ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root}); strings.Contains(excludedIDs(out), "needs-a-profile") {
		t.Fatalf("with no injected predicate, only protected surface routes to the console:\n%s", out)
	}
}

func TestHooksFor_ThreadsTheLanePredicate(t *testing.T) {
	forbid := func(string) bool { return true }
	if h := hooksFor(Config{LaneForbidden: forbid}); h.forbidden == nil || !h.forbidden("any") {
		t.Fatal("Config.LaneForbidden must reach the hooks")
	}
	if h := hooksFor(Config{}); h.forbidden != nil {
		t.Fatal("an unset predicate stays nil so the protected-surface default applies")
	}
}

func TestTriageClassify_RefusesATopNCardNamingAPathTheLanePredicateForbids(t *testing.T) {
	artifact := "## top_n\n- historian: add the historian profile — priority=H, files={.evolve/profiles/historian.json}, source=scout\n"
	sandboxDenied := func(p string) bool { return strings.HasPrefix(p, ".evolve/profiles/") }
	verdict, diags, _ := hooks{forbidden: sandboxDenied}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})
	if codes := errorCodesOf(diags); verdict != core.VerdictFAIL || len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageProtectedSurface {
		t.Fatalf("a card naming a lane-forbidden path must be refused: verdict=%s diags=%+v", verdict, diags)
	}
	_, diags, _ = hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})
	for _, code := range errorCodesOf(diags) {
		if code == cyclestate.DiagCodeTriageProtectedSurface {
			t.Fatal("with no injected predicate the breaker judges the manifest only")
		}
	}
}
