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

func launchedCmd(tmux *fakeTmux, binary string) string {
	for _, k := range tmux.sentKeys {
		if strings.HasPrefix(k, binary+" ") || k == binary {
			return k
		}
	}
	return ""
}

func TestRealizerWiring_NoCrossCLILeak(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
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
		want   string
		absent []string
	}{
		{
			cli:    "claude-tmux",
			binary: "claude",
			marker: "❯",
			want:   "claude --strict-mcp-config --model sonnet --dangerously-skip-permissions --effort medium --append-system-prompt-file {identity} --exclude-dynamic-system-prompt-sections --disable-slash-commands --setting-sources project --plugin-dir .evolve/plugin",
			absent: []string{"--no-session-persistence"},
		},
		{
			cli:    "agy-tmux",
			binary: "agy",
			marker: "? for shortcuts",
			want:   "agy --model 'Gemini 3.8 Flash (High)' --dangerously-skip-permissions",
			absent: []string{" -m ", "--setting-sources", "--plugin-dir", "--exclude-dynamic-system-prompt-sections", "--no-session-persistence"},
		},
		{
			cli:    "codex-tmux",
			binary: "codex",
			marker: "›",
			want:   "codex --yolo -c 'check_for_update_on_startup=false' -m gpt-5.6-terra -c 'model_reasoning_effort=medium' -c 'plan_mode_reasoning_effort=medium'",
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
			eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), CaptureBaseline: zeroBaselineCapture})

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
			want := strings.ReplaceAll(tc.want, "{identity}", shellQuotePOSIX(filepath.Join(ws, "pane-authority.md")))
			if got != want {
				t.Fatalf("launch cmd:\n got: %q\nwant: %q\nsentKeys=%v", got, want, tmux.sentKeys)
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

func TestEngineLaunch_EnablesBypassForInProcessPath(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	profile := writeIntentProfile(t, ws, "agent", "agy-tmux", nil)
	artifact := filepath.Join(ws, "artifact.md")
	if err := os.WriteFile(artifact, []byte("DONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{"? for shortcuts"}}
	var stderr bytes.Buffer
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: &stderr, CaptureBaseline: zeroBaselineCapture})
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
