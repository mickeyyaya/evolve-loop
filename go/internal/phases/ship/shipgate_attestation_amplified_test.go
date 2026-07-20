// shipgate_attestation_amplified_test.go — Test Amplifier (cycle 987).
//
// Adversarial additions on top of the TDD-contracted binding tests
// (TestShipGate_{Stale,Fresh,Missing}Attestation{Blocked,Passes} in
// shipgate_attestation_binding_test.go). These probe malformed/empty input,
// the "untracked .commit-gate/ never perturbs the diff" assumption the TDD
// contract states as fact, a time-of-check-to-time-of-use staleness gap, and
// a large-diff scale case. Written black-box against tdd-contract.md /
// build-report.md prose only — verifyCommitGateAttestation's body and the
// Builder's new binding test file were deliberately not read, to avoid
// anchoring test design on the implementation.
package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestShipGate_MalformedAttestation_Blocked: a commit gate that reads an
// untrusted-format file must fail closed (non-nil error), never silently
// treat garbage/empty/incomplete JSON as a valid fresh review.
func TestShipGate_MalformedAttestation_Blocked(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"garbage-not-json", "not json at all {{{"},
		{"empty-file", ""},
		{"missing-tree-sha-field", `{"ts":"2026-05-27T00:00:00Z","checks_passed":[]}` + "\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := makeRepo(t)
			mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nactual change\n")
			mustMkdir(t, filepath.Join(repo, ".commit-gate"))
			mustWrite(t, filepath.Join(repo, ".commit-gate", "attestation.json"), tc.body)

			err := verifyCommitGateAttestation(context.Background(), &Options{ProjectRoot: repo}, &RunResult{})
			if err == nil {
				t.Fatalf("malformed attestation (%s) must not silently pass as fresh; got nil error", tc.name)
			}
		})
	}
}

// TestShipGate_AttestationDirDoesNotPerturbTreeState locks in the assumption
// the TDD contract states as fact ("the enforcer performs no git add, and
// the untracked .commit-gate/ never perturbs the diff"): tree-state SHA
// computed before writing the attestation must equal the SHA computed after,
// and the attestation written against the pre-write SHA must verify fresh.
// If this invariant ever breaks (e.g. .commit-gate/ becomes tracked, or
// treeStateSHA starts including untracked files), this test catches it
// immediately instead of every ship attempt silently looking stale.
func TestShipGate_AttestationDirDoesNotPerturbTreeState(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nactual change\n")

	before := treeStateSHA(t, repo)
	writeAttestation(t, repo, before)
	after := treeStateSHA(t, repo)

	if before != after {
		t.Fatalf("writing .commit-gate/attestation.json perturbed tree-state SHA: before=%s after=%s", before, after)
	}

	if err := verifyCommitGateAttestation(context.Background(), &Options{ProjectRoot: repo}, &RunResult{}); err != nil {
		t.Fatalf("attestation bound to the pre-write tree SHA must verify fresh; got %v", err)
	}
}

// TestShipGate_StaleAfterPostAttestationEdit_Blocked is a
// time-of-check-to-time-of-use case distinct from the contracted
// TestShipGate_StaleAttestationBlocked (which binds attestation to an
// arbitrary constant SHA). Here the attestation is bound to a REAL,
// once-fresh tree state, then the tracked tree is edited again — simulating
// a reviewer approving snapshot A while the author keeps typing into
// snapshot B. The gate must re-derive freshness from current tree state, not
// cache the fact that the attestation was valid at write time.
func TestShipGate_StaleAfterPostAttestationEdit_Blocked(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\napproved change\n")
	approvedSHA := treeStateSHA(t, repo)
	writeAttestation(t, repo, approvedSHA)

	// Reviewed snapshot was fresh at write time.
	if err := verifyCommitGateAttestation(context.Background(), &Options{ProjectRoot: repo}, &RunResult{}); err != nil {
		t.Fatalf("attestation must verify fresh immediately after writing; got %v", err)
	}

	// Author edits again post-approval without re-attesting.
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\napproved change\nunreviewed follow-up\n")

	err := verifyCommitGateAttestation(context.Background(), &Options{ProjectRoot: repo}, &RunResult{})
	wantShipErr(t, err, core.CodeCommitGateStale, core.ShipClassConfig, "")
}

// TestShipGate_LargeDiffFreshAttestationPasses is the large-scale adversarial
// case: a multi-kilobyte tracked change must not defeat SHA computation or
// attestation matching (e.g. via truncation, buffering, or a hard-coded size
// assumption in the diff/hash path).
func TestShipGate_LargeDiffFreshAttestationPasses(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\n"+strings.Repeat("large adversarial payload line\n", 20000))

	sha := treeStateSHA(t, repo)
	writeAttestation(t, repo, sha)

	if err := verifyCommitGateAttestation(context.Background(), &Options{ProjectRoot: repo}, &RunResult{}); err != nil {
		t.Fatalf("large tracked diff with a matching attestation must verify fresh; got %v", err)
	}
}
