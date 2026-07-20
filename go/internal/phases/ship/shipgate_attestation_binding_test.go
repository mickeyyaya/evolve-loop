// shipgate_attestation_binding_test.go — cycle-987 gate-wiring binding tests
// for verifyCommitGateAttestation (commitgate.go), UNGUARDED (default suite).
//
// WHY UNGUARDED (the point of the task): the pre-existing coverage for the
// commit-attestation triangle (TestCommitGate_Manual*Attestation_* in
// commitgate_test.go) sits behind //go:build integration, so a severed
// verifyCommitGateAttestation wire is invisible to plain `go test ./...`.
// These three tests drive the enforcer directly — stale → block, fresh →
// pass, missing → block — in the DEFAULT build suite, so the wire is caught
// by normal CI. They call the real enforcer against a real .commit-gate
// attestation fixture (writeAttestation) and a real git tree (makeRepo /
// treeStateSHA), reusing the untagged helpers in realgit_testhelpers_test.go.
//
// verifyCommitGateAttestation computes computeTreeStateSHA → treestate.SHA →
// sha256(`git diff HEAD`), which is byte-identical to treeStateSHA(t, repo);
// the .commit-gate/ fixture is untracked so it never perturbs that diff (no
// git add is performed by the enforcer). Options.runner() defaults to
// sysexec.DefaultRunner when unset, so a bare Options{ProjectRoot: repo} runs
// real git in the fixture repo.

package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestShipGate_StaleAttestationBlocked — an attestation bound to a DIFFERENT
// tree than the one that would be committed is stale: verifyCommitGateAttestation
// must refuse with core.CodeCommitGateStale.
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

// TestShipGate_FreshAttestationPasses — the positive twin: an attestation whose
// tree_state_sha matches the staged tree passes (nil error), proving the gate
// is a real comparison and not an unconditional refuse.
func TestShipGate_FreshAttestationPasses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nreviewed change\n")
	// git diff HEAD is identical whether staged or not, and .commit-gate/ is
	// untracked, so this SHA matches what the enforcer computes.
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

// TestShipGate_MissingAttestationBlocked — the negative/edge twin: no
// .commit-gate/attestation.json at all ⇒ core.CodeCommitGateMissing.
func TestShipGate_MissingAttestationBlocked(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nunreviewed change\n")
	// No writeAttestation call: the attestation file is absent.

	opts := &Options{Class: ClassManual, ProjectRoot: repo}
	res := &RunResult{}
	err := verifyCommitGateAttestation(context.Background(), opts, res)
	wantShipErr(t, err, core.CodeCommitGateMissing, core.ShipClassConfig, "missing")
}
