package bridge

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

func TestRunTmuxREPL_CutsOverInboxBeforeOpeningLiveChannel(t *testing.T) {
	raw, err := os.ReadFile("driver_tmux_repl.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	cursorAt := strings.Index(source, "newReplInboxCursor(cfg)")
	channelAt := strings.Index(source, "openReplLiveChannel(cfg, deps, lp)")
	if cursorAt < 0 || channelAt < 0 {
		t.Fatalf("runTmuxREPL must initialize both inbox cursor and live channel")
	}
	if cursorAt > channelAt {
		t.Fatal("inbox cursor cutover must precede live-channel file creation so an envelope cannot arrive in an unobserved interval")
	}
}

func TestReplInboxCursorRetainsEnvelopeAppendedAfterLiveChannelOpen(t *testing.T) {
	workspace := t.TempDir()
	cfg := &Config{Workspace: workspace, Agent: "build"}
	appendEnvelope := func(body string) {
		t.Helper()
		if err := inbox.Append(workspace, cfg.Agent, inbox.Envelope{Body: body}, time.Now); err != nil {
			t.Fatal(err)
		}
	}

	appendEnvelope("old backlog")
	cursor := newReplInboxCursor(cfg)
	deps := covDeps()
	deps.Stderr = io.Discard
	deps.RecoveryStage = "enforce"
	channel := openReplLiveChannel(cfg, deps, tmuxLaunch{name: "claude-tmux"})
	defer channel.close()
	appendEnvelope("arrived after cutover")

	envelopes, err := cursor.Drain()
	if err != nil {
		t.Fatal(err)
	}
	if len(envelopes) != 1 || envelopes[0].Body != "arrived after cutover" {
		t.Fatalf("drained envelopes = %+v, want only the post-cutover envelope", envelopes)
	}
}
