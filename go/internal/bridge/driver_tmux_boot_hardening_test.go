package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPrimePaneShell_QuotesTheWorkingDirectoryInTheCd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		dir  string
		want string
	}{
		{name: "space quote dollar and semicolon", dir: "/tmp/my dir/it's $HOME; touch pwned", want: `cd '/tmp/my dir/it'\''s $HOME; touch pwned'`},
		{name: "a newline stays inside the quotes", dir: "/tmp/a\ntouch pwned", want: "cd '/tmp/a\ntouch pwned'"},
		{name: "a backtick stays inside the quotes", dir: "/tmp/`id`", want: "cd '/tmp/`id`'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmux := &fakeTmux{}
			deps := Deps{Tmux: tmux, Stderr: &bytes.Buffer{}}

			if err := primePaneShell(context.Background(), deps, &Config{}, "s", tc.dir); err != nil {
				t.Fatal(err)
			}

			if !slices.Contains(tmux.sentKeys, tc.want) {
				t.Fatalf("pane keys = %q, want the exact quoted line %q", tmux.sentKeys, tc.want)
			}
		})
	}
}

type nudgeSendFailTmux struct {
	*fakeTmux
	trigger string
}

func (n nudgeSendFailTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if strings.Contains(keys, n.trigger) {
		return errors.New("pane gone")
	}
	return n.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestDeliverArtifactNudge_AFailedSendIsNotSubmitVerified(t *testing.T) {
	t.Parallel()
	waiter, state, baseTmux := newDispositionFixture(t, ReviewPause)
	waiter.deps.Tmux = nudgeSendFailTmux{fakeTmux: baseTmux, trigger: waiter.cfg.Artifact}

	got := waiter.applyCheckpointDisposition(state, 10)

	if got != checkpointStopWaiting {
		t.Fatalf("disposition = %v, want checkpointStopWaiting for a nudge that never reached the pane", got)
	}
	if state.nudgeSent || state.nudgeEvent != nil {
		t.Errorf("a failed send armed the nudge outcome: sent=%v event=%+v", state.nudgeSent, state.nudgeEvent)
	}
	if !strings.Contains(state.lastVerdict.Reason, "pane gone") {
		t.Errorf("verdict reason = %q, want the send error", state.lastVerdict.Reason)
	}
	outcomes := waiter.recorder.Outcomes()
	if len(outcomes) != 1 || outcomes[0].Result != interaction.ResultNotVerified {
		t.Fatalf("recorded outcomes = %+v, want exactly one %s", outcomes, interaction.ResultNotVerified)
	}
}

func TestHandleTickInteractions_ADrainErrorIsACodedWarn(t *testing.T) {
	t.Parallel()
	waiter, state, _ := newDispositionFixture(t, ReviewPause)
	blocker := filepath.Join(waiter.cfg.Workspace, ".bridge-inbox")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	waiter.cursor = inbox.NewCursor(waiter.cfg.Workspace, waiter.cfg.Agent)
	center := signalcenter.New()
	waiter.deps.Signals = center
	var stderr bytes.Buffer
	waiter.deps.Stderr = &stderr
	waiter.responder = &autoResponder{}
	waiter.channel = openReplLiveChannel(waiter.cfg, waiter.deps, tmuxLaunch{name: "claude-tmux"})
	defer waiter.channel.close()

	waiter.handleTickInteractions(state, 10, "", false)
	waiter.handleTickInteractions(state, 11, "", false)
	center.Flush()

	if n := countCode(center.Recent(), CodeInboxDrainFailed); n != 1 {
		t.Fatalf("%s emitted %d times over two failing ticks, want once per wait", CodeInboxDrainFailed, n)
	}
	idx := slices.IndexFunc(center.Recent(), func(e signalcenter.Event) bool { return e.Code == CodeInboxDrainFailed })
	if idx < 0 {
		t.Fatalf("events = %+v, want %s", center.Recent(), CodeInboxDrainFailed)
	}
	if center.Recent()[idx].Severity != signalcenter.SeverityWarn {
		t.Errorf("severity = %v, want WARN", center.Recent()[idx].Severity)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("stderr = %q, want a WARN line", stderr.String())
	}
}

func countCode(events []signalcenter.Event, code signalcenter.Code) int {
	n := 0
	for _, e := range events {
		if e.Code == code {
			n++
		}
	}
	return n
}
