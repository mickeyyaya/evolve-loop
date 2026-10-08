package bridge

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const smokeNeverReadyFrames = 256

func neverReadySmokePane() *FakeTmuxController {
	frames := make([]string, smokeNeverReadyFrames)
	for i := range frames {
		frames[i] = "starting"
	}
	return &FakeTmuxController{CaptureFrames: frames}
}

func smokeDeps(tm TmuxController) Deps {
	return Deps{Tmux: tm, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: io.Discard}
}

func smokeLaunchArgv(t *testing.T, sent []string, binary string) []string {
	t.Helper()
	for _, keys := range sent {
		argv := launchArgv(keys)
		if slices.ContainsFunc(argv, func(field string) bool { return filepath.Base(field) == binary }) {
			return argv
		}
	}
	t.Fatalf("no %s launch line among the keys sent: %q", binary, sent)
	return nil
}

func TestBootSmokeTest_ACallerWithNoRealizationLaunchesCodexWithTheManifestsArgs(t *testing.T) {
	pane := neverReadySmokePane()

	BootSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: t.TempDir()}, smokeDeps(pane))

	argv := smokeLaunchArgv(t, pane.SentKeys, "codex")
	if !slices.Contains(argv, "--yolo") || !carriesConfigOverride(argv, codexUpdateCheckOverride) {
		t.Fatalf("readiness-gate boot of codex launched %q; want the manifest's --yolo and -c %s, the argv a phase launch carries", argv, codexUpdateCheckOverride)
	}
}

func TestLiveSmokeTest_ACallerWithNoRealizationLaunchesCodexWithTheManifestsArgs(t *testing.T) {
	pane := neverReadySmokePane()

	LiveSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: t.TempDir()}, smokeDeps(pane))

	argv := smokeLaunchArgv(t, pane.SentKeys, "codex")
	if !slices.Contains(argv, "--yolo") || !carriesConfigOverride(argv, codexUpdateCheckOverride) {
		t.Fatalf("live probe of codex launched %q; want the manifest's --yolo and -c %s, the argv a phase launch carries", argv, codexUpdateCheckOverride)
	}
}

func TestBootSmokeTest_ACallerWithNoRealizationBootsClaudeAndAgyAsAPhaseLaunchDoes(t *testing.T) {
	cases := []struct {
		driver, binary string
		wantArg        string
		wantExport     string
	}{
		{"claude-tmux", "claude", "--dangerously-skip-permissions", "export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false"},
		{"agy-tmux", "agy", "--dangerously-skip-permissions", ""},
	}
	for _, tc := range cases {
		t.Run(tc.driver, func(t *testing.T) {
			pane := neverReadySmokePane()

			BootSmokeTest(context.Background(), tc.driver, &Config{Workspace: t.TempDir()}, smokeDeps(pane))

			if argv := smokeLaunchArgv(t, pane.SentKeys, tc.binary); !slices.Contains(argv, tc.wantArg) {
				t.Fatalf("%s boot smoke launched %q; want %s, the bypass posture the smoke already assumes", tc.driver, argv, tc.wantArg)
			}
			if tc.wantExport != "" && !slices.Contains(pane.SentKeys, tc.wantExport) {
				t.Fatalf("%s boot smoke never exported the manifest's default_env (%q); sent %q", tc.driver, tc.wantExport, pane.SentKeys)
			}
		})
	}
}

func TestBootSmokeTest_ACallerWithNoRealizationGetsOllamasDefaultArgs(t *testing.T) {
	cfg := &Config{Workspace: t.TempDir()}

	BootSmokeTest(context.Background(), "ollama-tmux", cfg, smokeDeps(neverReadySmokePane()))

	if !slices.Contains(cfg.Realization.LaunchFlags, "--experimental-yolo") {
		t.Fatalf("ollama boot smoke realized %q; want the manifest's --experimental-yolo", cfg.Realization.LaunchFlags)
	}
}

func TestBootSmokeTest_ACallerRealizationIsLaunchedUnchanged(t *testing.T) {
	pane := neverReadySmokePane()
	supplied := Realization{LaunchFlags: []string{"--caller-supplied-flag"}}

	BootSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: t.TempDir(), Realization: supplied}, smokeDeps(pane))

	argv := smokeLaunchArgv(t, pane.SentKeys, "codex")
	if !slices.Contains(argv, "--caller-supplied-flag") || slices.Contains(argv, "--yolo") {
		t.Fatalf("codex boot smoke launched %q; want the caller's realization alone", argv)
	}
	if strings.Contains(strings.Join(argv, " "), codexUpdateCheckOverride) {
		t.Fatalf("codex boot smoke merged the manifest into the caller's realization: %q", argv)
	}
}

func TestSmokeTests_AModelOnTheConfigReachesTheLaunchArgv(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	cases := []struct {
		driver, binary, model, want string
	}{
		{"agy-claude-tmux", "agy", "Claude Opus 5.5 (High)", "--model " + shellQuotePOSIX("Claude Opus 5.5 (High)")},
		{"agy-claude-tmux", "agy", "deep", "--model " + shellQuotePOSIX("Claude Opus 5.5 (Medium)")},
		{"claude-tmux", "claude", "opus", "--model opus"},
	}
	smokes := map[string]func(context.Context, string, *Config, Deps){
		"boot": func(ctx context.Context, d string, c *Config, deps Deps) { BootSmokeTest(ctx, d, c, deps) },
		"live": func(ctx context.Context, d string, c *Config, deps Deps) { LiveSmokeTest(ctx, d, c, deps) },
	}
	for name, smoke := range smokes {
		for _, tc := range cases {
			pane := neverReadySmokePane()
			smoke(context.Background(), tc.driver, &Config{Workspace: t.TempDir(), Model: tc.model}, smokeDeps(pane))
			line, _ := launchLineFor(t, pane.SentKeys, tc.binary)
			if !strings.Contains(line, tc.want) {
				t.Errorf("%s smoke of %s with model %q launched %q; want %s", name, tc.driver, tc.model, line, tc.want)
			}
		}
	}
}

func launchLineFor(t *testing.T, sent []string, binary string) (string, int) {
	t.Helper()
	for i, keys := range sent {
		if slices.ContainsFunc(launchArgv(keys), func(f string) bool { return filepath.Base(f) == binary }) {
			return keys, i
		}
	}
	t.Fatalf("no %s launch line among the keys sent: %q", binary, sent)
	return "", -1
}
