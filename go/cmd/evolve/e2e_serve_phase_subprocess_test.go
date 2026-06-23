//go:build e2e && evolve_test_phases

// End-to-end proof that phaseproto.SubprocessRunner can drive a real
// `evolve serve-phase` subprocess. Closes the "cross-CLI parity
// hardening" gate from Phase 3 task #17 (progress doc, sub-bullet 1).
//
// Strategy: build the evolve binary with -tags evolve_test_phases so a
// test-only `echo` phase is registered. The wire path under test is:
//
//	test goroutine
//	  └── phaseproto.SubprocessRunner.Run
//	         └── exec.CommandContext("evolve", "serve-phase", "echo")
//	                └── dispatch -> runServePhase
//	                       └── phaseproto.ServeStdio
//	                              └── echoPhaseRunner.Run
//	         (response envelope flows back up)
//
// No real phase work, no Claude CLI required.
package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/pkg/phaseproto"
)

// buildEvolveBinaryWithTestPhases compiles the evolve binary into a
// temp dir with the evolve_test_phases build tag. Skips if `go` is not
// on PATH (e.g. some hermetic CI sandboxes).
func buildEvolveBinaryWithTestPhases(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go not on PATH: %v", err)
	}
	out := filepath.Join(t.TempDir(), "evolve")
	repoRoot := mustRepoRoot(t)
	cmd := exec.Command("go", "build", "-tags", "evolve_test_phases", "-o", out, "./cmd/evolve")
	cmd.Dir = filepath.Join(repoRoot, "go")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags evolve_test_phases: %v\n%s", err, combined)
	}
	return out
}

func TestServePhase_SubprocessRunnerRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E subprocess test; skipped in -short mode")
	}
	bin := buildEvolveBinaryWithTestPhases(t)

	runner := phaseproto.NewSubprocessRunner(
		"echo",
		bin,
		[]string{"serve-phase", "echo"},
		nil, // no extra env
	)

	req := core.PhaseRequest{
		Cycle:       42,
		ProjectRoot: "/tmp/p",
		Workspace:   "/tmp/w",
		GoalHash:    "deadbeef",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := runner.Run(ctx, req)
	if err != nil {
		t.Fatalf("SubprocessRunner.Run: %v", err)
	}

	if resp.Phase != "echo" {
		t.Errorf("Phase=%q want echo", resp.Phase)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q want PASS", resp.Verdict)
	}
	// Round-trip integrity: the echo phase encodes Cycle into
	// ArtifactsDir. If the wire ate the request, this fails.
	want := "cycle-42"
	if resp.ArtifactsDir != want {
		t.Errorf("ArtifactsDir=%q want %q — request lost on wire", resp.ArtifactsDir, want)
	}
}

// TestServePhase_SubprocessRunner_UnknownPhaseSurfacesError confirms
// the "unknown phase" exit path produces a CodeChildCrashed wire error
// (exit 10 from the binary), not a parse error or hang.
func TestServePhase_SubprocessRunner_UnknownPhaseSurfacesError(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E subprocess test; skipped in -short mode")
	}
	bin := buildEvolveBinaryWithTestPhases(t)

	runner := phaseproto.NewSubprocessRunner(
		"nope",
		bin,
		[]string{"serve-phase", "nope-phase"},
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := runner.Run(ctx, core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("expected error from unknown phase, got nil")
	}
	werr, ok := err.(*phaseproto.WireError)
	if !ok {
		t.Fatalf("want *phaseproto.WireError, got %T: %v", err, err)
	}
	if werr.Code != phaseproto.CodeChildCrashed {
		t.Errorf("WireError.Code=%q want %q", werr.Code, phaseproto.CodeChildCrashed)
	}
}
