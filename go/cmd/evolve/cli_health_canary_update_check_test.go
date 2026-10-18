package main

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestCLIHealthCanaryProbe_LaunchesCodexWithItsStartupUpdateCheckOff(t *testing.T) {
	t.Parallel()
	frames := slices.Repeat([]string{"starting"}, 256)
	pane := &bridge.FakeTmuxController{CaptureFrames: frames}
	deps := bridge.Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: func(string) (string, bool) { return "", false }, Stderr: io.Discard}

	liveProbeWith(context.Background(), t.TempDir(), "", deps)("codex-tmux")

	for _, keys := range pane.SentKeys {
		argv := strings.Fields(strings.ReplaceAll(keys, "'", ""))
		if !slices.ContainsFunc(argv, func(f string) bool { return filepath.Base(f) == "codex" }) {
			continue
		}
		if i := slices.Index(argv, "check_for_update_on_startup=false"); i < 1 || argv[i-1] != "-c" {
			t.Fatalf("the canary's codex probe launched %q; want -c check_for_update_on_startup=false, or codex's update menu answers the probe", argv)
		}
		return
	}
	t.Fatalf("the canary probe sent no codex launch line: %q", pane.SentKeys)
}

func TestDefaultLiveProbe_TheCanaryLaunchesTheCLIsOwnDefaultModelAndNeverAChosenTier(t *testing.T) {
	t.Parallel()
	for _, driver := range []string{"agy-tmux", "claude-tmux"} {
		t.Run(driver, func(t *testing.T) {
			t.Parallel()
			pane := &bridge.FakeTmuxController{CaptureFrames: slices.Repeat([]string{"starting"}, 256)}
			deps := bridge.Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: func(string) (string, bool) { return "", false }, Stderr: io.Discard}

			defaultLiveProbe(context.Background(), t.TempDir(), deps)(driver)

			launches := launchLines(pane.SentKeys)
			if len(launches) == 0 {
				t.Fatalf("the canary probe launched nothing: %q", pane.SentKeys)
			}
			for _, line := range launches {
				if strings.Contains(line, "--model") {
					t.Errorf("the canary tests whether a benched CLI recovered on its own default, so it passes no model: %s", line)
				}
			}
		})
	}
}
