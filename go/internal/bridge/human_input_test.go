package bridge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHumanInput_NeedsAReasonAndAnAuditSink(t *testing.T) {
	t.Parallel()
	if _, err := parseLaunchArgs([]string{"--human-input", "--stream-output"}, nil); err == nil || !strings.Contains(err.Error(), "--human-input=<why>") {
		t.Errorf("a bare --human-input: err = %v, want a refusal that asks for the reason", err)
	}
	if err := (Deps{}).auditBypass("human-input", "demo"); !errors.Is(err, errBypassNotAudited) {
		t.Errorf("no sink: err = %v, want errBypassNotAudited", err)
	}
	var got []string
	deps := Deps{RecordBypass: func(name, reason string) error { got = append(got, name+"="+reason); return nil }}
	if err := deps.auditBypass("human-input", "demo"); err != nil || len(got) != 1 || got[0] != "human-input=demo" {
		t.Errorf("sink: err = %v, calls = %v, want one record", err, got)
	}
}

func TestHumanSampleMS(t *testing.T) {
	t.Parallel()
	if d := humanSampleMS(65, 20); d < 10*time.Millisecond {
		t.Fatalf("sample = %v, want >= 10ms", d)
	}
	if d := humanSampleMS(0, 0); d < 10*time.Millisecond {
		t.Fatalf("floor sample = %v, want >= 10ms", d)
	}
}

func TestHumanPrimitives(t *testing.T) {
	t.Parallel()
	tmux := &fakeTmux{}
	deps := Deps{Sleep: func(time.Duration) {}, Stderr: io.Discard, Tmux: tmux}

	humanBootPause(deps)

	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "line1\nline2\nline3")
	_, _ = pastePrompt(context.Background(), deps, "[t]", "s", pf, true)
	if len(tmux.sentKeys) == 0 {
		t.Fatal("paste-with-review should send Enter")
	}
	_, _ = pastePrompt(context.Background(), deps, "[t]", "s", "/no/such/file-xyz", true)

	tmux2 := &fakeTmux{}
	deps.Tmux = tmux2
	humanSendKeysCSV(context.Background(), deps, "s", "y,,Enter")
	if !tmux2.sentContains("y") {
		t.Fatalf("human send should deliver 'y'; sent=%v", tmux2.sentKeys)
	}

	humanReadingPause(deps, "a b")
	humanReadingPause(deps, strings.Repeat("word ", 40))
}

func TestClaudeTmux_HumanInput_AnAuditedReasonTakesTheHumanPath(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault, "this model is deprecated, Continue?"}}
	code2, se2 := runTmuxAudited(t, fx, tmux, "--allow-bypass", "--human-input=demo")
	if code2 != ExitRespondLoopGuard {
		t.Fatalf("human path exit = %d, want ExitRespondLoopGuard", code2)
	}
	if !tmux.sentContains("y") {
		t.Fatalf("human auto-respond should send 'y'; sent=%v", tmux.sentKeys)
	}
	if !strings.Contains(se2, "[human-input]") {
		t.Fatalf("human path should log [human-input]; se=%q", se2)
	}
}

func runTmuxAudited(t *testing.T, fx launchFixture, tmux *fakeTmux, extra ...string) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), CaptureBaseline: zeroBaselineCapture,
		RecordBypass: func(string, string) error { return nil },
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

func TestLaunch_AHumanInputWithoutAnAuditIsRefused(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	code, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--human-input=demo")
	if code != ExitSafetyGate || !strings.Contains(se, "no audit sink") {
		t.Fatalf("code = %d, stderr = %q, want the safety gate", code, se)
	}
}

func TestLaunch_ADryRunHumanInputNeedsNoAudit(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	code, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--human-input=demo", "--dry-run")
	if strings.Contains(se, "no audit sink") || code == ExitSafetyGate {
		t.Fatalf("code = %d, stderr = %q, want a dry run that writes no bypass record", code, se)
	}
}

func TestLaunch_AValidateOnlyHumanInputNeedsNoAudit(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	code, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--human-input=demo", "--validate-only")
	if strings.Contains(se, "no audit sink") || code == ExitSafetyGate {
		t.Fatalf("code = %d, stderr = %q, want a validate-only run that writes no bypass record", code, se)
	}
}
