//go:build acs

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

const mintedName = "mint-metadata-probe"

type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}
func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

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

func guardTripped(s phasespec.PhaseSpec) bool {
	return s.Optional && s.WhenToUse == "" && s.Description == ""
}

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

func TestC1310_002_DriverlessMintRejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Dispatch.CLI = ""

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

func TestC1310_003_MintedPhaseStaysCatalogGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	proj := t.TempDir()

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
		PhasesDir:   filepath.Join(proj, ".evolve", "phases"),
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

func TestC1310_004_DurableWiringProofTestExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "-run", "TestRegister_UnknownPhaseNameStaysCatalogGreen", "./internal/phaseregistrar")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -run TestRegister_UnknownPhaseNameStaysCatalogGreen ./internal/phaseregistrar failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "no tests to run") {
		t.Fatalf("TestRegister_UnknownPhaseNameStaysCatalogGreen does not exist in internal/phaseregistrar (go test matched nothing):\n%s", out)
	}
}

func TestC1310_005_AdvisorMetadataPreserved(t *testing.T) {
	const want = "Reviews the diff for envelope escapes."
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Description = want

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
