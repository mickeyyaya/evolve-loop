package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// realizer_wiring_test.go — ADR-0022 Phase 2b/3 acceptance. Proves the tmux
// drivers build their launch command from the per-CLI Realization (the single
// owner of model+permission+raw flags), so a claude-origin profile's flags
// never leak into agy/codex. This is the contract the cycle-1 multi-CLI boot
// failure violated: profile.extra_flags were claude argv forwarded verbatim to
// every CLI.
//
// The migrated profile shape: extra_flags_by_cli keyed per CLI (claude flags
// live under "claude-tmux"), and NO permission_mode (the bypass posture is the
// realized default). A profile switched to agy/codex therefore realizes to
// that CLI's own flags only — RawByCLI[agy/codex] is nil.

// writeIntentProfile writes a migrated-shape profile (extra_flags_by_cli, no
// permission_mode) and returns its path. The launch goes through the real
// runner entry (engine.Launch), which must enable bypass for the in-process
// path so the tmux safety gates pass without an explicit --allow-bypass.
func writeIntentProfile(t *testing.T, dir, name, cli string, extraByCLI map[string][]string) string {
	t.Helper()
	body := map[string]any{
		"name":               name,
		"cli":                cli,
		"model_tier_default": "sonnet",
		"allowed_tools":      []string{"Read", "Write"},
		"extra_flags_by_cli": extraByCLI,
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

// launchedCmd returns the recorded SendKeys line that launched the inner CLI
// (the one beginning with the binary name), or "" if none was sent.
func launchedCmd(tmux *fakeTmux, binary string) string {
	for _, k := range tmux.sentKeys {
		if strings.HasPrefix(k, binary+" ") || k == binary {
			return k
		}
	}
	return ""
}

func TestRealizerWiring_NoCrossCLILeak(t *testing.T) {
	// The claude flags every profile carried before the migration. Keyed under
	// claude-tmux so a profile switched to agy/codex realizes none of them.
	claudeRaw := []string{
		"--exclude-dynamic-system-prompt-sections",
		"--disable-slash-commands",
		"--setting-sources", "project",
		"--plugin-dir", ".evolve/plugin",
	}
	extraByCLI := map[string][]string{"claude-tmux": claudeRaw}

	cases := []struct {
		cli    string
		binary string
		marker string
		want   string   // exact launch command line
		absent []string // flags that must NOT appear (cross-CLI leak)
	}{
		{
			cli:    "claude-tmux",
			binary: "claude",
			marker: "❯",
			want:   "claude --model sonnet --dangerously-skip-permissions --exclude-dynamic-system-prompt-sections --disable-slash-commands --setting-sources project --plugin-dir .evolve/plugin",
			absent: []string{"--no-session-persistence"},
		},
		{
			cli:    "agy-tmux",
			binary: "agy",
			marker: "? for shortcuts",
			want:   "agy --dangerously-skip-permissions",
			absent: []string{"--setting-sources", "--plugin-dir", "--exclude-dynamic-system-prompt-sections", "--model", "--no-session-persistence"},
		},
		{
			cli:    "codex-tmux",
			binary: "codex",
			marker: "›",
			want:   "codex -m gpt-5.4",
			absent: []string{"--setting-sources", "--plugin-dir", "--dangerously-skip-permissions", "--exclude-dynamic-system-prompt-sections", "--no-session-persistence"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.cli, func(t *testing.T) {
			ws := t.TempDir()
			profile := writeIntentProfile(t, ws, "agent", tc.cli, extraByCLI)
			artifact := filepath.Join(ws, "artifact.md")
			if err := os.WriteFile(artifact, []byte("DONE\n"), 0o644); err != nil {
				t.Fatalf("seed artifact: %v", err)
			}
			tmux := &fakeTmux{paneSeq: []string{tc.marker}}
			eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})

			resp, err := eng.Launch(context.Background(), core.BridgeRequest{
				CLI:          tc.cli,
				Profile:      profile,
				Model:        "sonnet",
				Prompt:       "do the thing",
				Workspace:    ws,
				ArtifactPath: artifact,
				Agent:        "agent",
			})
			if err != nil || resp.ExitCode != ExitOK {
				t.Fatalf("launch failed: exit=%d err=%v (stderr=%q)", resp.ExitCode, err, resp.Stderr)
			}

			got := launchedCmd(tmux, tc.binary)
			if got != tc.want {
				t.Fatalf("launch cmd:\n got: %q\nwant: %q\nsentKeys=%v", got, tc.want, tmux.sentKeys)
			}
			joined := strings.Join(tmux.sentKeys, " ")
			for _, leak := range tc.absent {
				if strings.Contains(joined, leak) {
					t.Fatalf("cross-CLI leak: %q must not reach %s; sentKeys=%v", leak, tc.cli, tmux.sentKeys)
				}
			}
		})
	}
}

// TestEngineLaunch_EnablesBypassForInProcessPath pins the fix for the
// AllowBypass gap: the runner's in-process entry (engine.Launch) must let the
// tmux safety gates pass without the caller threading --allow-bypass, since
// the autonomous orchestrator is the trusted bypass authority. Uses a buffer
// to confirm we never hit ExitSafetyGate.
func TestEngineLaunch_EnablesBypassForInProcessPath(t *testing.T) {
	ws := t.TempDir()
	profile := writeIntentProfile(t, ws, "agent", "agy-tmux", nil)
	artifact := filepath.Join(ws, "artifact.md")
	if err := os.WriteFile(artifact, []byte("DONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{"? for shortcuts"}}
	var stderr bytes.Buffer
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: &stderr})
	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "agy-tmux", Profile: profile, Model: "sonnet", Prompt: "x",
		Workspace: ws, ArtifactPath: artifact, Agent: "agent",
	})
	if resp.ExitCode == ExitSafetyGate {
		t.Fatalf("in-process launch must enable bypass; got ExitSafetyGate (stderr=%q)", resp.Stderr)
	}
	if err != nil || resp.ExitCode != ExitOK {
		t.Fatalf("launch failed: exit=%d err=%v stderr=%q", resp.ExitCode, err, resp.Stderr)
	}
}
