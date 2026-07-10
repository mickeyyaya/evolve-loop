package phaseintegrity

// repin_ifdrifted_test.go — RED tests (cycle 636, task ship-sha-repin-after-build).
//
// Context: SELF_SHA_TAMPERED denied the terminal ship gate on 8 consecutive
// cycles (625->634). The pin was FROZEN (byte-identical expected_ship_sha vs
// on-disk go/bin/evolve every time, both WITHIN plugin version 22.0.1). Root
// cause: the cycle-514 provenance-gated auto-repin runs ONLY at boot; a
// legitimate within-version rebuild that replaces go/bin/evolve without
// repinning leaves every subsequent cycle inheriting a doomed pin.
//
// The fix "extends/duplicates the cycle-514 boot healer's repin path" so it also
// fires after a successful build. To avoid duplicating the detect+provenance-gate+
// repin logic across the boot path (cmd/evolve) and the new post-build path
// (internal/core) — the "never duplicate, centralize" invariant — the Builder
// factors the shared logic into ONE new primitive here:
//
//	// RepinIfDrifted re-pins state.json:expected_ship_sha to the on-disk binary at
//	// binPath WHEN (and only when) its sha256 has drifted from the pin AND the
//	// running binary's build-commit is provenance-verified (the same gate the boot
//	// healer uses via RepinShipSHA). The single shared repin path invoked BOTH at
//	// boot recovery AND immediately after a successful build phase.
//	//   - no pin / binary absent / sha == pin  -> RepinResult{Repinned:false}, nil (no write, prov NOT consulted)
//	//   - drift + provenance-verified           -> RepinShipSHA fires -> RepinResult{Repinned:true, NewSHA: <sha>}
//	//   - drift + provenance-UNVERIFIED          -> RepinResult{Repinned:false} + error; pin left UNTOUCHED
//	func RepinIfDrifted(statePath, binPath, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error)
//
// RED now: RepinIfDrifted is undefined -> package phaseintegrity test build fails.
// Do NOT modify this file — implement the primitive so both repin paths share it.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// sha256HexBytes mirrors core.ShipSHAMismatch's hashing (raw sha256 of the file
// bytes, hex-encoded) so a fixture can seed a pin that either matches or drifts.
func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeRepinFixture lays down a state.json (with the given pin) and a binary blob,
// returning (statePath, binPath). An empty pin ⇒ no expected_ship_sha key.
func writeRepinFixture(t *testing.T, dir, pin string, binBytes []byte) (string, string) {
	t.Helper()
	statePath := filepath.Join(dir, "state.json")
	state := map[string]any{"lastCycleNumber": 7}
	if pin != "" {
		state["expected_ship_sha"] = pin
		state["expected_ship_version"] = "22.0.1"
	}
	b, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(statePath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "evolve")
	if binBytes != nil {
		if err := os.WriteFile(binPath, binBytes, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return statePath, binPath
}

func readPin(t *testing.T, statePath string) string {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("torn/invalid state.json: %v\n%s", err, raw)
	}
	s, _ := st["expected_ship_sha"].(string)
	return s
}

// AC-1 (positive): a legitimate within-version rebuild changed the binary so its
// sha has drifted from the pin; provenance is VERIFIED (build-commit is an
// ancestor of HEAD). RepinIfDrifted must re-pin expected_ship_sha to the new sha.
func TestRepinIfDrifted_ProvenanceVerifiedRebuild_Repins(t *testing.T) {
	dir := t.TempDir()
	binBytes := []byte("\x7fELF-freshly-rebuilt-within-version")
	statePath, binPath := writeRepinFixture(t, dir, "STALE_PIN_FROM_BEFORE_THE_REBUILD", binBytes)

	res, err := RepinIfDrifted(statePath, binPath, "verified-build-commit", "22.0.1",
		func(string) bool { return true })
	if err != nil {
		t.Fatalf("verified-provenance drift must re-pin, got err=%v", err)
	}
	if !res.Repinned {
		t.Fatalf("verified-provenance drift must re-pin; res=%+v", res)
	}
	wantSHA := sha256HexBytes(binBytes)
	if res.NewSHA != wantSHA {
		t.Errorf("NewSHA = %q, want the freshly-built binary sha %q", res.NewSHA, wantSHA)
	}
	if got := readPin(t, statePath); got != wantSHA {
		t.Errorf("expected_ship_sha on disk = %q, want re-pinned to %q", got, wantSHA)
	}
}

// AC-2 (negative / anti-tamper): the binary drifted but provenance is UNVERIFIED
// (possible tampering). RepinIfDrifted must REFUSE — Repinned=false, an error,
// and the pin left byte-for-byte unchanged. A "fix" that re-pins unconditionally
// is a trust-kernel hole and fails here (the strongest anti-no-op signal).
func TestRepinIfDrifted_UnverifiedProvenance_RefusesAndKeepsPin(t *testing.T) {
	dir := t.TempDir()
	const pin = "TRUSTED_PIN_DO_NOT_TOUCH"
	statePath, binPath := writeRepinFixture(t, dir, pin, []byte("\x7fELF-UNTRUSTED-tampered"))

	res, err := RepinIfDrifted(statePath, binPath, "unverified-commit", "22.0.1",
		func(string) bool { return false })
	if err == nil {
		t.Errorf("unverified-provenance drift must return an error (refusal), got nil")
	}
	if res.Repinned {
		t.Errorf("unverified-provenance drift must NOT re-pin — anti-tamper must hold; res=%+v", res)
	}
	if got := readPin(t, statePath); got != pin {
		t.Errorf("expected_ship_sha must be UNCHANGED on unverified provenance; got %q want %q", got, pin)
	}
}

// AC-3 (edge / no-op): the pin already equals the on-disk binary's sha — no drift.
// RepinIfDrifted must be a clean no-op (Repinned=false, nil error) and must NOT
// consult provenance at all (nothing to authorize), asserted via a prov spy.
func TestRepinIfDrifted_NoDrift_IsNoOp(t *testing.T) {
	dir := t.TempDir()
	binBytes := []byte("\x7fELF-already-pinned")
	pin := sha256HexBytes(binBytes)
	statePath, binPath := writeRepinFixture(t, dir, pin, binBytes)

	provCalled := false
	res, err := RepinIfDrifted(statePath, binPath, "commit", "22.0.1",
		func(string) bool { provCalled = true; return true })
	if err != nil {
		t.Fatalf("no-drift no-op must not error, got %v", err)
	}
	if res.Repinned {
		t.Errorf("no drift ⇒ nothing to re-pin; res=%+v", res)
	}
	if provCalled {
		t.Errorf("no drift ⇒ RepinIfDrifted must short-circuit BEFORE consulting provenance")
	}
	if got := readPin(t, statePath); got != pin {
		t.Errorf("pin must be untouched on a no-op; got %q want %q", got, pin)
	}
}

// AC-4 (edge): a project with NO expected_ship_sha (fresh / never pinned) — or a
// missing binary — is a no-op, never a panic, and never consults provenance.
func TestRepinIfDrifted_MissingPinOrBinary_IsNoOp(t *testing.T) {
	// (a) no pin yet.
	dir := t.TempDir()
	statePath, binPath := writeRepinFixture(t, dir, "", []byte("\x7fELF-bytes"))
	provCalled := false
	res, err := RepinIfDrifted(statePath, binPath, "commit", "", func(string) bool { provCalled = true; return true })
	if err != nil || res.Repinned || provCalled {
		t.Errorf("no expected_ship_sha ⇒ no-op, no provenance; res=%+v err=%v provCalled=%v", res, err, provCalled)
	}

	// (b) binary absent (nothing to hash/compare).
	dir2 := t.TempDir()
	statePath2, _ := writeRepinFixture(t, dir2, "SOME_PIN", nil)
	res2, err2 := RepinIfDrifted(statePath2, filepath.Join(dir2, "evolve"), "commit", "", func(string) bool { return true })
	if err2 != nil || res2.Repinned {
		t.Errorf("absent binary ⇒ no-op (no mismatch signal), got res=%+v err=%v", res2, err2)
	}
	if got := readPin(t, statePath2); got != "SOME_PIN" {
		t.Errorf("absent binary must leave the pin untouched; got %q", got)
	}
}
