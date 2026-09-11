package bridge

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// admitPrompt verifies the prompt after dispatch and returns ExitOK when the
// completion loop may begin. It deliberately reuses one pane capture for token
// telemetry, dead-shell detection, and submission verification so admission
// cannot consume different fixture frames or disagree about what was visible.
func (w replWaiter) admitPrompt(state *replWaitState) int {
	pane, captureErr := w.deps.Tmux.CapturePane(w.ctx, w.launch.session, w.launch.bootScrollback)
	if captureErr != nil {
		// This pane is submit-verify's only observation on the clean path. A
		// failed capture leaves the input-line state unknown and must be loud.
		fmt.Fprintf(w.deps.Stderr, "%s submit-verify: prompt NOT verified — baseline capture failed, input-line state unknown: %v\n", w.prefix, captureErr)
	}
	state.result.recordTokens(pane)
	state.result.lastGoodPane = pane

	// A shell continuation after paste means the CLI process is gone. Check it
	// before submission verification so a dead shell never receives more Enters.
	if w.launch.guardDeadShell && paneLooksLikeShellSpill(pane) {
		if shellCmd, isShell := paneShellProcess(w.ctx, w.deps.Tmux, w.launch.session); isShell {
			fmt.Fprintf(w.deps.Stderr, "%s FAIL: prompt spilled into a dead shell (%s) after paste — CLI process gone (cycle-274 class)\n", w.prefix, shellCmd)
			return ExitREPLBootTimeout
		}
	}

	codexPasteEcho := ""
	if w.launch.name == "codex-tmux" {
		codexPasteEcho = "[Pasted Content "
	}
	outcome := verifySubmitted(w.ctx, w.deps, w.launch, w.prefix, "prompt", pane,
		promptSubmitEcho(w.resolvedPrompt), firstNonEmptyLine(w.resolvedPrompt), tmuxPastePlaceholderEcho, codexPasteEcho)
	// Preserve the pane heuristic in the interaction ledger even when stronger
	// disk evidence below recovers it. Consumers use the phase result as the
	// disposition and the ledger outcome as diagnostic evidence.
	recordSubmitVerify(w.recorder, w.phaseName, w.cfg.Cycle, "prompt", outcome)
	if outcome.Result != interaction.ResultSubmitWedged {
		return ExitOK
	}

	// Pane echo is heuristic evidence. A new regular artifact on disk proves the
	// prompt landed even when the input line still looks parked; the completion
	// detector retains responsibility for its stability window.
	if path, found := artifactLocate(w.cfg); found {
		// Match regularFileNonEmpty's no-symlink contract and reject an artifact
		// that is identical to the pre-dispatch baseline.
		if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() && !w.artifactBase.matches(path, fi) {
			fmt.Fprintf(w.deps.Stderr, "%s submit-verify: pane looks parked but a post-dispatch deliverable is already on disk — submission evidently landed; continuing the normal wait\n", w.prefix)
			return ExitOK
		}
	}

	state.submitWedged = true
	state.lastVerdict = ReviewVerdict{
		Action: ReviewPause,
		Reason: fmt.Sprintf("prompt %s (resends=%d)", outcome.Result, outcome.Resends),
	}
	w.writeArtifactTimeoutMarker(state, false)
	return ExitArtifactTimeout
}
