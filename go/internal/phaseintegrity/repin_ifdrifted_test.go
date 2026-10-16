package phaseintegrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

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

func TestRepinIfDrifted_MissingPinOrBinary_IsNoOp(t *testing.T) {
	dir := t.TempDir()
	statePath, binPath := writeRepinFixture(t, dir, "", []byte("\x7fELF-bytes"))
	provCalled := false
	res, err := RepinIfDrifted(statePath, binPath, "commit", "", func(string) bool { provCalled = true; return true })
	if err != nil || res.Repinned || provCalled {
		t.Errorf("no expected_ship_sha ⇒ no-op, no provenance; res=%+v err=%v provCalled=%v", res, err, provCalled)
	}

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
