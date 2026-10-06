package opscmd

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func neverReadyPaneDeps(stderr io.Writer) (*bridge.FakeTmuxController, bridge.Deps) {
	frames := make([]string, 256)
	for i := range frames {
		frames[i] = "starting"
	}
	pane := &bridge.FakeTmuxController{CaptureFrames: frames}
	return pane, bridge.Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: func(string) (string, bool) { return "", false }, Stderr: stderr}
}

func agyLaunch(sent []string) string {
	for _, keys := range sent {
		if strings.Contains(keys, " agy ") || strings.HasPrefix(keys, "agy ") {
			return keys
		}
	}
	return ""
}

func TestDoctorLive_TheModelFlagReachesTheLaunchArgv(t *testing.T) {
	for _, args := range [][]string{
		{"agy-claude-tmux", "--model", "Claude Opus 5.5 (High)"},
		{"--model", "Claude Opus 5.5 (High)", "agy-claude-tmux"},
		{"agy-claude-tmux", "--model=Claude Opus 5.5 (High)"},
	} {
		pane, deps := neverReadyPaneDeps(io.Discard)

		runDoctorLiveWith(args, io.Discard, deps)

		line := agyLaunch(pane.SentKeys)
		if !strings.Contains(line, "--model 'Claude Opus 5.5 (High)'") {
			t.Errorf("evolve doctor live %q launched %q; want agy --model 'Claude Opus 5.5 (High)'", args, line)
		}
	}
}

func TestDoctorLive_TheReportNamesTheModelItProbed(t *testing.T) {
	var out, errb bytes.Buffer
	_, deps := neverReadyPaneDeps(&errb)

	runDoctorLiveWith([]string{"agy-claude-tmux", "--json", "--model", "Claude Sonnet 5.5 (High)"}, &out, deps)

	var rep struct {
		Driver string `json:"driver"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("json report %q: %v", out.String(), err)
	}
	if rep.Driver != "agy-claude-tmux" || rep.Model != "Claude Sonnet 5.5 (High)" {
		t.Fatalf("report = %+v, want the driver and the model probed", rep)
	}
	if want := `LIVE FAILED: agy-claude-tmux --model "Claude Sonnet 5.5 (High)" rc=`; !strings.Contains(errb.String(), want) {
		t.Fatalf("stderr = %q, want a line naming %s", errb.String(), want)
	}
}

func TestDoctorLive_AModelFlagWithNoValueIsAUsageError(t *testing.T) {
	_, deps := neverReadyPaneDeps(io.Discard)
	if code := runDoctorLiveWith([]string{"agy-claude-tmux", "--model"}, io.Discard, deps); code != 10 {
		t.Fatalf("exit = %d, want 10", code)
	}
}
