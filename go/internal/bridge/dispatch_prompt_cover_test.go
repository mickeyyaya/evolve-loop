package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type seedFailTmux struct{ *fakeTmux }

func (s seedFailTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if keys != "" {
		return errors.New("pane gone")
	}
	return s.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestDispatchTmuxPrompt_AFailedSeedLineWarnsAndThePromptIsStillDelivered(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prompt := filepath.Join(ws, "resolved-prompt.txt")
	if err := os.WriteFile(prompt, []byte("do the task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tx := &fakeTmux{paneSeq: []string{"❯"}}
	var stderr bytes.Buffer
	deps := Deps{Tmux: seedFailTmux{tx}, Stderr: &stderr, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Now: time.Now}
	cfg := &Config{Workspace: ws, Realization: Realization{REPLInput: []string{"/model opus"}}}

	_, _, rc, err := dispatchTmuxPrompt(context.Background(), cfg, deps, tmuxLaunch{name: "claude-tmux", session: "s1"},
		replPreparation{prefix: "[claude-tmux]", resolvedPromptFile: prompt}, false, "build")

	if err != nil || rc != ExitOK {
		t.Fatalf("dispatchTmuxPrompt rc=%d err=%v, want the prompt delivered after a failed seed\n%s", rc, err, stderr.String())
	}
	for _, want := range []string{"[claude-tmux] WARN: REPL seed send failed index=1 total=1", "[claude-tmux] seeded 0/1 REPL input line(s)", "[claude-tmux] prompt delivered"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if !strings.Contains(strings.Join(tx.sentSeq, ","), "paste-buffer") {
		t.Errorf("sent = %v, want the prompt pasted", tx.sentSeq)
	}
}

func TestPrepareTmuxREPL_AnUnwritableSystemPromptIsABadFlagBeforeAnySession(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	blocker := filepath.Join(ws, "plain")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tx := &fakeTmux{}
	var stderr bytes.Buffer
	deps := Deps{Tmux: tx, Stderr: &stderr, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Now: time.Now}
	cfg := &Config{Workspace: ws, Worktree: t.TempDir(), Realization: Realization{SystemPromptFile: filepath.Join(blocker, "system.md")}}

	_, rc, err := prepareTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{name: "claude-tmux", session: "s1", bootOnly: true})

	if rc != ExitBadFlags || err == nil || !strings.HasPrefix(err.Error(), "[claude-tmux] write system prompt: ") || len(tx.existing) != 0 {
		t.Errorf("rc=%d err=%v sessions=%v, want ExitBadFlags, the system-prompt error and no session", rc, err, tx.existing)
	}
}
