package bridge

import (
	"context"
	"strings"
	"testing"
)

func TestExecTmuxCapture_AMissingSessionIsACapturePaneError(t *testing.T) {
	t.Parallel()
	session := "evolve-cover-no-such-capture-" + thisDispatch[:5]
	captures := map[string]func() (string, error){
		"plain":  func() (string, error) { return execTmux{}.CapturePane(context.Background(), session, 0) },
		"joined": func() (string, error) { return execTmux{}.CapturePaneJoined(context.Background(), session, 200) },
	}
	for name, capture := range captures {
		_, err := capture()
		if err == nil || !strings.HasPrefix(err.Error(), "tmux capture-pane: ") || !strings.Contains(err.Error(), session) {
			t.Errorf("%s capture error = %v, want a tmux capture-pane error that names %s", name, err, session)
		}
	}
}
