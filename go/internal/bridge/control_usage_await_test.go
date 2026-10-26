package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

func usageController(t *testing.T, panes ...string) (clicontrol.Controller, *fakeTmux) {
	t.Helper()
	tx := &fakeTmux{existing: map[string]bool{"evolve-bridge-named-usage310": true}, paneSeq: panes}
	cfg := &Config{Workspace: t.TempDir(), Agent: "control", SessionName: "usage310"}
	return NewController(cfg, recipeDeps(tx)), tx
}

func TestController_Usage2_1_310WaitsPastTheIdlePromptUntilTheRowsStopChanging(t *testing.T) {
	t.Parallel()
	idle := usagePane(t, "claude_usage_2.1.310_idle.txt")
	partial := usagePane(t, "claude_usage_2.1.310_partial.txt")
	full := usagePane(t, "claude_usage_2.1.310.txt")
	ctrl, _ := usageController(t, idle, idle, partial, partial, full, full)
	resp, err := ctrl.Do(context.Background(), "claude", clicontrol.EventUsage)
	if err != nil {
		t.Fatal(err)
	}
	windows := UsageWindows("claude", resp.Pane, usageNow)
	if len(windows) != 3 || windows[2].Model != "Fable" || windows[1].PercentUsed != 36 {
		t.Fatalf("windows = %+v; the idle prompt already shows the marker, so the capture must wait for the three rows of claude 2.1.310", windows)
	}
}

func TestController_AUsageScreenWithNoRowsIsReturnedAfterTheUsageBudget(t *testing.T) {
	t.Parallel()
	ctrl, tx := usageController(t, usagePane(t, "claude_usage_2.1.310_idle.txt"))
	resp, err := ctrl.Do(context.Background(), "claude", clicontrol.EventUsage)
	if err != nil || !strings.Contains(resp.Pane, "❯") {
		t.Fatalf("pane %q, err %v; the last screen is returned for the caller to judge", resp.Pane, err)
	}
	if got := len(tx.captureScrollback); got < usageCaptureSettleTicks {
		t.Errorf("captures = %d, want the whole usage budget of %d ticks spent", got, usageCaptureSettleTicks)
	}
}

func TestController_AStatusControlStillStopsAtThePromptMarker(t *testing.T) {
	t.Parallel()
	ctrl, tx := usageController(t, "status ❯")
	resp, err := ctrl.Do(context.Background(), "claude", clicontrol.EventStatus)
	if err != nil || resp.Pane != "status ❯" || len(tx.captureScrollback) >= usageCaptureSettleTicks {
		t.Fatalf("pane %q after %d captures, err %v; a control without a windows await stops at the first marker", resp.Pane, len(tx.captureScrollback), err)
	}
}

func TestParseManifest_RefusesAWindowsAwaitWithoutAWindowSpec(t *testing.T) {
	t.Parallel()
	body := `{"cli":"x-tmux","binary":"x","controls":{"status":{"send":"/status","await":"windows"}}}`
	_, err := parseManifestWithStderr("x-tmux", []byte(body), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "controls.status.await") {
		t.Fatalf("err = %v; a windows await with no windows to read must be refused", err)
	}
}

func TestUsageWindowsDeclared_NamesTheFamiliesWhoseManifestReadsWindows(t *testing.T) {
	t.Parallel()
	for family, want := range map[string]bool{"claude": true, "agy": true, "codex": false, "no-such": false} {
		if got := UsageWindowsDeclared(family); got != want {
			t.Errorf("UsageWindowsDeclared(%q) = %v, want %v", family, got, want)
		}
	}
}
