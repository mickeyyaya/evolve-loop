package bridge

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// cutover_test.go — the EVOLVE_BRIDGE_GO M7 cutover branch: Launch/Probe
// route to the in-process engine when enabled, and stay on the bash path
// (engineFactory untouched) when not.

type fakeEngine struct {
	launched        bool
	probed          bool
	promptHasPolicy bool
	resp            core.BridgeResponse
}

func (f *fakeEngine) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.launched = true
	f.promptHasPolicy = strings.Contains(req.Prompt, "Subagent Interactive Policy")
	return f.resp, nil
}

func (f *fakeEngine) Probe(context.Context) (core.BridgeProbe, error) {
	f.probed = true
	return core.BridgeProbe{Version: "fake"}, nil
}

// failRunner is a CmdRunner that fails the test if the bash path is taken.
func failRunner(t *testing.T) CmdRunner {
	return func(context.Context, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		t.Helper()
		t.Fatal("bash runner must NOT be called on the in-process path")
		return 0, nil
	}
}

func TestAdapter_Launch_RoutesInProcessWhenEnabled(t *testing.T) {
	fe := &fakeEngine{resp: core.BridgeResponse{ExitCode: 0, Stdout: "ARTIFACT"}}
	a := New("bridge", failRunner(t))
	a.engineFactory = func(map[string]string) core.Bridge { return fe }

	resp, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "p", Workspace: t.TempDir(), ArtifactPath: "a",
		Prompt: "do it", Agent: "scout",
		Env: map[string]string{"EVOLVE_BRIDGE_GO": "1"},
	})
	if err != nil {
		t.Fatalf("Launch err: %v", err)
	}
	if !fe.launched {
		t.Fatal("Launch should route to the in-process engine when EVOLVE_BRIDGE_GO=1")
	}
	if !fe.promptHasPolicy {
		t.Fatal("adapter should inject the interactive-policy prefix before the in-process Launch")
	}
	if resp.Stdout != "ARTIFACT" {
		t.Fatalf("response not propagated from engine; got %q", resp.Stdout)
	}
}

func TestAdapter_Launch_BashPathWhenDisabled(t *testing.T) {
	fe := &fakeEngine{}
	bashCalled := false
	a := New("bridge", func(context.Context, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		bashCalled = true
		return 0, nil
	})
	a.engineFactory = func(map[string]string) core.Bridge { return fe }

	if _, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "p", Workspace: t.TempDir(), ArtifactPath: "a", Prompt: "x",
	}); err != nil {
		t.Fatalf("Launch err: %v", err)
	}
	if fe.launched {
		t.Fatal("must NOT route in-process when EVOLVE_BRIDGE_GO is unset")
	}
	if !bashCalled {
		t.Fatal("bash runner should have been used on the default path")
	}
}

func TestAdapter_Probe_RoutesInProcessWhenEnabled(t *testing.T) {
	t.Setenv("EVOLVE_BRIDGE_GO", "1")
	fe := &fakeEngine{}
	a := New("bridge", failRunner(t))
	a.engineFactory = func(map[string]string) core.Bridge { return fe }

	p, err := a.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe err: %v", err)
	}
	if !fe.probed {
		t.Fatal("Probe should route in-process when EVOLVE_BRIDGE_GO=1")
	}
	if p.Version != "fake" {
		t.Fatalf("probe result not propagated; got %q", p.Version)
	}
}
