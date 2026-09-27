package core

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
