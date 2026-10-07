package phasecmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/phaseproto"
)

type recordingInstaller struct {
	roots []string
	err   error
	stub  *stubPhase
	ranAt int
}

func (r *recordingInstaller) install(root string) error {
	r.roots = append(r.roots, root)
	if r.stub.got.Cycle == 0 {
		r.ranAt = len(r.roots)
	}
	return r.err
}

func stubbedIntent(t *testing.T) *stubPhase {
	t.Helper()
	stub := &stubPhase{resp: core.PhaseResponse{Phase: "intent", Verdict: core.VerdictPASS}}
	t.Cleanup(registry.SnapshotForTest())
	registry.ResetForTesting()
	registry.Register("intent", func(core.PhaseRequest) core.PhaseRunner { return stub })
	return stub
}

func TestRunPhase_InstallsTheRootRouterForTheRequestsProjectBeforeTheRunnerRuns(t *testing.T) {
	stub := stubbedIntent(t)
	inst := &recordingInstaller{stub: stub}
	reqJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 7, ProjectRoot: "/declared"})
	var stdout, stderr bytes.Buffer

	code := NewRunPhase(nil, inst.install)([]string{"intent"}, bytes.NewReader(reqJSON), &stdout, &stderr)

	if code != 0 || len(inst.roots) != 1 || inst.roots[0] != "/declared" || inst.ranAt != 1 || stub.got.Cycle != 7 {
		t.Fatalf("code=%d roots=%v installedBeforeRun=%v ran=%v stderr=%s", code, inst.roots, inst.ranAt == 1, stub.got.Cycle == 7, stderr.String())
	}
}

func TestRunPhase_ARefusedRootRouterExitsTwoAndRunsNothing(t *testing.T) {
	stub := stubbedIntent(t)
	inst := &recordingInstaller{stub: stub, err: errors.New("cli_routing.clis: claude must be listed")}
	reqJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 7, ProjectRoot: "/declared"})
	var stdout, stderr bytes.Buffer

	code := NewRunPhase(nil, inst.install)([]string{"intent"}, bytes.NewReader(reqJSON), &stdout, &stderr)

	if code != exitRoutingRefused || stub.got.Cycle != 0 || !bytes.Contains(stderr.Bytes(), []byte("cli_routing.clis")) {
		t.Fatalf("code=%d ran=%v stderr=%q, want exit 2 naming the refusal, nothing run", code, stub.got.Cycle != 0, stderr.String())
	}
}

func TestNewRunServePhase_InstallsTheRootRouterForTheRequestsProject(t *testing.T) {
	stub := stubbedIntent(t)
	inst := &recordingInstaller{stub: stub}
	var stdout, stderr bytes.Buffer

	code := NewRunServePhase(inst.install)([]string{"intent"}, bytes.NewReader(envelopeStdin(t, core.PhaseRequest{Cycle: 11, ProjectRoot: "/declared"})), &stdout, &stderr)

	if code != 0 || len(inst.roots) != 1 || inst.roots[0] != "/declared" || inst.ranAt != 1 || stub.got.Cycle != 11 {
		t.Fatalf("code=%d roots=%v installedBeforeRun=%v stderr=%s", code, inst.roots, inst.ranAt == 1, stderr.String())
	}
}

func TestNewRunServePhase_ARefusedRootRouterIsAnErrorEnvelopeAndRunsNothing(t *testing.T) {
	stub := stubbedIntent(t)
	inst := &recordingInstaller{stub: stub, err: errors.New("cli_routing.clis: claude must be listed")}
	var stdout, stderr bytes.Buffer

	NewRunServePhase(inst.install)([]string{"intent"}, bytes.NewReader(envelopeStdin(t, core.PhaseRequest{Cycle: 11, ProjectRoot: "/declared"})), &stdout, &stderr)

	if env := decodeResponseEnvelope(t, stdout.Bytes()); env.Kind != phaseproto.KindError || stub.got.Cycle != 0 {
		t.Fatalf("envelope kind %q, ran=%v: a refused route is a wire error and the phase never runs", env.Kind, stub.got.Cycle != 0)
	}
}

func TestRunPhase_ANilRouterInstallerRunsThePhaseWithNoRouterStep(t *testing.T) {
	stub := stubbedIntent(t)
	var noInstall RouterInstaller
	reqJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 9, ProjectRoot: "/legacy"})
	var stdout, stderr bytes.Buffer

	code := NewRunPhase(nil, noInstall)([]string{"intent"}, bytes.NewReader(reqJSON), &stdout, &stderr)

	if code != 0 || stub.got.Cycle != 9 {
		t.Fatalf("code=%d ran=%v stderr=%s: a nil installer is no step, never a refusal", code, stub.got.Cycle == 9, stderr.String())
	}
}
