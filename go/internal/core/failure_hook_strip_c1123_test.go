package core

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func TestC1123_AgentDiffQuotedSignatureStillReachesAdvisor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build",
		"⏺ Editing go/internal/recovery/detector.go\n"+
			"    72 +\t\t\tSubstr: \"There's an issue with the selected model\",\n"+
			"    73 +\t\t\tCause:  CauseModelInvalid,\n"+
			"⚠ some never-seen fatal pane state, definitely novel")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{
		Cause: "dead_shell", PaneSubstr: "never-seen fatal pane state", Justification: "novel wedge under an edit buffer"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 1123, ws, root, PhaseBuild, wrapTimeout(), nil)

	if fa.calls != 1 {
		t.Fatalf("advisor consulted %d time(s), want 1 — the hook matched the registry against the agent's OWN diff content and short-circuited as 'already classified', so a genuinely novel fatal pane teaches the registry nothing (ADR-0044 C3 learning loop disabled by the agent's own text)", fa.calls)
	}
}

func TestC1123_BareDiffPrefixedSignatureStillReachesAdvisor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build",
		"--- a/go/internal/recovery/detector.go\n"+
			"+++ b/go/internal/recovery/detector.go\n"+
			"+\tNote: \"Update ran successfully! Please restart — codex self-upgrade\",\n"+
			"⚠ an entirely novel wedge nobody has classified yet")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{
		Cause: "dead_shell", PaneSubstr: "entirely novel wedge nobody has classified", Justification: "novel"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 1123, ws, root, PhaseBuild, wrapTimeout(), nil)

	if fa.calls != 1 {
		t.Fatalf("advisor consulted %d time(s), want 1 — a bare unified-diff content line quoting a seed still short-circuits the C3 path", fa.calls)
	}
}

func TestC1123_RealChromeStillSkipsAdvisor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "retro", "⏺ There's an issue with the selected model (auto). It may not exist.")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{
		Cause: "model_invalid", PaneSubstr: "irrelevant long substring", Justification: "j"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 1123, ws, root, PhaseRetro, wrapTimeout(), nil)

	if fa.calls != 0 {
		t.Fatalf("advisor consulted %d time(s), want 0 — a registry-classified pane must never reach the LLM (deterministic-first, Rule 5); the strip is eating CLI chrome", fa.calls)
	}
}

func TestC1123_AnchoredSeedUnderDiffLineStillSkipsAdvisor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build", "    41 +\tpane := captureTail(sess)\nquote>")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{
		Cause: "dead_shell", PaneSubstr: "some long enough substring", Justification: "j"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 1123, ws, root, PhaseBuild, wrapTimeout(), nil)

	if fa.calls != 0 {
		t.Fatalf("advisor consulted %d time(s), want 0 — a genuinely wedged shell stopped being classified after stripping: the strip DELETED the diff line instead of blanking it, collapsing the \"\\nquote>\" anchor (D1, cycle-274)", fa.calls)
	}
}
