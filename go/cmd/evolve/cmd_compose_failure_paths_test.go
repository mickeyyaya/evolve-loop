package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

type erroringComposeRunner struct{}

func (erroringComposeRunner) Name() string { return "scout" }

func (erroringComposeRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Phase: "scout", Verdict: core.VerdictPASS}, errors.New("bridge died")
}

func composeSpecProject(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("read phase registry: %v", err)
	}
	return composeRegistryProject(t, string(raw))
}

func composeRegistryProject(t *testing.T, registryJSON string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase-registry.json"), []byte(registryJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunCompose_ArgumentErrorsExit10(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"--bogus"}, "flag provided but not defined"},
		{"only separators", []string{"--phases", " , ,, "}, "evolve compose: --phases produced empty list after trimming"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := runCompose(tc.args, strings.NewReader(""), &stdout, &stderr)

			if code != 10 || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("code=%d stderr=%q, want 10 and %q", code, stderr.String(), tc.want)
			}
		})
	}
}

func TestRunCompose_ARefusingRoutingTableStopsBeforeAnyPhase(t *testing.T) {
	root := campaignProject(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "scout")
	req, _ := json.Marshal(core.PhaseRequest{Cycle: 1, ProjectRoot: root})
	var stdout, stderr bytes.Buffer

	code := runCompose([]string{"--phases", "scout"}, bytes.NewReader(req), &stdout, &stderr)

	if code != exitRoutingRefused || !strings.Contains(stderr.String(), "evolve compose: the CLI routing table refuses to route") {
		t.Fatalf("code=%d stderr=%q, want %d and the routing refusal", code, stderr.String(), exitRoutingRefused)
	}
	if len(stub.calls) != 0 {
		t.Errorf("ran %v, want no phase after the refusal", stub.calls)
	}
}

func TestRunCompose_ARunnerErrorFailsTheCompositionButTheRestStillRuns(t *testing.T) {
	defer registry.SnapshotForTest()()
	stub := &composeStub{}
	registerCompose(t, stub, "audit")
	registry.Register("scout", func(core.PhaseRequest) core.PhaseRunner { return erroringComposeRunner{} })
	var stdout, stderr bytes.Buffer

	code := runCompose([]string{"--phases", "scout,audit"}, strings.NewReader(`{"cycle":1}`), &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "[compose] scout ERROR: bridge died") {
		t.Fatalf("code=%d stderr=%q, want 1 and the runner error", code, stderr.String())
	}
	if len(stub.calls) != 1 || stub.calls[0] != "audit" {
		t.Errorf("later phases ran %v, want audit to still run", stub.calls)
	}
	if !strings.Contains(stdout.String(), "[compose] at least one phase did not PASS") {
		t.Errorf("stdout=%q, want the not-all-PASS summary", stdout.String())
	}
}

func TestRunCompose_AnUnresolvablePhaseIsReportedPerPhase(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", composeSpecProject(t))
	cases := []struct {
		name string
		root string
		want string
	}{
		{"catalog cannot load at the request root", t.TempDir(), "[compose] plan-review ERROR: load phase catalog"},
		{"request root knows no such spec", composeRegistryProject(t, `{"schema_version":4,"phases":[]}`), `[compose] plan-review ERROR: evolve compose: unknown phase "plan-review"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := json.Marshal(core.PhaseRequest{Cycle: 1, ProjectRoot: tc.root})
			var stdout, stderr bytes.Buffer

			code := runCompose([]string{"--phases", "plan-review"}, bytes.NewReader(req), &stdout, &stderr)

			if code != 1 || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("code=%d stderr=%q, want 1 and %q", code, stderr.String(), tc.want)
			}
		})
	}
}

func TestComposeRequest_StdinReadFailureExits1(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	req, code := composeRequest(0, "", nil, iotest.ErrReader(errors.New("pipe closed")), &stderr)

	if code != 1 || stderr.String() != "evolve compose: read stdin: pipe closed\n" {
		t.Errorf("code=%d stderr=%q, want 1 and the read failure", code, stderr.String())
	}
	if req.Cycle != 0 || req.ProjectRoot != "" {
		t.Errorf("req=%+v, want the zero request", req)
	}
}

func TestComposeResolveFailure(t *testing.T) {
	t.Parallel()
	if got := composeResolveFailure("x", t.TempDir(), errors.New("boom")); got != "boom" {
		t.Errorf("with an error = %q, want the error text", got)
	}
	got := composeResolveFailure("x", composeRegistryProject(t, `{"schema_version":4,"phases":[]}`), nil)
	if !strings.HasPrefix(got, `evolve compose: unknown phase "x" (known: `) {
		t.Errorf("without an error = %q, want the unknown-phase message", got)
	}
}
