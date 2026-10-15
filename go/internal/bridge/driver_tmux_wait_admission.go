package bridge

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

func (w replWaiter) admitPrompt(state *replWaitState) int {
	pane, captureErr := w.deps.Tmux.CapturePane(w.ctx, w.launch.session, w.launch.bootScrollback)
	if captureErr != nil {
		fmt.Fprintf(w.deps.Stderr, "%s submit-verify: prompt NOT verified — baseline capture failed, input-line state unknown: %v\n", w.prefix, captureErr)
	}
	state.result.recordTokens(pane)
	state.result.lastGoodPane = pane

	if w.launch.guardDeadShell && paneLooksLikeShellSpill(pane) {
		if shellCmd, isShell := paneShellProcess(w.ctx, w.deps.Tmux, w.launch.session); isShell {
			fmt.Fprintf(w.deps.Stderr, "%s FAIL: prompt spilled into a dead shell (%s) after paste — CLI process gone (cycle-274 class)\n", w.prefix, shellCmd)
			reportDeadShellAfterPaste(w.deps, shellCmd)
			return ExitREPLBootTimeout
		}
	}

	codexPasteEcho := ""
	if w.launch.name == "codex-tmux" {
		codexPasteEcho = "[Pasted Content "
	}
	outcome := verifySubmitted(w.ctx, w.deps, w.launch, w.prefix, "prompt", pane,
		promptSubmitEcho(w.resolvedPrompt), firstNonEmptyLine(w.resolvedPrompt), tmuxPastePlaceholderEcho, codexPasteEcho)
	recordSubmitVerify(w.recorder, w.phaseName, w.cfg.Cycle, "prompt", outcome, w.paste)
	if outcome.Result != interaction.ResultSubmitWedged {
		return ExitOK
	}

	if path, found := artifactLocate(w.cfg); found {
		if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() && !w.dispatchBase.matches(path, fi) {
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
