package bridge

import (
	"context"
	"strings"
	"testing"
)

const agyAutoUpdateOffEnv = "AGY_CLI_DISABLE_AUTO_UPDATE"

func TestAgyManifests_EveryLaunchRealizesTheSelfUpdaterOff(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"agy-tmux", "agy"} {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if m.AutoUpdateOffEnv != agyAutoUpdateOffEnv {
			t.Errorf("%s auto_update_off_env = %q, want %q (agy's own updater reads it)", name, m.AutoUpdateOffEnv, agyAutoUpdateOffEnv)
		}
		if got := RealizeFor(name, LaunchIntent{}).Env[agyAutoUpdateOffEnv]; got == "" {
			t.Errorf("a realized %s launch must carry %s, or agy updates itself mid-wave; env=%v", name, agyAutoUpdateOffEnv, RealizeFor(name, LaunchIntent{}).Env)
		}
	}
}

func TestAutoUpdateOffEnv_EveryDeclaredOffSwitchIsSetInDefaultEnv(t *testing.T) {
	t.Parallel()
	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if m.AutoUpdateOffEnv != "" && m.DefaultEnv[m.AutoUpdateOffEnv] == "" {
			t.Errorf("%s names %s as its self-updater's off switch but default_env leaves it unset", name, m.AutoUpdateOffEnv)
		}
	}
}

func TestRecipeBoot_ExportsTheRealizedEnvBetweenTheCdAndTheLaunch(t *testing.T) {
	tx := &fakeTmux{paneSeq: []string{"booting", "ready\n❯"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "sess")
	d.cfg.Realization = Realization{Env: map[string]string{agyAutoUpdateOffEnv: "1"}}

	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}

	cdAt, exportAt, launchAt := -1, -1, -1
	for i, k := range tx.sentKeys {
		switch {
		case strings.HasPrefix(k, "cd "):
			cdAt = i
		case strings.HasPrefix(k, "export "+agyAutoUpdateOffEnv+"="):
			exportAt = i
		case strings.Contains(k, "--dangerously-skip-permissions"):
			launchAt = i
		}
	}
	if !(cdAt >= 0 && cdAt < exportAt && exportAt < launchAt) {
		t.Fatalf("the recipe boot must export the realized env after the cd and before the launch (cd=%d export=%d launch=%d); sent %q", cdAt, exportAt, launchAt, tx.sentKeys)
	}
}

func TestRecipeBoot_AClaudeModelCaptureLaunchesWithTheRealManifestsDefaultEnv(t *testing.T) {
	tx := &fakeTmux{paneSeq: []string{"booting", "ready\n❯"}}
	deps := recipeDeps(tx)
	d, _, err := newRecipeDriver(&Config{Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "models", Realization: RealizeFor("claude-tmux", LaunchIntent{})}, deps, "claude-tmux")
	if err != nil {
		t.Fatalf("newRecipeDriver: %v", err)
	}

	_ = d.EnsureSession(context.Background())

	if !tx.sentContains(promptSuggestionExport) {
		t.Fatalf("claude's /model capture (bridgeModelCapturer, the recipe boot) must launch with claude-tmux's default_env; sent %q", tx.sentKeys)
	}
}
