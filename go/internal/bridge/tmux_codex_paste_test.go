package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type codexPasteChipTmux struct {
	*pasteStickyTmux
	pane string
}

func (p *codexPasteChipTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	p.mu.Lock()
	stuck := p.stuck
	p.mu.Unlock()
	if stuck {
		return p.pane, nil
	}
	return p.fakeTmux.CapturePane(ctx, session, scrollback)
}

func TestTmuxPromptCodexPasteChipRecovery(t *testing.T) {
	for _, tc := range []struct{ name, driver, marker, pane, want string }{
		{"parked", "codex-tmux", "›", "› [Pasted Content 52509 chars]\n  gpt-6-astra high", `"payload":"site=prompt resends=1","result":"submitted_after_resend"`},
		{"quoted history", "codex-tmux", "›", "Earlier: [Pasted Content 52509 chars]\n› ", `"payload":"site=prompt resends=0","result":"submit_verified"`},
		{"different driver", "claude-tmux", "❯", "❯ [Pasted Content 52509 chars]", `"payload":"site=prompt resends=0","result":"submit_verified"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fixtureConfig(t)
			tm := &codexPasteChipTmux{pasteStickyTmux: &pasteStickyTmux{fakeTmux: &fakeTmux{paneSeq: []string{tc.marker}}}, pane: tc.pane}
			rc, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{name: tc.driver, session: "codex-chip", launchCmd: "fixture", promptMarker: tc.marker, inputLineMarker: tc.marker, bootIntervalS: 1})
			// No artifact is produced; only the parked Codex input licenses a resend.
			if err != nil || rc != ExitArtifactTimeout {
				t.Fatalf("rc=%d err=%v", rc, err)
			}
			events, err := os.ReadFile(filepath.Join(cfg.Workspace, "build-interactions.ndjson"))
			if err != nil {
				t.Fatal(err)
			}
			first := strings.SplitN(string(events), "\n", 2)[0]
			if !strings.Contains(first, tc.want) {
				t.Fatalf("wrong initial submission outcome: %s, want %s", first, tc.want)
			}
		})
	}
}
