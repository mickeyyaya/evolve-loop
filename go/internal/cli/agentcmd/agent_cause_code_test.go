package agentcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type stalledBridge struct {
	*fakeBridge
	cause string
}

func (b stalledBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	resp, _ := b.fakeBridge.Launch(ctx, req)
	resp.ExitCode, resp.CauseCode = 81, b.cause
	return resp, errors.New("bridge: launch exit=81: stalled: " + core.ErrArtifactTimeout.Error())
}

func runStalled(t *testing.T, cause string) (int, map[string]any) {
	t.Helper()
	fb := stalledBridge{fakeBridge: &fakeBridge{}, cause: cause}
	deps := depsWith(fb.fakeBridge)
	deps.Bridge = func(string, *signalcenter.Center) core.Bridge { return fb }
	var stdout, stderr bytes.Buffer
	code := New(deps)(runArgs(project(t), "--json"), strings.NewReader("diff"), &stdout, &stderr)
	var env map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("no envelope: %v out=%s err=%s", err, stdout.String(), stderr.String())
	}
	return code, env
}

func TestAgentRun_TheEnvelopeCarriesTheCauseCodeOfAStalledLaunch(t *testing.T) {
	code, env := runStalled(t, core.CauseGuardRefused)
	if code == 0 || env["cause_code"] != core.CauseGuardRefused {
		t.Fatalf("code=%d envelope=%v: the cause must reach the caller as a typed field", code, env)
	}
}

func TestAgentRun_AnOrdinaryStallKeepsItsOwnCauseInTheEnvelope(t *testing.T) {
	_, env := runStalled(t, "artifact_timeout")
	if env["cause_code"] != "artifact_timeout" {
		t.Fatalf("envelope=%v", env)
	}
}

func TestAgentRun_ASuccessfulRunHasNoCauseCode(t *testing.T) {
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer
	code := New(depsWith(fb))(runArgs(project(t), "--json"), strings.NewReader("diff"), &stdout, &stderr)
	if code != 0 || strings.Contains(stdout.String(), "cause_code") {
		t.Fatalf("code=%d out=%s", code, stdout.String())
	}
}
