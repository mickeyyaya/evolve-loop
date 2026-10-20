package opscmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

type fakeClaudeReply struct {
	*bridge.FakeTmuxController
	configRoot, model string
}

func (f *fakeClaudeReply) PasteBuffer(ctx context.Context, session string) error {
	if err := f.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	ws := filepath.Dir(f.LoadedBuffers[len(f.LoadedBuffers)-1])
	dir := filepath.Join(f.configRoot, "projects", "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	line := `{"type":"assistant","cwd":"` + filepath.Join(ws, "bridge-scratch-cwd") + `","timestamp":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","message":{"id":"m1","model":"` + f.model + `","usage":{"input_tokens":1,"output_tokens":1}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(line), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws, bridge.LiveSmokeArtifact), []byte("OK\n"), 0o644)
}

type servedModelReport struct {
	ExitCode    int    `json:"exit_code"`
	ServedModel string `json:"served_model"`
}

func decodeServedModelReport(t *testing.T, out []byte) servedModelReport {
	t.Helper()
	var rep servedModelReport
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("json report %q: %v", out, err)
	}
	return rep
}

func TestDoctorLive_ReportsTheModelThatServedTheReplyFromAFakeTranscript(t *testing.T) {
	var out, errb bytes.Buffer
	pane := &fakeClaudeReply{
		FakeTmuxController: &bridge.FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}},
		configRoot:         t.TempDir(),
		model:              "claude-sonnet-5-5",
	}
	deps := bridge.Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: func(string) (string, bool) { return "", false }, Stderr: &errb, TokenResolver: tokenusage.DefaultResolver(pane.configRoot)}

	code := runDoctorLiveWith([]string{"claude-tmux", "--model", "sonnet", "--json"}, &out, deps)

	rep := decodeServedModelReport(t, out.Bytes())
	if code != exitOK || rep.ServedModel != "claude-sonnet-5-5" {
		t.Fatalf("exit %d, report %+v, stderr:\n%s\nwant exit 0 and served_model claude-sonnet-5-5", code, rep, errb.String())
	}
	if !strings.Contains(errb.String(), "[doctor] served model: claude-sonnet-5-5\n") {
		t.Errorf("stderr lacks the served model line:\n%s", errb.String())
	}
}

func TestDoctorLive_ADriverThatReportsNoModelPrintsUnreportedAndKeepsItsExitCode(t *testing.T) {
	installDoctorUsage(t, usageprobe.Evidence{CLI: "agy", Family: "agy-claude", Verdict: usageprobe.VerdictHealthy, Detail: "d"})
	var out, errb bytes.Buffer
	_, deps := neverReadyPaneDeps(&errb)

	code := runDoctorLiveWith([]string{"agy-claude-tmux", "--json"}, &out, deps)

	rep := decodeServedModelReport(t, out.Bytes())
	if code != exitFail || rep.ExitCode == bridge.ExitOK || rep.ServedModel != "unreported" {
		t.Fatalf("exit %d, report %+v, want exit 1, the probe's failing rc and served_model unreported", code, rep)
	}
	if !strings.Contains(errb.String(), "[doctor] served model: unreported\n") {
		t.Errorf("stderr lacks the unreported served model line:\n%s", errb.String())
	}
}
