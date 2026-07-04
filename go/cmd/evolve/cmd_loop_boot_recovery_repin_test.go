package main

// cmd_loop_boot_recovery_repin_test.go — RED tests (cycle 514, task
// boot-recovery-auto-repin-shipsha). Cycle 507 wired *detection* of a ship-binary
// SHA mismatch into runLoop's boot path (detectShipSHAMismatch → res.SHAMismatch),
// but it only WARNs — it never invokes the existing, provenance-gated repin
// primitive phaseintegrity.RepinShipSHA. Result: the SELF_SHA_TAMPERED ship
// cascade recurred on cycles 508-513 (nine of the last ~20 cycles). This task
// closes the wiring gap: on a detected mismatch, boot recovery AUTO-REPINS
// expected_ship_sha to the on-disk binary WHEN (and only when) the running
// binary's build-commit is provenance-verified (git ancestor of HEAD) — the
// unattended-boot successor to `evolve reset-sha`. An unverifiable mismatch
// (possible tampering) must still be refused, so the anti-tamper guarantee holds.
//
// Contract the Builder implements (TDD-defined seam; mirrors the established
// bootRecoverFn / runLoopPreflightFn package-var seam idiom):
//
//	type bootRecoveryResult struct { Quarantined, Sealed, SHAMismatch, Healed bool }
//	// Healed == an auto-repin fired (the cascade was self-healed at boot).
//
//	// shipRepinProvenanceFn resolves the build-commit + provenance check used to
//	// authorize a boot-time auto-repin. A seam so boot recovery stays git-free
//	// (deterministic) in tests. Production: version.Commit() + a
//	// `git merge-base --is-ancestor <commit> HEAD` closure over cfg.ProjectRoot,
//	// exactly what runResetSHA (cmd_resetsha.go) uses.
//	var shipRepinProvenanceFn = defaultShipRepinProvenance
//	func defaultShipRepinProvenance(projectRoot string) (commit string, prov phaseintegrity.ProvenanceVerified)
//
//	// defaultBootRecovery: on a detected SHA mismatch, attempt an auto-repin via
//	// phaseintegrity.RepinShipSHA(statePath, actualSHA, commit, "", prov, false)
//	// — NEVER operatorAuthorized=true from an unattended boot. On repin success,
//	// set res.Healed=true. On provenance failure, keep today's warn-only behavior
//	// (res.SHAMismatch stays true; the pin is untouched; the ship gate still blocks).
//
// RED now (Healed field + shipRepinProvenanceFn undefined → package main test
// build fails). Do NOT modify this file — implement the seam.

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

// AC-1 (positive): a provenance-VERIFIED ship-SHA mismatch — a legitimate
// rebuild that changed the binary — is auto-repinned at boot, so the very next
// boot/ship sees NO mismatch (the 508-513 SELF_SHA_TAMPERED cascade is broken).
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
	// expected_ship_sha is stale — a legitimate rebuild changed the binary.
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": "stale-pin-from-before-the-rebuild"})

	// Provenance VERIFIED: the running binary's build-commit is an ancestor of
	// HEAD (legit rebuild). Injected so the decision is git-free/deterministic —
	// git is the only non-deterministic input, exactly why phaseintegrity injects
	// ProvenanceVerified.
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
	// The cascade is ACTUALLY fixed: the pin now equals the on-disk binary sha, so
	// core.ShipSHAMismatch reports no mismatch on the re-pinned state.
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

// AC-2 (negative / anti-tamper): a provenance-UNVERIFIED mismatch — the running
// binary's build-commit is NOT an ancestor of HEAD (possible tampering) — must
// NOT be auto-repinned. The mismatch stays flagged (the ship gate still blocks)
// and the pin is left untouched. This is the strongest anti-no-op signal: a
// "fix" that repins unconditionally would be a trust-kernel hole and fails here.
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

	// Provenance UNVERIFIED — the repin must be refused.
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

// AC-3 (edge): a project with NO expected_ship_sha yet (fresh / never pinned) is
// a no-op — no mismatch, no heal — and boot recovery must NOT reach the
// provenance/git path at all (short-circuit before any git subprocess). Asserted
// via a spy on the provenance resolver: it is the exact seam that would shell out
// to git, so "never called" proves "no git subprocess invoked".
func TestDefaultBootRecovery_NoExpectedSHAIsNoOpAndSkipsProvenance(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// state.json exists but carries NO expected_ship_sha (nothing pinned yet).
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
