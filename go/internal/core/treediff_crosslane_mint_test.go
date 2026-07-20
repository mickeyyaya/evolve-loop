//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
)

// --- cycle-967: fleet cross-lane mint false-abort ---
// (fix spec: treediff-967-crosslane-mint-fix-spec, Variant A2)
//
// Both fleet lanes run `evolve cycle run` against the SHARED project root.
// The advisor mint (phaseregistrar.Registrar) persists a minted phase's
// config to the shared .evolve/phases/<name>/phase.json, while the per-phase
// tree-diff guard diffs that same shared tree against a PER-LANE baseline —
// so lane A's mint landing during lane B's phase is charged to lane B.
// Cycle-967's PASS scout was aborted on concurrent lane-970's
// .evolve/phases/gate-wiring-proof/phase.json mint.
//
// Fix: the registrar records minted names in the shared mintregistry BEFORE
// persisting files; the guard exempts a leaked .evolve/phases/<name> path IFF
// <name> is a registered, fresh mint. An UNREGISTERED phase-config write must
// still abort — the deliverable-leak loophole TestIsScoutEvalMaterialization
// pins against stays closed (TestGuardStillAbortsUnregisteredPhaseConfigLeak
// holds both before and after the fix).

// writeMintSpec persists a phase spec JSON at root/.evolve/phases/<name>/phase.json
// exactly as the registrar's persist step would (normalized form: optional user
// spec). body overrides the default valid spec when non-empty.
func writeMintSpec(t *testing.T, root, name, body string) {
	t.Helper()
	if body == "" {
		body = `{"name":"` + name + `","optional":true}`
	}
	dir := filepath.Join(root, ".evolve", "phases", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runScoutLeakCycle runs a cycle where scout leaks leakPath into the main
// tree and returns RunCycle's error, sharing the NewOrchestrator + RunCycle
// scaffolding every composed guard test below needs. Callers set up the
// registry/spec fixtures beforehand and assert on the returned error.
func runScoutLeakCycle(t *testing.T, root, leakPath string) error {
	t.Helper()
	dirty := &fakeGitDirty{baseline: []string{}, afterLeak: []string{leakPath}}
	runners := minimalRunners(PhaseScout, &leakInjector{name: PhaseScout})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
		WithGitDirtyPaths(dirty.Fn()),
	)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	return err
}

// TestGuardExemptsConcurrentLaneMintedPhaseConfig is the cycle-967 RED proof:
// a concurrent lane's REGISTERED mint appearing in this lane's post-scout
// diff must not abort the cycle. The on-disk spec is present in the registrar's
// normalized form — the guard verifies content (clamp parity), not just the
// registry name.
func TestGuardExemptsConcurrentLaneMintedPhaseConfig(t *testing.T) {
	root := t.TempDir()
	const mintLeak = ".evolve/phases/xlane-mint/phase.json"
	// The concurrent lane's registrar registered the mint before its files
	// became visible (Register appends to the registry before persisting).
	if err := mintregistry.Append(mintregistry.Path(root), "xlane-mint", time.Now()); err != nil {
		t.Fatalf("registry append: %v", err)
	}
	writeMintSpec(t, root, "xlane-mint", "")
	err := runScoutLeakCycle(t, root, mintLeak)
	if err != nil && strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("guard must not abort on a concurrent lane's registered mint; got: %v", err)
	}
	// Non-vacuous (mirrors TestGuardIgnoresScoutEvalMaterialization): the
	// cycle must ADVANCE PAST scout's guard to a phase with no runner.
	if err == nil || !strings.Contains(err.Error(), "no runner") {
		t.Fatalf("expected the cycle to advance past scout's guard to a no-runner phase; got: %v", err)
	}
}

// TestGuardStillAbortsUnregisteredPhaseConfigLeak pins the loophole shut: a
// phase-config write whose name is NOT in the mint registry is a deliverable
// leak and must abort, exactly as before the exemption existed.
func TestGuardStillAbortsUnregisteredPhaseConfigLeak(t *testing.T) {
	root := t.TempDir()
	const leak = ".evolve/phases/evil/phase.json"
	// Deliberately NO registry entry for "evil".
	err := runScoutLeakCycle(t, root, leak)
	if err == nil {
		t.Fatal("expected tree-diff abort for unregistered phase-config leak; got nil error")
	}
	if !strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("abort must come from the tree-diff guard; got: %v", err)
	}
	if !strings.Contains(err.Error(), leak) {
		t.Errorf("abort error must name the leaked path %q; got: %v", leak, err)
	}
}

// TestGuardAbortsForgedRegistryEntryWithUnclampedSpec is the adversarial pin
// (security review, Finding 1): a registry ENTRY alone must not exempt — the
// on-disk phase.json must pass the same spec-side clamp the registrar
// enforces (parses, user-floor valid, optional, name matches the dir). A
// rogue agent forging the registry buys nothing beyond what a sanctioned
// mint could already carry; anything unclamped still aborts loudly.
func TestGuardAbortsForgedRegistryEntryWithUnclampedSpec(t *testing.T) {
	const leak = ".evolve/phases/xlane-mint/phase.json"
	cases := []struct {
		label string
		body  string // on-disk phase.json content; "-" = write no file at all
	}{
		{"missing-spec-file", "-"},
		{"garbage-json", "{not json"},
		{"non-optional-spec", `{"name":"xlane-mint","optional":false}`},
		{"name-mismatch", `{"name":"other-phase","optional":true}`},
		{"invalid-kind", `{"name":"xlane-mint","optional":true,"kind":"exec"}`},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			root := t.TempDir()
			if err := mintregistry.Append(mintregistry.Path(root), "xlane-mint", time.Now()); err != nil {
				t.Fatalf("registry append: %v", err)
			}
			if tc.body != "-" {
				writeMintSpec(t, root, "xlane-mint", tc.body)
			}
			err := runScoutLeakCycle(t, root, leak)
			if err == nil || !strings.Contains(err.Error(), "tree-diff") {
				t.Fatalf("guard must abort a registered name whose spec fails the clamp (%s); got: %v", tc.label, err)
			}
		})
	}
}

// TestGuardAbortsCompanionPayloadUnderRegisteredMint: a legitimate mint writes
// EXACTLY phase.json; any companion file smuggled under a registered name is
// a leak and must abort even though the name is registered and clamp-valid.
func TestGuardAbortsCompanionPayloadUnderRegisteredMint(t *testing.T) {
	root := t.TempDir()
	const payload = ".evolve/phases/xlane-mint/payload.sh"
	if err := mintregistry.Append(mintregistry.Path(root), "xlane-mint", time.Now()); err != nil {
		t.Fatalf("registry append: %v", err)
	}
	writeMintSpec(t, root, "xlane-mint", "")
	err := runScoutLeakCycle(t, root, payload)
	if err == nil || !strings.Contains(err.Error(), "tree-diff") || !strings.Contains(err.Error(), payload) {
		t.Fatalf("guard must abort a companion payload under a registered mint; got: %v", err)
	}
}

// TestGuardQuarantinesCorruptRegistryAndStaysArmed (security review, Finding
// 2): a corrupt registry must (a) keep the guard armed — the leak still
// aborts — and (b) be quarantined on the spot so the degraded (exemption-off)
// window is bounded to the one check instead of persisting fleet-wide until
// the next mint.
func TestGuardQuarantinesCorruptRegistryAndStaysArmed(t *testing.T) {
	root := t.TempDir()
	const leak = ".evolve/phases/xlane-mint/phase.json"
	regPath := mintregistry.Path(root)
	if err := os.MkdirAll(filepath.Dir(regPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMintSpec(t, root, "xlane-mint", "")
	err := runScoutLeakCycle(t, root, leak)
	if err == nil || !strings.Contains(err.Error(), "tree-diff") {
		t.Fatalf("corrupt registry must keep the guard armed; got: %v", err)
	}
	if _, statErr := os.Stat(regPath); !os.IsNotExist(statErr) {
		t.Errorf("corrupt registry must be quarantined (renamed away); stat err: %v", statErr)
	}
	matches, _ := filepath.Glob(regPath + ".corrupt-*")
	if len(matches) != 1 {
		t.Errorf("expected exactly one quarantined registry file; got %v", matches)
	}
	// Self-healed: the next read is a clean empty registry, no error.
	names, readErr := mintregistry.ActiveNames(regPath, time.Now())
	if readErr != nil || len(names) != 0 {
		t.Errorf("post-quarantine registry must read empty/no-error; got %v, %v", names, readErr)
	}
}

// TestGuardStillAbortsExpiredMintPhaseConfigLeak: a registered mint older
// than the TTL no longer exempts its path — the registry cannot decay into a
// standing allowlist.
func TestGuardStillAbortsExpiredMintPhaseConfigLeak(t *testing.T) {
	root := t.TempDir()
	const leak = ".evolve/phases/stale-mint/phase.json"
	if err := mintregistry.Append(mintregistry.Path(root), "stale-mint", time.Now().Add(-mintregistry.TTL-time.Minute)); err != nil {
		t.Fatalf("registry append: %v", err)
	}
	err := runScoutLeakCycle(t, root, leak)
	if err == nil || !strings.Contains(err.Error(), "tree-diff") {
		t.Fatalf("expected tree-diff abort for expired mint; got: %v", err)
	}
}

// TestIsActiveMintPhasePath (classifier scope pinning) lives untagged in
// mint_exemption_test.go so it runs in both the default and integration
// test modes.
