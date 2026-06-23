package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"
)

// composeStub records every Run call and returns scripted responses
// keyed by phase name.
type composeStub struct {
	scripts map[string]core.PhaseResponse
	calls   []string
}

func (s *composeStub) factory(name string) registry.Factory {
	return func(req core.PhaseRequest) core.PhaseRunner {
		return &composeStubRunner{stub: s, name: name}
	}
}

type composeStubRunner struct {
	stub *composeStub
	name string
}

func (r *composeStubRunner) Name() string { return r.name }
func (r *composeStubRunner) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	r.stub.calls = append(r.stub.calls, r.name)
	resp, ok := r.stub.scripts[r.name]
	if !ok {
		return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS}, nil
	}
	return resp, nil
}

func registerCompose(t *testing.T, stub *composeStub, phaseNames ...string) {
	t.Helper()
	registry.ResetForTesting()
	for _, n := range phaseNames {
		registry.Register(n, stub.factory(n))
	}
}

// TestCompose_HappyPath_AllPhasesPASS — exit 0 + all calls recorded.
func TestCompose_HappyPath_AllPhasesPASS(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "scout", "audit")

	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 1})
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "scout,audit"}, bytes.NewReader(envJSON), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d, want 0; stderr=%s", code, stderr.String())
	}
	if len(stub.calls) != 2 || stub.calls[0] != "scout" || stub.calls[1] != "audit" {
		t.Errorf("call sequence=%v, want [scout audit]", stub.calls)
	}
	if !strings.Contains(stdout.String(), "sequence: scout -> audit") {
		t.Errorf("stdout missing sequence header")
	}
}

// TestCompose_FailVerdict_Exit1 — at least one phase returning FAIL
// flips overall exit to 1; composition still continues past the fail.
func TestCompose_FailVerdict_Exit1(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{
		scripts: map[string]core.PhaseResponse{
			"scout": {Phase: "scout", Verdict: core.VerdictFAIL},
			"audit": {Phase: "audit", Verdict: core.VerdictPASS},
		},
	}
	registerCompose(t, stub, "scout", "audit")

	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 1})
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "scout,audit"}, bytes.NewReader(envJSON), &stdout, &stderr)
	if code != 1 {
		t.Errorf("code=%d, want 1", code)
	}
	if len(stub.calls) != 2 {
		t.Errorf("expected both phases to run despite first fail; got %d calls", len(stub.calls))
	}
}

// TestCompose_MissingPhasesArg_Exit10 — bad args.
func TestCompose_MissingPhasesArg_Exit10(t *testing.T) {
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	var stdout, stderr bytes.Buffer
	code := runCompose(nil, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
	if !strings.Contains(stderr.String(), "missing --phases") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

// TestCompose_UnknownPhase_Exit10 — phase not in registry rejected.
func TestCompose_UnknownPhase_Exit10(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "scout") // audit not registered
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "scout,nopephase"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
	if !strings.Contains(stderr.String(), "unknown phase") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

// TestCompose_ShipWithoutOverride_Exit2 — refuses to compose ship
// unless --ship-anyway is passed.
func TestCompose_ShipWithoutOverride_Exit2(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "ship")
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "ship"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 2 {
		t.Errorf("code=%d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "ship") {
		t.Errorf("stderr should mention ship; got %q", stderr.String())
	}
}

// TestCompose_ShipWithOverride_Proceeds — --ship-anyway lets it through.
func TestCompose_ShipWithOverride_Proceeds(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "ship")
	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 1})
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "ship", "--ship-anyway"}, bytes.NewReader(envJSON), &stdout, &stderr)
	if code != 0 {
		t.Errorf("code=%d, want 0; stderr=%s", code, stderr.String())
	}
	if len(stub.calls) != 1 || stub.calls[0] != "ship" {
		t.Errorf("expected ship to run; got %v", stub.calls)
	}
}

// TestCompose_DryRun_NoExecution — --dry-run prints the plan and exits.
func TestCompose_DryRun_NoExecution(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "scout", "audit")
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "scout,audit", "--dry-run"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d, want 0", code)
	}
	if len(stub.calls) != 0 {
		t.Errorf("dry-run should not execute phases; got %d calls", len(stub.calls))
	}
	if !strings.Contains(stdout.String(), "DRY-RUN") {
		t.Errorf("stdout missing DRY-RUN marker")
	}
}

// TestCompose_ExportsComposeSignal — PhaseRequest.ComposePhases is true
// when the factory is called from evolve compose (cycle-10: replaced the
// retired EVOLVE_COMPOSE_PHASES env signal with a DI bool).
func TestCompose_ExportsComposeSignal(t *testing.T) {
	defer registry.SnapshotForTest()()
	var observedDuring bool
	registry.ResetForTesting()
	registry.Register("scout", func(req core.PhaseRequest) core.PhaseRunner {
		observedDuring = req.ComposePhases
		return &composeStubRunner{stub: &composeStub{}, name: "scout"}
	})
	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 1})
	var stdout, stderr bytes.Buffer
	_ = runCompose([]string{"--phases", "scout"}, bytes.NewReader(envJSON), &stdout, &stderr)
	if !observedDuring {
		t.Error("PhaseRequest.ComposePhases must be true during composition")
	}
}

// TestCompose_MalformedJSONStdin_Exit10 — invalid JSON envelope.
func TestCompose_MalformedJSONStdin_Exit10(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "scout")
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "scout"}, strings.NewReader("not-json"), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
}

// TestCompose_EmptyStdin_OK — empty stdin is treated as empty
// PhaseRequest (no JSON parse attempted).
func TestCompose_EmptyStdin_OK(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "scout")
	var stdout, stderr bytes.Buffer
	code := runCompose([]string{"--phases", "scout"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("code=%d, want 0; stderr=%s", code, stderr.String())
	}
}

// TestSplitNonEmptyPhases_DirectUnit — direct unit test.
func TestSplitNonEmptyPhases_DirectUnit(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{",a,,b,", []string{"a", "b"}},
	}
	for _, c := range cases {
		got := splitNonEmptyPhases(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitNonEmptyPhases(%q) len=%d want %d", c.in, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitNonEmptyPhases(%q)[%d]=%q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
