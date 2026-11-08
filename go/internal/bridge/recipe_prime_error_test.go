package bridge

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

type detachRefusingTmux struct {
	*fakeTmux
}

func (f detachRefusingTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if keys == paneTmuxDetachLine {
		return errors.New("tmux refused the keys")
	}
	return f.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestRecipeDriver_EnsureSession_APaneShellThatCannotBePrimedNeverLaunches(t *testing.T) {
	t.Parallel()
	tx := &fakeTmux{paneSeq: []string{"❯"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	d.deps.Tmux = detachRefusingTmux{tx}

	err := d.EnsureSession(context.Background())

	if err == nil || !strings.Contains(err.Error(), "prime pane shell") || !strings.Contains(err.Error(), "tmux refused the keys") {
		t.Fatalf("err = %v, want the prime failure with its cause", err)
	}
	if slices.Contains(tx.sentKeys, d.launchCmd) {
		t.Fatalf("the launch command was sent into a shell that was not primed: %v", tx.sentKeys)
	}
}
