package agentcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestAgentRun_SeatTokenFileReachesTheBridgeRequestAsAnAbsolutePath(t *testing.T) {
	root := project(t)
	dir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.WriteFile(filepath.Join(dir, "tok"), []byte("sk-ant-oat01-X"), 0o600); err != nil {
		t.Fatal(err)
	}
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer
	code := New(depsWith(fb))(runArgs(root, "--seat-token-file", "tok"), strings.NewReader("diff"), &stdout, &stderr)
	if code != 0 || len(fb.requests) != 1 {
		t.Fatalf("code=%d err=%s", code, stderr.String())
	}
	got, _ := filepath.EvalSymlinks(fb.requests[0].SeatTokenFile)
	want, _ := filepath.EvalSymlinks(filepath.Join(dir, "tok"))
	if got != want || !filepath.IsAbs(fb.requests[0].SeatTokenFile) {
		t.Fatalf("SeatTokenFile=%q want %q", fb.requests[0].SeatTokenFile, want)
	}
}

func TestAgentRun_WithoutTheFlagTheRequestHasNoSeatTokenFile(t *testing.T) {
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer
	code := New(depsWith(fb))(runArgs(project(t)), strings.NewReader("diff"), &stdout, &stderr)
	if code != 0 || len(fb.requests) != 1 || fb.requests[0].SeatTokenFile != "" {
		t.Fatalf("code=%d requests=%+v", code, fb.requests)
	}
}

type seatLoginBridge struct {
	*fakeBridge
	login string
}

func (b seatLoginBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	resp, err := b.fakeBridge.Launch(ctx, req)
	resp.SeatLogin = b.login
	return resp, err
}

func TestAgentRun_TheEnvelopeNamesTheSeatLoginAndNeverTheTokenFileContent(t *testing.T) {
	root := project(t)
	tok := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(tok, []byte("sk-ant-oat01-ENVELOPE-CANARY"), 0o600); err != nil {
		t.Fatal(err)
	}
	fb := seatLoginBridge{fakeBridge: &fakeBridge{artifact: passReport}, login: "isolated"}
	var stdout, stderr bytes.Buffer
	deps := depsWith(fb.fakeBridge)
	deps.Bridge = func(string, *signalcenter.Center) core.Bridge { return fb }
	code := New(deps)(runArgs(root, "--json", "--seat-token-file", tok), strings.NewReader("diff"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr.String())
	}
	var env struct {
		SeatLogin string `json:"seat_login"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || env.SeatLogin != "isolated" {
		t.Fatalf("err=%v out=%s", err, stdout.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "ENVELOPE-CANARY") {
		t.Fatal("the token file content reached the output")
	}
}

func TestAgentRun_UsageNamesTheSeatTokenFileFlag(t *testing.T) {
	if !strings.Contains(usage, "--seat-token-file FILE") || !strings.Contains(usage, "one seat") {
		t.Fatal("the usage does not describe --seat-token-file")
	}
}

func TestAgentRun_ASeatTokenFileThatCannotBeMadeAbsoluteIsAUsageError(t *testing.T) {
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	d := depsWith(fb)
	d.Abs = func(string) (string, error) { return "", errors.New("no working directory") }
	var stdout, stderr bytes.Buffer
	code := New(d)(runArgs(root, "--seat-token-file", "tok"), strings.NewReader("diff"), &stdout, &stderr)
	if code == 0 || len(fb.requests) != 0 || !strings.Contains(stderr.String(), "--seat-token-file: no working directory") {
		t.Fatalf("code=%d err=%s", code, stderr.String())
	}
}

func TestAgentRun_ATokenGivenAsTheSeatTokenFileValueIsRefusedAndNeverPrinted(t *testing.T) {
	probe := "sk-ant-oat01-PROBECANARY-abc"
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer
	code := New(depsWith(fb))(runArgs(project(t), "--seat-token-file", "/cwd/"+probe), strings.NewReader("diff"), &stdout, &stderr)
	if code == 0 || len(fb.requests) != 0 {
		t.Fatalf("code=%d requests=%d: the launch must not start", code, len(fb.requests))
	}
	for name, text := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String()} {
		if strings.Contains(text, "PROBECANARY") {
			t.Errorf("%s holds the token: %q", name, text)
		}
	}
	if !strings.Contains(stderr.String(), "--seat-token-file") {
		t.Errorf("stderr=%q", stderr.String())
	}
}
