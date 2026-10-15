package phaseregistrar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}
func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func validCfg() phaseconfig.PhaseConfig {
	return phaseconfig.PhaseConfig{
		PhaseSpec: phasespec.PhaseSpec{Name: "minted-reviewer", Optional: true},
		Dispatch: phaseconfig.Dispatch{
			CLI:               "claude",
			AllowedCLIs:       []string{"claude", "codex"},
			ModelTierDefault:  "balanced",
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"},
		},
		Prompt: "You are a reviewer.",
	}
}

func newRegistrar(t *testing.T) Registrar {
	t.Helper()
	return Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}),
		ProfilesDir: filepath.Join(t.TempDir(), "profiles"),
		PhasesDir:   filepath.Join(t.TempDir(), "phases"),
	}
}

func TestRegister_ValidConfig_ReturnsRunnerAndPersists(t *testing.T) {
	r := newRegistrar(t)
	res, err := r.Register(validCfg())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Runner == nil {
		t.Fatal("Register returned nil runner")
	}
	if res.Runner.Name() != "minted-reviewer" {
		t.Errorf("runner Name=%q, want minted-reviewer", res.Runner.Name())
	}
	if !res.Spec.Optional {
		t.Error("normalized spec must be Optional=true")
	}
	if _, err := os.Stat(filepath.Join(r.ProfilesDir, "minted-reviewer.json")); err != nil {
		t.Errorf("expected persisted profile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")); err != nil {
		t.Errorf("expected persisted phase.json: %v", err)
	}
}

func TestRegister_ForcesOptional(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Optional = false
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register must force Optional, not reject: %v", err)
	}
	if !res.Spec.Optional {
		t.Error("Optional not forced true")
	}
}

func TestRegister_TierOutsideEnvelope_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierEnvelope = &profiles.ModelTierEnvelope{Min: "fast", Max: "fast"}
	cfg.Dispatch.ModelTierDefault = "deep"
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: tier deep outside [fast..fast]")
	}
}

func TestRegister_CLINotAllowed_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.AllowedCLIs = []string{"codex"}
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: cli claude not in allowed_clis [codex]")
	}
}

func TestRegister_InvalidName_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Name = "Minted Reviewer"
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: invalid name")
	}
}

func TestRegister_NonKebabDerivedProfileName_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Name = "minted-reviewer"
	cfg.Agent = "evolve-Bad Name"
	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, "must be lowercase kebab-case")
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")) {
		t.Error("phase spec persisted despite a rejected derived profile name")
	}
}

func TestRegister_SourceWriter_GetsSandbox(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.WritesSource = true
	cfg.Dispatch.Sandbox = nil
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(r.ProfilesDir, "minted-reviewer.json"))
	if err != nil {
		t.Fatalf("read persisted profile: %v", err)
	}
	var prof profiles.Profile
	if err := json.Unmarshal(raw, &prof); err != nil {
		t.Fatalf("parse persisted profile: %v", err)
	}
	if prof.Sandbox == nil || !prof.Sandbox.Enabled {
		t.Errorf("source-writer profile must enable sandbox; got %+v", prof.Sandbox)
	}
	if prof.Sandbox != nil && prof.Sandbox.ReadOnlyRepo {
		t.Error("source-writer sandbox must not be read-only")
	}
	if !res.Spec.WritesSource {
		t.Error("WritesSource must be preserved on the normalized spec")
	}
}

func TestRegister_UnclassifiableTierWithEnvelope_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = "gpt-4o-mega"
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: unclassifiable tier with an envelope set")
	}
}

func TestRegister_CLIWithDriverSuffix_ClampedByBase(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.CLI = "claude-tmux"
	cfg.Dispatch.AllowedCLIs = []string{"claude"}
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("claude-tmux should clamp to base claude (allowed); got %v", err)
	}
}

func TestRegister_NoEnvelopeNoAllowed_Passes(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierEnvelope = nil
	cfg.Dispatch.AllowedCLIs = nil
	cfg.Dispatch.ModelTierDefault = "anything-goes"
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("no-restriction minted phase should register; got %v", err)
	}
}

func TestRegister_EmptyDirs_SkipsPersistence(t *testing.T) {
	r := Registrar{Bridge: fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})}
	res, err := r.Register(validCfg())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Runner == nil {
		t.Fatal("expected a runner even when persistence is skipped")
	}
}

func TestRegister_SpecModelHint_DrivesClamp(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = ""
	cfg.Dispatch.ModelTierEnvelope = &profiles.ModelTierEnvelope{Min: "fast", Max: "fast"}
	cfg.Model = "deep"
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: spec model hint deep outside [fast..fast]")
	}
}

func TestRegister_SpecModelHint_InEnvelope_Persists(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = ""
	cfg.Model = "balanced"
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Spec.Model != "balanced" {
		t.Errorf("spec Model=%q, want balanced (preserved)", res.Spec.Model)
	}
	var prof profiles.Profile
	if err := json.Unmarshal([]byte(fixtures.MustRead(t, filepath.Join(r.ProfilesDir, "minted-reviewer.json"))), &prof); err != nil {
		t.Fatalf("parse persisted profile: %v", err)
	}
	if prof.ModelTierDefault != "" {
		t.Errorf("persisted model_tier_default=%q, want empty (dispatch default was unset)", prof.ModelTierDefault)
	}
}

func TestRegister_ProfilePersistFails_PropagatesError(t *testing.T) {
	r := newRegistrar(t)
	fixtures.MustWrite(t, r.ProfilesDir, "i am a file, not a dir")

	_, err := r.Register(validCfg())
	fixtures.RequireErrContains(t, err, "persist profile")
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")) {
		t.Error("phase spec persisted despite profile-write failure")
	}
}

func TestRegister_SpecPersistFails_PropagatesError(t *testing.T) {
	r := newRegistrar(t)
	fixtures.MustWrite(t, r.PhasesDir, "i am a file, not a dir")

	_, err := r.Register(validCfg())
	fixtures.RequireErrContains(t, err, "persist phase spec")
	if !fixtures.FilePresent(filepath.Join(r.ProfilesDir, "minted-reviewer.json")) {
		t.Error("expected the dispatch profile to persist before the spec write failed")
	}
}

func TestRegister_AppendsActiveMintRegistry(t *testing.T) {
	r := newRegistrar(t)
	r.RegistryPath = mintregistry.Path(t.TempDir())
	if _, err := r.Register(validCfg()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	names, err := mintregistry.ActiveNames(r.RegistryPath, time.Now())
	if err != nil {
		t.Fatalf("ActiveNames: %v", err)
	}
	if !names["minted-reviewer"] {
		t.Errorf("registered mint must be in the active-mints registry; got %v", names)
	}
}

func TestRegister_Rejected_NoRegistryEntry(t *testing.T) {
	r := newRegistrar(t)
	r.RegistryPath = mintregistry.Path(t.TempDir())
	cfg := validCfg()
	cfg.Dispatch.AllowedCLIs = []string{"codex"}
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected clamp rejection")
	}
	names, err := mintregistry.ActiveNames(r.RegistryPath, time.Now())
	if err != nil {
		t.Fatalf("ActiveNames: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("a rejected mint must not be registered; got %v", names)
	}
}

func TestRegister_RegistryAppendFails_RejectsBeforePersist(t *testing.T) {
	r := newRegistrar(t)
	base := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(base, "blocker"), "a file, not a dir")
	r.RegistryPath = filepath.Join(base, "blocker", "active-mints.json")
	_, err := r.Register(validCfg())
	fixtures.RequireErrContains(t, err, "mintregistry")
	if fixtures.FilePresent(filepath.Join(r.ProfilesDir, "minted-reviewer.json")) {
		t.Error("profile persisted despite registry failure")
	}
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")) {
		t.Error("phase spec persisted despite registry failure")
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func metadataLessCfg() phaseconfig.PhaseConfig {
	cfg := validCfg()
	cfg.Name = "minted-catalog-probe"
	cfg.Description = ""
	cfg.WhenToUse = ""
	return cfg
}

func guardCondition(s phasespec.PhaseSpec) bool {
	return s.Optional && s.WhenToUse == "" && s.Description == ""
}

func TestRegister_UnknownPhaseNameStaysCatalogGreen(t *testing.T) {
	proj := t.TempDir()

	regRel := filepath.Join("docs", "architecture", "phase-registry.json")
	registry, err := os.ReadFile(filepath.Join(repoRootDir(t), regRel))
	if err != nil {
		t.Fatalf("read built-in phase registry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(proj, "docs", "architecture"), 0o755); err != nil {
		t.Fatalf("mkdir registry dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, regRel), registry, 0o644); err != nil {
		t.Fatalf("seed built-in phase registry: %v", err)
	}

	r := Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}),
		ProfilesDir: filepath.Join(proj, ".evolve", "profiles"),
		PhasesDir:   filepath.Join(proj, ".evolve", "phases"),
	}
	cfg := metadataLessCfg()
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("Register(unknown name, no metadata) must succeed with defaults, not reject: %v", err)
	}

	cat, _, _, err := phasespec.MergedCatalog(proj)
	if err != nil {
		t.Fatalf("MergedCatalog after mint: %v", err)
	}
	var found bool
	for _, s := range cat.All() {
		if s.Name != cfg.Name {
			continue
		}
		found = true
		if guardCondition(s) {
			t.Errorf("minted phase %q reached the merged catalog without SELECT metadata — TestPhaseCatalog_OptionalPhasesHaveSelectMetadata would fail on it (Description=%q WhenToUse=%q)",
				s.Name, s.Description, s.WhenToUse)
		}
	}
	if !found {
		t.Fatalf("minted phase %q is absent from the merged catalog; the mint never reached the discovery root %s", cfg.Name, r.PhasesDir)
	}
}

func TestRegister_MetadataLessMint_DefaultsSelectMetadata(t *testing.T) {
	r := newRegistrar(t)
	cfg := metadataLessCfg()

	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if guardCondition(res.Spec) {
		t.Errorf("returned spec trips the catalog guard: Optional=%v Description=%q WhenToUse=%q",
			res.Spec.Optional, res.Spec.Description, res.Spec.WhenToUse)
	}

	raw, err := os.ReadFile(filepath.Join(r.PhasesDir, cfg.Name, "phase.json"))
	if err != nil {
		t.Fatalf("read persisted phase.json: %v", err)
	}
	var onDisk phasespec.PhaseSpec
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse persisted phase.json: %v", err)
	}
	if guardCondition(onDisk) {
		t.Errorf("persisted phase.json trips the catalog guard: Description=%q WhenToUse=%q (raw=%s)",
			onDisk.Description, onDisk.WhenToUse, string(raw))
	}
}

func TestRegister_AdvisorMetadata_NotClobbered(t *testing.T) {
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
	if res.Spec.WhenToUse != "" {
		t.Errorf("WhenToUse defaulted despite a non-empty Description: got %q", res.Spec.WhenToUse)
	}
}

func TestRegister_DriverlessCLI_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Dispatch.CLI = ""

	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, "driverless profile stub")
	if fixtures.FilePresent(filepath.Join(r.ProfilesDir, cfg.Name+".json")) {
		t.Error("driverless profile persisted despite rejection")
	}
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, cfg.Name, "phase.json")) {
		t.Error("phase spec persisted despite a rejected driverless mint")
	}
}

func TestRegister_DriverlessCLI_AllowedWithoutPersistence(t *testing.T) {
	r := Registrar{Bridge: fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})}
	cfg := metadataLessCfg()
	cfg.Dispatch.CLI = ""

	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("a persistence-free registration of a dispatch-less phase must succeed; got %v", err)
	}
	if res.Runner == nil {
		t.Fatal("expected a runner")
	}
}
