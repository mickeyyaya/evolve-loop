//go:build acs

// Package cycle1310 materialises the cycle-1310 acceptance criteria for the one
// fleet-scoped item pinned to this lane: `phase-mint-carries-select-metadata`
// (scout tasks `mint-default-select-metadata` + `mint-live-path-wiring-proof`).
//
// The defect class. phaseregistrar.Registrar.Register persists a minted
// phase.json + profile stub built verbatim from the advisor-supplied
// PhaseConfig, so a config with empty Description/WhenToUse mints a stub that —
// once swept into git tracking by ship's whole-tree bind — fails the repo-wide
// guard TestPhaseCatalog_OptionalPhasesHaveSelectMetadata, and a config with an
// empty Dispatch.CLI mints a driverless profile that fails
// TestSmoke_RealProfiles / TestRepoPersonaProfilePairing and dies later at
// dispatch preflight. Four live instances (#399, #404, #406, #407) were each
// fixed by hand; this cycle closes the class at the mint seam.
//
// Predicate strategy — every predicate DRIVES the live seam
// (phaseregistrar.Registrar.Register, the real function the orchestrator's mint
// path calls) and asserts on its return value, its error, or the artifact it
// actually wrote to disk. No predicate greps registrar.go for a magic string
// (the cycle-85 degenerate-predicate ban):
//
//   - 001 mints with empty Description AND WhenToUse and asserts the RETURNED
//     spec no longer satisfies the guard's missing-metadata condition, and that
//     the PERSISTED phase.json carries the same metadata (a default applied only
//     in memory would still write a contract-breaking stub to disk).
//   - 002 is the negative/anti-no-op predicate: an empty Dispatch.CLI must be
//     REJECTED (non-nil error) and NOTHING may be persisted — a driverless
//     profile stub is the literal #406 failure.
//   - 003 is the wiring proof named in the inbox item: mint an unknown phase
//     name through the LIVE Register into a temp project root, then reload it
//     through phasespec.MergedCatalog — the exact loader the guard test uses —
//     and apply the guard's own condition to the merged catalog. Passing on the
//     returned spec alone (001) would not prove the round-trip through disk +
//     the real catalog loader stays guard-green.
//   - 004 asserts the wiring-proof TEST itself exists in the normal (untagged)
//     suite and passes: `go test -run TestRegister_UnknownPhaseNameStaysCatalogGreen
//     ./internal/phaseregistrar`. ACS predicates are cycle-scoped; the durable
//     regression lives in registrar_test.go, so its absence is a failed AC.
//   - 005 is the semantic/edge predicate: the default must fire only when BOTH
//     metadata fields are empty (mirroring the guard's own OR-condition), so an
//     advisor-supplied Description survives verbatim and is never clobbered.
package cycle1310

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// mintedName is the unknown phase name every predicate mints. It is absent from
// docs/architecture/phase-registry.json and from the metadataAllowlist in
// phasespec/catalog_metadata_test.go, so a metadata-less mint of this name is
// exactly the "NEW optional phase with no metadata" the guard is built to fail.
// Letters-only and multi-word: phasespec.twoTierNameRE (`^[a-z]+(-[a-z]+)+$`)
// rejects digits, so a cycle-numbered name would fail the floor, not the AC.
const mintedName = "mint-metadata-probe"

// fakeBridge is the launch seam stub: Register constructs a specrunner over it
// but never launches during registration.
type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}
func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

// newRegistrar returns a Registrar persisting into t.TempDir(), i.e. the real
// Register/persist path with no shared-tree side effects.
func newRegistrar(t *testing.T) phaseregistrar.Registrar {
	t.Helper()
	base := t.TempDir()
	return phaseregistrar.Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}),
		ProfilesDir: filepath.Join(base, "profiles"),
		PhasesDir:   filepath.Join(base, "phases"),
	}
}

// metadataLessCfg is the hazard shape: a well-formed, in-envelope minted phase
// whose advisor-supplied Description and WhenToUse are BOTH empty.
func metadataLessCfg() phaseconfig.PhaseConfig {
	return phaseconfig.PhaseConfig{
		PhaseSpec: phasespec.PhaseSpec{Name: mintedName},
		Dispatch: phaseconfig.Dispatch{
			CLI:               "claude",
			AllowedCLIs:       []string{"claude", "codex"},
			ModelTierDefault:  "balanced",
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"},
		},
		Prompt: "You are a probe.",
	}
}

// guardTripped is the EXACT condition
// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata applies to every catalog
// entry (phasespec/catalog_metadata_test.go:58). Restated here so the predicate
// gates on the guard's real semantics, not a paraphrase.
func guardTripped(s phasespec.PhaseSpec) bool {
	return s.Optional && s.WhenToUse == "" && s.Description == ""
}

// TestC1310_001_MintDefaultsSelectMetadata drives the live Register with an
// empty-metadata config and asserts BOTH the returned spec and the persisted
// phase.json carry SELECT metadata.
func TestC1310_001_MintDefaultsSelectMetadata(t *testing.T) {
	r := newRegistrar(t)

	res, err := r.Register(metadataLessCfg())
	if err != nil {
		t.Fatalf("Register(metadata-less cfg) = %v; a minted phase with no advisor metadata must still register (defaulted, not rejected)", err)
	}
	if guardTripped(res.Spec) {
		t.Errorf("returned spec still trips the catalog guard: Optional=%v Description=%q WhenToUse=%q; Register must default SELECT metadata at mint time",
			res.Spec.Optional, res.Spec.Description, res.Spec.WhenToUse)
	}

	// In-memory defaulting is not enough: the file swept into git is the one
	// that fails CI, so assert the PERSISTED spec too.
	raw, err := os.ReadFile(filepath.Join(r.PhasesDir, mintedName, "phase.json"))
	if err != nil {
		t.Fatalf("read persisted phase.json: %v", err)
	}
	var onDisk phasespec.PhaseSpec
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse persisted phase.json: %v", err)
	}
	if guardTripped(onDisk) {
		t.Errorf("persisted phase.json still trips the catalog guard: Description=%q WhenToUse=%q (raw=%s)", onDisk.Description, onDisk.WhenToUse, string(raw))
	}
}

// TestC1310_002_DriverlessMintRejected is the negative predicate: an empty
// Dispatch.CLI mints a profile with no known driver — the #406 / instance-4
// failure — and must be rejected before anything is written.
func TestC1310_002_DriverlessMintRejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Dispatch.CLI = "" // no driver

	_, err := r.Register(cfg)
	if err == nil {
		t.Fatal("Register(empty Dispatch.CLI) = nil error; a driverless profile stub must be rejected at mint time, not persisted")
	}

	if _, statErr := os.Stat(filepath.Join(r.ProfilesDir, mintedName+".json")); statErr == nil {
		t.Error("driverless profile persisted despite rejection")
	}
	if _, statErr := os.Stat(filepath.Join(r.PhasesDir, mintedName, "phase.json")); statErr == nil {
		t.Error("phase spec persisted despite a rejected driverless mint")
	}
}

// TestC1310_003_MintedPhaseStaysCatalogGreen is the wiring proof: mint through
// the LIVE Register into a temp project root, then reload via the real
// phasespec.MergedCatalog (the guard test's own loader) and apply the guard
// condition to the merged catalog.
func TestC1310_003_MintedPhaseStaysCatalogGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	proj := t.TempDir()

	// A merged catalog needs the built-in registry; copy the real one so the
	// merge path (builtin + user overlay) is the production one.
	regRel := filepath.Join("docs", "architecture", "phase-registry.json")
	registry, err := os.ReadFile(filepath.Join(root, regRel))
	if err != nil {
		t.Fatalf("read built-in phase registry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(proj, "docs", "architecture"), 0o755); err != nil {
		t.Fatalf("mkdir registry dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, regRel), registry, 0o644); err != nil {
		t.Fatalf("seed built-in phase registry: %v", err)
	}

	r := phaseregistrar.Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}),
		ProfilesDir: filepath.Join(proj, ".evolve", "profiles"),
		PhasesDir:   filepath.Join(proj, ".evolve", "phases"), // the default discovery root
	}
	if _, err := r.Register(metadataLessCfg()); err != nil {
		t.Fatalf("Register: %v", err)
	}

	cat, _, _, err := phasespec.MergedCatalog(proj)
	if err != nil {
		t.Fatalf("MergedCatalog after mint: %v", err)
	}

	var found bool
	for _, s := range cat.All() {
		if s.Name != mintedName {
			continue
		}
		found = true
		if guardTripped(s) {
			t.Errorf("minted phase %q is in the merged catalog WITHOUT select metadata — TestPhaseCatalog_OptionalPhasesHaveSelectMetadata would fail on it (Description=%q WhenToUse=%q)", s.Name, s.Description, s.WhenToUse)
		}
	}
	if !found {
		t.Fatalf("minted phase %q is absent from the merged catalog; the mint did not reach the real discovery root %s", mintedName, r.PhasesDir)
	}
}

// TestC1310_004_DurableWiringProofTestExists asserts the regression test named
// in the inbox item's `fix` field lives in the NORMAL suite and passes. ACS
// predicates die with the cycle; this is what keeps the class closed after it.
func TestC1310_004_DurableWiringProofTestExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "-run", "TestRegister_UnknownPhaseNameStaysCatalogGreen", "./internal/phaseregistrar")
	cmd.Dir = filepath.Join(root, "go") // never rely on process cwd (worktree vs main tree)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -run TestRegister_UnknownPhaseNameStaysCatalogGreen ./internal/phaseregistrar failed: %v\n%s", err, out)
	}
	// A missing test exits 0 with this warning — the silent-pass trap.
	if strings.Contains(string(out), "no tests to run") {
		t.Fatalf("TestRegister_UnknownPhaseNameStaysCatalogGreen does not exist in internal/phaseregistrar (go test matched nothing):\n%s", out)
	}
}

// TestC1310_005_AdvisorMetadataPreserved pins the defaulting boundary: the guard
// condition is an AND over both fields, so the default must fire only when BOTH
// are empty. An advisor-supplied Description must survive verbatim.
func TestC1310_005_AdvisorMetadataPreserved(t *testing.T) {
	const want = "Reviews the diff for envelope escapes."
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Description = want // WhenToUse stays empty: guard already satisfied

	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Spec.Description != want {
		t.Errorf("advisor Description clobbered: got %q, want %q", res.Spec.Description, want)
	}

	raw, err := os.ReadFile(filepath.Join(r.PhasesDir, mintedName, "phase.json"))
	if err != nil {
		t.Fatalf("read persisted phase.json: %v", err)
	}
	var onDisk phasespec.PhaseSpec
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse persisted phase.json: %v", err)
	}
	if onDisk.Description != want {
		t.Errorf("persisted Description = %q, want the advisor's %q", onDisk.Description, want)
	}
}
