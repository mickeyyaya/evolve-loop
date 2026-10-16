package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
)

func TestDefaultBootRecovery_AutoRepinsWhenProvenanceVerified(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("\x7fELF-legitimately-rebuilt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": "stale-pin-from-before-the-rebuild"})

	prev := shipRepinProvenanceFn
	defer func() { shipRepinProvenanceFn = prev }()
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "verified-build-commit", func(string) bool { return true }
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if !res.Healed {
		t.Fatalf("a provenance-verified SHA mismatch must auto-repin at boot; res=%+v stderr=%q", res, stderr.String())
	}
	newExpected := readExpectedShipSHA(t, evolveDir)
	if newExpected == "stale-pin-from-before-the-rebuild" || newExpected == "" {
		t.Fatalf("expected_ship_sha was not re-pinned (still %q)", newExpected)
	}
	mismatch, _, err := core.ShipSHAMismatch(binPath, newExpected)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch {
		t.Errorf("after auto-repin, expected_ship_sha (%q) must match the on-disk binary — the cascade is not fixed", newExpected)
	}
}

func TestDefaultBootRecovery_DeclinesRepinWhenProvenanceUnverified(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("\x7fELF-UNTRUSTED-tampered-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const pin = "trusted-pin-do-not-touch"
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": pin})

	prev := shipRepinProvenanceFn
	defer func() { shipRepinProvenanceFn = prev }()
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "unverified-commit", func(string) bool { return false }
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if res.Healed {
		t.Errorf("an UNVERIFIABLE binary must NOT be auto-repinned — anti-tamper must hold; res=%+v", res)
	}
	if !res.SHAMismatch {
		t.Errorf("the mismatch must still be flagged (blocked, not silently healed); res=%+v", res)
	}
	if got := readExpectedShipSHA(t, evolveDir); got != pin {
		t.Errorf("expected_ship_sha must be UNCHANGED on unverified provenance; got %q want %q", got, pin)
	}
}

func TestDefaultBootRecovery_NoExpectedSHAIsNoOpAndSkipsProvenance(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"lastCycleNumber": 3})

	provCalled := false
	prev := shipRepinProvenanceFn
	defer func() { shipRepinProvenanceFn = prev }()
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		provCalled = true
		return "", func(string) bool { return false }
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if res.SHAMismatch || res.Healed {
		t.Errorf("no expected_ship_sha ⇒ nothing to check; want no mismatch/heal, got %+v", res)
	}
	if provCalled {
		t.Errorf("with no expected_ship_sha, boot recovery must short-circuit BEFORE the provenance/git path (no git subprocess)")
	}
}

func readExpectedShipSHA(t *testing.T, evolveDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
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
