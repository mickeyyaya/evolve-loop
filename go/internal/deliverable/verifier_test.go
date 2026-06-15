package deliverable

// verifier_test.go — ADR-0045 I2: the rung re-check is BREAKER-NEUTRAL
// (§8 TestLadder_RungRechecksAreBreakerNeutral). White-box (package
// deliverable) to drive the Reviewer's breakerPath override beside the
// Verifier on identical inputs.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestLadder_RungRechecksAreBreakerNeutral — same violating deliverable, two
// paths: Reviewer.Review (the gate) increments the persistent breaker;
// Verifier.VerifyDeliverable (the rung re-check) must leave it untouched, no
// matter how many times a multi-rung repair re-checks.
func TestLadder_RungRechecksAreBreakerNeutral(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: root}
	breaker := filepath.Join(root, ".evolve", "contract-gate-breaker.json")

	v := &Verifier{resolver: phasecontract.BuiltinResolver{}}
	for i := 0; i < 3; i++ {
		res, err := v.VerifyDeliverable(context.Background(), in)
		if err != nil {
			t.Fatalf("VerifyDeliverable: %v", err)
		}
		if res.OK {
			t.Fatal("missing artifact must verify !OK")
		}
	}
	if _, err := os.Stat(breaker); !os.IsNotExist(err) {
		t.Fatalf("THREE rung re-checks must not create/touch the breaker (stat err=%v) — one flaky deliverable would demote the gate batch-wide", err)
	}

	// Control: the GATE on the same input does count.
	r := newReviewer(config.StageEnforce, phasecontract.BuiltinResolver{}, config.StageOff)
	r.breakerPath = breaker
	if rr := r.Review(context.Background(), in); rr.Approve {
		t.Fatal("control: enforce gate must block the missing artifact")
	}
	if _, err := os.Stat(breaker); err != nil {
		t.Fatalf("control: the gate must persist the breaker count: %v", err)
	}
}

// TestVerifier_ReturnsContractedPathAndViolations — the salvage rung needs
// the CONTRACTED destination (the only path it may relocate to) and the
// violation strings for the evidence digest.
func TestVerifier_ReturnsContractedPathAndViolations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	in := core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: root}

	v := &Verifier{resolver: phasecontract.BuiltinResolver{}}
	res, err := v.VerifyDeliverable(context.Background(), in)
	if err != nil {
		t.Fatalf("VerifyDeliverable: %v", err)
	}
	if res.ArtifactPath == "" || !strings.HasPrefix(res.ArtifactPath, ws) {
		t.Errorf("ArtifactPath must be the contracted destination under the workspace; got %q", res.ArtifactPath)
	}
	if len(res.Violations) == 0 || !strings.Contains(res.Violations[0], "missing_artifact") {
		t.Errorf("violations must carry coded messages; got %v", res.Violations)
	}

	// Unknown phase ⇒ ambiguity ⇒ error (fail-open contract preserved).
	if _, err := v.VerifyDeliverable(context.Background(), core.ReviewInput{Phase: "no-such-phase", Workspace: ws, ProjectRoot: root}); err == nil {
		t.Error("unknown phase must surface the fail-open error, never a silent !OK")
	}
}
