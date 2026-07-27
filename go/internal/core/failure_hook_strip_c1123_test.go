package core

// failure_hook_strip_c1123_test.go — cycle-1123 RED tests for
// `fatalpane-strip-agent-content` at the SECOND raw-Detect call site:
// adviseOnUnclassifiedFailure (failure_hook.go:87).
//
// The hook's deterministic-first short-circuit — "pane already classified, skip
// the advisor" — reads report.FinalPane RAW. So a phase that aborted on a
// GENUINELY NOVEL wedge, whose final pane happens to carry agent-authored diff
// content quoting a seeded signature (an agent editing recovery/detector.go,
// a builder writing a fatal-pane regression fixture, this very cycle's work),
// is misread as "known". The C3 advise→promote path never runs, nothing is
// learned, and the next occurrence burns the maxExtends backstop again.
//
// The fix is the SAME stripper the bridge seam already uses, lifted to the
// registry's owning package and called here before Detect:
//
//	recovery.StripAgentContent(report.FinalPane, "", det.Signatures())
//
// The empty injectedPrompt is DELIBERATE, not an omission: for a fatal-pane
// scan the echo half is neutered by the protect-list anyway (D2 — a line
// carrying a seeded signature is never echo-stripped), so plumbing the phase
// prompt into the hook would add I/O and zero behaviour. The diff half is what
// closes this class. Passing "" is the documented fail-open value.
//
// DO NOT MODIFY THESE TESTS to make them pass. The two negative tests below
// (real chrome, and a real anchored seed under agent diff content) are the
// guard against the lazy over-fix — stripping so eagerly that the
// deterministic-first path stops working at all.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// TestC1123_AgentDiffQuotedSignatureStillReachesAdvisor is the primary RED: the
// pane's ONLY seeded signature is agent-authored diff content, so the pane is
// unclassified and the advisor must be consulted.
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

// TestC1123_BareDiffPrefixedSignatureStillReachesAdvisor is the EDGE form: the
// unnumbered unified-diff line from lingering patch scrollback, rather than the
// editor's numbered view. Both are agent authorship by construction.
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

// TestC1123_RealChromeStillSkipsAdvisor is the NEGATIVE half: an over-eager
// strip that also eats CLI chrome would break deterministic-first (every known
// pane would pay for an LLM call and re-promote a signature already seeded).
func TestC1123_RealChromeStillSkipsAdvisor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	// The real cycle-262 claude pane — no diff prefix, genuine chrome.
	writeEscalation(t, ws, "retro", "⏺ There's an issue with the selected model (auto). It may not exist.")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{
		Cause: "model_invalid", PaneSubstr: "irrelevant long substring", Justification: "j"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 1123, ws, root, PhaseRetro, wrapTimeout(), nil)

	if fa.calls != 0 {
		t.Fatalf("advisor consulted %d time(s), want 0 — a registry-classified pane must never reach the LLM (deterministic-first, Rule 5); the strip is eating CLI chrome", fa.calls)
	}
}

// TestC1123_AnchoredSeedUnderDiffLineStillSkipsAdvisor is D1 at THIS call site:
// the pane carries a genuine zsh continuation prompt (a real dead shell)
// directly below agent diff content. Blanked in place the anchor survives and
// the pane stays classified; deleted, the survivor loses its leading "\n", the
// dead shell reads as novel, and the hook spends a 3-minute LLM consultation
// re-learning a signature seeded since cycle-274.
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
