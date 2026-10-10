package bridge

import "fmt"

const paneLossConfirmTicks = 2

func (w replWaiter) paneLost(state *replWaitState, captureOK bool) bool {
	if captureOK || w.ctx.Err() != nil || w.deps.Tmux.HasSession(w.ctx, w.launch.session) {
		state.paneLossTicks = 0
		return false
	}
	state.paneLossTicks++
	if state.paneLossTicks < paneLossConfirmTicks {
		return false
	}
	state.paneLost = true
	state.lastVerdict.Reason = fmt.Sprintf("tmux session %s is gone: capture-pane failed and has-session found no session on %d consecutive ticks", w.launch.session, paneLossConfirmTicks)
	fmt.Fprintf(w.deps.Stderr, "%s PANE LOST: %s — ending the dispatch\n", w.prefix, state.lastVerdict.Reason)
	return true
}
