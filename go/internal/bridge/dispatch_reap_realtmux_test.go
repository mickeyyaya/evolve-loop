//go:build !notmux

package bridge

import (
	"context"
	"strings"
	"testing"
)

func TestExecTmuxPanePIDs_AMissingSessionIsAnError(t *testing.T) {
	t.Parallel()
	got, err := execTmux{}.PanePIDs(context.Background(), "evolve-cover-no-such-session-"+thisDispatch[:5])

	if err == nil || !strings.HasPrefix(err.Error(), "tmux list-panes: ") || got != nil {
		t.Errorf("PanePIDs = %v, %v, want no pids and a tmux list-panes error", got, err)
	}
}
