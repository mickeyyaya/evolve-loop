package bridge

import "testing"

func TestJoinedPaneCapturer_TheProductionTmuxJoinsWrappedLines(t *testing.T) {
	t.Parallel()
	var ctl TmuxController = execTmux{}
	if _, ok := ctl.(JoinedPaneCapturer); !ok {
		t.Fatal("the production tmux controller is not a JoinedPaneCapturer, so the cloud driver reads the wrapped pane")
	}
	if got := capturePaneArgs("s", 0, true); got[2] != "-J" {
		t.Fatalf("joined capture args = %v, want -J after capture-pane -p", got)
	}
}
