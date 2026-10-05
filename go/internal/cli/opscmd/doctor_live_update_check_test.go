package opscmd

import (
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestDoctorLive_LaunchesCodexWithItsStartupUpdateCheckOff(t *testing.T) {
	frames := slices.Repeat([]string{"starting"}, 256)
	pane := &bridge.FakeTmuxController{CaptureFrames: frames}
	deps := bridge.Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: func(string) (string, bool) { return "", false }, Stderr: io.Discard}

	runDoctorLiveWith([]string{"codex-tmux"}, io.Discard, deps)

	for _, keys := range pane.SentKeys {
		argv := strings.Fields(strings.ReplaceAll(keys, "'", ""))
		if !slices.ContainsFunc(argv, func(f string) bool { return filepath.Base(f) == "codex" }) {
			continue
		}
		if i := slices.Index(argv, "check_for_update_on_startup=false"); i < 1 || argv[i-1] != "-c" {
			t.Fatalf("evolve doctor live launched codex as %q; want -c check_for_update_on_startup=false, or codex's update menu answers the probe", argv)
		}
		return
	}
	t.Fatalf("evolve doctor live sent no codex launch line: %q", pane.SentKeys)
}
