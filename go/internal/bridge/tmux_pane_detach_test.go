package bridge

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type detachFailTmux struct{ *fakeTmux }

func (d detachFailTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if keys == paneTmuxDetachLine {
		return errors.New("pane gone")
	}
	return d.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestTmuxBoot_AFailedDetachSendStopsTheBootBeforeTheLaunch(t *testing.T) {
	tmux := &fakeTmux{}
	var stderr bytes.Buffer
	deps := Deps{
		Tmux:   detachFailTmux{tmux},
		Stderr: &stderr,
		Sleep:  func(time.Duration) {},
		Now:    time.Now,
		Env:    map[string]string{envSandboxMode: "off"},
	}.withDefaults()
	cfg := &Config{Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "build"}
	lp := tmuxLaunch{name: "itest-tmux", session: "detach-fail", launchCmd: "fake-cli --go", promptMarker: "READY"}

	release, code, err := bootTmuxREPL(context.Background(), cfg, deps, lp, replPreparation{prefix: "[t]", workingDir: cfg.Worktree}, nil)

	if err == nil || code != ExitBadFlags || release != nil {
		t.Fatalf("boot = (release=%v, %d, %v), want a refused boot with ExitBadFlags", release != nil, code, err)
	}
	if slices.Contains(tmux.sentKeys, lp.launchCmd) {
		t.Fatalf("the CLI must not launch in a pane still attached to the run server; keys sent: %q", tmux.sentKeys)
	}
	if !strings.Contains(stderr.String(), "detach the pane shell from the run tmux server") {
		t.Fatalf("stderr must name the failed detach; got %q", stderr.String())
	}
}
