package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestShipGate_StaleAttestationBlocked(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nactual change\n")
	// Attestation bound to some OTHER tree state.
	writeAttestation(t, repo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	opts := &Options{Class: ClassManual, ProjectRoot: repo}
	res := &RunResult{}
	err := verifyCommitGateAttestation(context.Background(), opts, res)
	wantShipErr(t, err, core.CodeCommitGateStale, core.ShipClassConfig, "stale")
}

func TestShipGate_FreshAttestationPasses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nreviewed change\n")
	// .commit-gate/ is untracked, so git diff HEAD is unaffected by writing it.
	writeAttestation(t, repo, treeStateSHA(t, repo))

	opts := &Options{Class: ClassManual, ProjectRoot: repo}
	res := &RunResult{}
	if err := verifyCommitGateAttestation(context.Background(), opts, res); err != nil {
		t.Fatalf("fresh attestation matching the staged tree must pass; got %v (logs=%v)", err, res.Logs)
	}
	if !containsLog(*res, "review attestation verified") {
		t.Errorf("a passing attestation must log verification; logs=%v", res.Logs)
	}
}

func TestShipGate_MissingAttestationBlocked(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nunreviewed change\n")
	// No writeAttestation call: the attestation file is absent.

	opts := &Options{Class: ClassManual, ProjectRoot: repo}
	res := &RunResult{}
	err := verifyCommitGateAttestation(context.Background(), opts, res)
	wantShipErr(t, err, core.CodeCommitGateMissing, core.ShipClassConfig, "missing")
}
