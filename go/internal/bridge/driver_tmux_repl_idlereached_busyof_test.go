package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func idleReachedBracketRegionSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "driver_tmux_channel.go"))
	if err != nil {
		t.Fatalf("read driver_tmux_channel.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	start, end := -1, -1
	for i, ln := range lines {
		if start == -1 && strings.Contains(ln, "func (c *replLiveChannel) observeIdle(") {
			start = i
		}
		if start != -1 && strings.Contains(ln, `c.openCorrID = ""`) {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatal("could not locate the idle_reached bracket region markers in driver_tmux_channel.go")
	}
	return strings.Join(lines[start:end+1], "\n")
}

func TestRunTmuxREPL_NoDirectChromeParseAtIdleReachedBracket(t *testing.T) {
	t.Parallel()
	region := idleReachedBracketRegionSource(t)
	if strings.Contains(region, "panestream.PaneBusy(") {
		t.Error("idle_reached bracket still calls panestream.PaneBusy( directly — must read it via panestream.LivenessCenter.BusyOf instead")
	}
}
