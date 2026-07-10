package core

// post_build_repin_test.go — RED tests (cycle 636, task ship-sha-repin-after-build).
//
// The cycle-514 boot healer auto-repins expected_ship_sha ONLY at boot; a
// legitimate within-version rebuild of go/bin/evolve between boots leaves a frozen
// pin that denied the ship gate on 8 consecutive cycles (625->634,
// SELF_SHA_TAMPERED, repair_outcome=declined). This closes the class: the orchestrator
// re-pins immediately AFTER a successful build phase, reusing the SAME
// provenance-gated primitive the boot healer uses (phaseintegrity.RepinIfDrifted).
//
// Contract the Builder implements (TDD-defined seam; mirrors cmd/evolve's proven
// shipRepinProvenanceFn boot-repin seam so the decision stays git-free/
// deterministic under test):
//
//	// postBuildRepinProvenanceFn resolves the running binary's build-commit + the
//	// provenance predicate authorizing a post-build auto-repin. Production:
//	// version.Commit() + a `git merge-base --is-ancestor <commit> HEAD` closure
//	// over projectRoot — identical to cmd/evolve's defaultShipRepinProvenance.
//	var postBuildRepinProvenanceFn = defaultPostBuildRepinProvenance
//	func defaultPostBuildRepinProvenance(projectRoot string) (commit string, prov phaseintegrity.ProvenanceVerified)
//
//	// repinShipSHAAfterBuild re-pins <projectRoot>/.evolve/state.json:expected_ship_sha
//	// to the freshly-built <projectRoot>/go/bin/evolve after a successful build, via
//	// phaseintegrity.RepinIfDrifted(statePath, binPath, commit, "", prov). NEVER
//	// operator-authorized (unattended). Fail-open: a refusal/error WARNs and returns
//	// a zero RepinResult; the ship gate stays the backstop. Wire it into
//	// recordAndBranch's `next == PhaseBuild` branch (see test-report.md WIR-1 checklist).
//	func repinShipSHAAfterBuild(projectRoot string) phaseintegrity.RepinResult
//
// RED now: repinShipSHAAfterBuild + postBuildRepinProvenanceFn undefined -> package
// core test build fails. Do NOT modify this file — implement the seam + wire it.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
)

func pbSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// pbSetupProject lays down <root>/go/bin/evolve (with binBytes) and
// <root>/.evolve/state.json (with the given pin), returning (projectRoot, binPath,
// statePath). A nil binBytes ⇒ no binary written.
func pbSetupProject(t *testing.T, pin string, binBytes []byte) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	binPath := filepath.Join(root, "go", "bin", "evolve")
	if binBytes != nil {
		if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binPath, binBytes, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(evolveDir, "state.json")
	state := map[string]any{"lastCycleNumber": 636}
	if pin != "" {
		state["expected_ship_sha"] = pin
	}
	b, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(statePath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, binPath, statePath
}

func pbReadPin(t *testing.T, statePath string) string {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	s, _ := st["expected_ship_sha"].(string)
	return s
}

func withVerifiedProvenance(t *testing.T, verified bool) {
	t.Helper()
	prev := postBuildRepinProvenanceFn
	t.Cleanup(func() { postBuildRepinProvenanceFn = prev })
	postBuildRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "post-build-commit", func(string) bool { return verified }
	}
}

// AC-1 (the named RED test): a legitimate in-version rebuild that changed
// go/bin/evolve — provenance VERIFIED — is re-pinned AFTER the build (not only at
// boot). expected_ship_sha now tracks the freshly-built binary, so the frozen-pin
// cascade cannot recur.
func TestBootRecovery_RepinsAfterBuildNotJustBoot(t *testing.T) {
	binBytes := []byte("\x7fELF-rebuilt-this-cycle-within-version-22.0.1")
	root, _, statePath := pbSetupProject(t, "STALE_PIN_FROM_A_PRIOR_BINARY", binBytes)
	withVerifiedProvenance(t, true)

	res := repinShipSHAAfterBuild(root)

	if !res.Repinned {
		t.Fatalf("a provenance-verified in-version rebuild must re-pin AFTER build; res=%+v", res)
	}
	wantSHA := pbSHA256Hex(binBytes)
	if got := pbReadPin(t, statePath); got != wantSHA {
		t.Errorf("expected_ship_sha after post-build repin = %q, want the freshly-built binary sha %q", got, wantSHA)
	}
}

// AC-3 (verify-only no-op cycle ships): the mechanical proof that after the
// post-build repin the ship gate's self-SHA check sees NO mismatch — i.e. a
// verify-only cycle on the freshly-rebuilt binary would not be denied
// SELF_SHA_TAMPERED. Uses the exact detector the ship gate uses (ShipSHAMismatch).
func TestBootRecovery_AfterBuildRepin_ShipGateSeesNoMismatch(t *testing.T) {
	binBytes := []byte("\x7fELF-fresh-binary-for-verify-only-cycle")
	root, binPath, _ := pbSetupProject(t, "STALE_PIN", binBytes)
	withVerifiedProvenance(t, true)

	if res := repinShipSHAAfterBuild(root); !res.Repinned {
		t.Fatalf("precondition: post-build repin must fire; res=%+v", res)
	}
	newPin := pbReadPin(t, filepath.Join(root, ".evolve", "state.json"))
	mismatch, actual, err := ShipSHAMismatch(binPath, newPin)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch {
		t.Errorf("after post-build repin the ship gate still sees a SHA mismatch (pin=%q on-disk=%q) — the cascade is not fixed", newPin, actual)
	}
}

// AC-2 (twin / anti-tamper at the wiring layer): a provenance-UNVERIFIED binary
// change is NOT re-pinned post-build — the pin is left untouched and tamper
// detection is preserved. The fix must not weaken the trust boundary.
func TestBootRecovery_PostBuildRepin_UnverifiedProvenance_KeepsPin(t *testing.T) {
	const pin = "TRUSTED_PIN_DO_NOT_TOUCH"
	root, _, statePath := pbSetupProject(t, pin, []byte("\x7fELF-UNTRUSTED-post-build"))
	withVerifiedProvenance(t, false)

	res := repinShipSHAAfterBuild(root)

	if res.Repinned {
		t.Errorf("an UNVERIFIED post-build binary must NOT be re-pinned — anti-tamper must hold; res=%+v", res)
	}
	if got := pbReadPin(t, statePath); got != pin {
		t.Errorf("expected_ship_sha must be UNCHANGED on unverified provenance; got %q want %q", got, pin)
	}
}

// Edge: a project with no built binary yet (or no pin) is a fail-open no-op —
// never a panic — so the post-build hook is safe on every cycle, including ones
// that never rebuilt the binary.
func TestBootRecovery_PostBuildRepin_NoBinaryIsNoOp(t *testing.T) {
	root, _, statePath := pbSetupProject(t, "SOME_PIN", nil) // no binary written
	withVerifiedProvenance(t, true)

	res := repinShipSHAAfterBuild(root)

	if res.Repinned {
		t.Errorf("no built binary ⇒ nothing to re-pin; res=%+v", res)
	}
	if got := pbReadPin(t, statePath); got != "SOME_PIN" {
		t.Errorf("pin must be untouched when there is no binary; got %q", got)
	}
}
