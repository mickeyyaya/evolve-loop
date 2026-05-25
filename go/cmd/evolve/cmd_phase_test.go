package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

// stubPhase is a minimal PhaseRunner used to drive cmd_phase tests
// without spinning real bridges + prompts.
type stubPhase struct {
	resp core.PhaseResponse
	err  error
	got  core.PhaseRequest
}

func (s *stubPhase) Name() string { return s.resp.Phase }
func (s *stubPhase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	s.got = req
	return s.resp, s.err
}

// snapshotRegistry captures every currently-registered phase and
// returns a restore func that re-establishes them. Tests that want a
// controlled (one-phase) registry pair this with ResetForTesting +
// Register inside the test body.
func snapshotRegistry(t *testing.T) func() {
	t.Helper()
	names := registry.Names()
	snap := make(map[string]registry.Factory, len(names))
	for _, n := range names {
		f, _ := registry.For(n)
		snap[n] = f
	}
	return func() {
		registry.ResetForTesting()
		for n, f := range snap {
			registry.Register(n, f)
		}
	}
}

func TestRunPhase_DispatchesToFactory(t *testing.T) {
	stub := &stubPhase{resp: core.PhaseResponse{
		Phase:   "intent",
		Verdict: core.VerdictPASS,
	}}
	defer snapshotRegistry(t)()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return stub })

	req := core.PhaseRequest{Cycle: 7, ProjectRoot: "/p", Workspace: "/w"}
	reqJSON, _ := json.Marshal(req)
	stdin := bytes.NewReader(reqJSON)
	var stdout, stderr bytes.Buffer

	code := runPhase([]string{"intent"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d, want 0 (stderr=%s)", code, stderr.String())
	}
	if stub.got.Cycle != 7 {
		t.Errorf("stub got Cycle=%d, want 7", stub.got.Cycle)
	}
	var got core.PhaseResponse
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse stdout: %v (raw=%s)", err, stdout.String())
	}
	if got.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", got.Verdict)
	}
}

func TestRunPhase_MissingPhaseName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPhase(nil, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
	if !strings.Contains(stderr.String(), "missing phase name") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestRunPhase_UnknownPhase(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPhase([]string{"nopephase"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
	if !strings.Contains(stderr.String(), "unknown phase") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestRunPhase_MalformedJSON(t *testing.T) {
	defer snapshotRegistry(t)()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return &stubPhase{} })

	var stdout, stderr bytes.Buffer
	code := runPhase([]string{"intent"}, strings.NewReader("not-json"), &stdout, &stderr)
	if code != 11 {
		t.Errorf("code=%d, want 11", code)
	}
}

func TestRunPhase_RunnerErrorExits1(t *testing.T) {
	stub := &stubPhase{
		resp: core.PhaseResponse{Phase: "intent", Verdict: core.VerdictFAIL},
		err:  errors.New("oops"),
	}
	defer snapshotRegistry(t)()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return stub })

	req := core.PhaseRequest{Cycle: 1}
	rJSON, _ := json.Marshal(req)
	var stdout, stderr bytes.Buffer
	code := runPhase([]string{"intent"}, bytes.NewReader(rJSON), &stdout, &stderr)
	if code != 1 {
		t.Errorf("code=%d, want 1", code)
	}
	// Partial response should still be emitted to stdout.
	var got core.PhaseResponse
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Errorf("expected partial response on stdout; parse err=%v raw=%q", err, stdout.String())
	}
}

// Regression: a real phase factory entry must exist in the registry
// for every phase constant. Catches missed phase additions.
func TestPhaseFactoriesCoverAllPhases(t *testing.T) {
	want := []core.Phase{
		core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD,
		core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro,
	}
	for _, p := range want {
		if _, ok := registry.For(string(p)); !ok {
			t.Errorf("registry missing %q", p)
		}
	}
}
