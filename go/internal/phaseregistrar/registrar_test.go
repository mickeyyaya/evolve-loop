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

// validCfg is a minimal, in-envelope minted phase carrying an inline prompt.
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
		Prompts:     prompts.NewFromFS(fstest.MapFS{}), // empty: inline prompt only
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
	// Dispatch profile persisted so the runner resolves cli/model from disk.
	if _, err := os.Stat(filepath.Join(r.ProfilesDir, "minted-reviewer.json")); err != nil {
		t.Errorf("expected persisted profile: %v", err)
	}
	// Spec persisted for routing discovery + --resume durability.
	if _, err := os.Stat(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")); err != nil {
		t.Errorf("expected persisted phase.json: %v", err)
	}
}

func TestRegister_ForcesOptional(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Optional = false // advisor forgot; Registrar must force it, not reject
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
	cfg.Dispatch.ModelTierDefault = "deep" // rank 3 outside [1..1]
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: tier deep outside [fast..fast]")
	}
}

func TestRegister_CLINotAllowed_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.AllowedCLIs = []string{"codex"} // claude not allowed
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: cli claude not in allowed_clis [codex]")
	}
}

func TestRegister_InvalidName_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Name = "Minted Reviewer" // spaces + caps → not kebab-case
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: invalid name")
	}
}

// TestRegister_NonKebabDerivedProfileName_Rejected covers the SECOND name guard:
// the spec Name is valid kebab (passes ValidateUserSpec), but the advisor-set
// Agent field derives a non-kebab profile name. ValidateUserSpec does not check
// Agent, so this guard is the only thing stopping a bad filename + silent runner
// lookup miss. (Distinct from TestRegister_InvalidName_Rejected, which is caught
// earlier by ValidateUserSpec and never reaches the derived-name check.)
func TestRegister_NonKebabDerivedProfileName_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Name = "minted-reviewer"  // valid kebab → passes ValidateUserSpec
	cfg.Agent = "evolve-Bad Name" // ProfileName() strips "evolve-" → "Bad Name": caps+space
	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, "must be lowercase kebab-case")
	// Nothing persisted: the guard fires before persist.
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")) {
		t.Error("phase spec persisted despite a rejected derived profile name")
	}
}

func TestRegister_SourceWriter_GetsSandbox(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.WritesSource = true
	cfg.Dispatch.Sandbox = nil // not declared; Registrar must force it on
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Unmarshal the persisted profile and assert the sandbox is enabled for a
	// source-writer AND not read-only (a writer must be able to write).
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

// TestRegister_UnclassifiableTierWithEnvelope_Rejected locks the trust-kernel
// fix: a novel/typo tier string (TierRank 0) must NOT silently escape a
// declared envelope (ValidatePin alone would exempt rank 0).
func TestRegister_UnclassifiableTierWithEnvelope_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = "gpt-4o-mega" // unclassifiable → rank 0
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: unclassifiable tier with an envelope set")
	}
}

// TestRegister_CLIWithDriverSuffix_ClampedByBase proves the -tmux/-p suffix is
// stripped before the allowed_clis check, so an allowed base CLI passes.
func TestRegister_CLIWithDriverSuffix_ClampedByBase(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.CLI = "claude-tmux"              // driver-suffixed
	cfg.Dispatch.AllowedCLIs = []string{"claude"} // base form
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("claude-tmux should clamp to base claude (allowed); got %v", err)
	}
}

// TestRegister_NoEnvelopeNoAllowed_Passes proves the "no restriction" state
// (nil envelope + nil allowed_clis) is valid — clamp is opt-in.
func TestRegister_NoEnvelopeNoAllowed_Passes(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierEnvelope = nil
	cfg.Dispatch.AllowedCLIs = nil
	cfg.Dispatch.ModelTierDefault = "anything-goes" // rank 0 ok when no envelope
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("no-restriction minted phase should register; got %v", err)
	}
}

// TestRegister_EmptyDirs_SkipsPersistence proves persistence is opt-in: with
// both dirs empty, Register still returns a runner and writes nothing.
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

// TestRegister_SpecModelHint_DrivesClamp proves dispatchTier falls back to the
// spec Model hint when Dispatch.ModelTierDefault is empty: an out-of-envelope
// spec model must be rejected, which can only happen if cfg.Model is the tier
// fed to the clamp. (Mirror of TestRegister_TierOutsideEnvelope but via the
// Model-hint code path, covering dispatchTier's second return.)
func TestRegister_SpecModelHint_DrivesClamp(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = ""                                                     // force the spec-model-hint branch
	cfg.Dispatch.ModelTierEnvelope = &profiles.ModelTierEnvelope{Min: "fast", Max: "fast"} // [1..1]
	cfg.Model = "deep"                                                                     // rank 3, outside [1..1]
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: spec model hint deep outside [fast..fast]")
	}
}

// TestRegister_SpecModelHint_InEnvelope_Persists is the GREEN companion: with
// no ModelTierDefault, an in-envelope spec model registers AND the persisted
// profile carries an empty model_tier_default (proving ToProfile copied the
// empty dispatch default, not the spec hint).
func TestRegister_SpecModelHint_InEnvelope_Persists(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = "" // spec-model-hint branch
	cfg.Model = "balanced"             // rank 2, inside [fast..deep]
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

// TestRegister_ProfilePersistFails_PropagatesError proves a persist failure on
// the dispatch profile aborts Register with a wrapped error and writes no spec.
// We make ProfilesDir a regular FILE so the MkdirAll inside atomicwrite.JSON
// fails. (Covers persist's profile error return + Register's persist error
// propagation; atomicwrite's own MkdirAll branch is pinned in that package.)
func TestRegister_ProfilePersistFails_PropagatesError(t *testing.T) {
	r := newRegistrar(t)
	// Replace ProfilesDir with a file occupying that path → MkdirAll fails.
	fixtures.MustWrite(t, r.ProfilesDir, "i am a file, not a dir")

	_, err := r.Register(validCfg())
	fixtures.RequireErrContains(t, err, "persist profile")
	// The spec must NOT have been persisted (profile write failed first).
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")) {
		t.Error("phase spec persisted despite profile-write failure")
	}
}

// TestRegister_SpecPersistFails_PropagatesError proves a persist failure on the
// phase spec (after the profile succeeds) aborts Register with the spec-specific
// wrap. PhasesDir is a regular file, so MkdirAll for PhasesDir/<name> fails.
func TestRegister_SpecPersistFails_PropagatesError(t *testing.T) {
	r := newRegistrar(t)
	fixtures.MustWrite(t, r.PhasesDir, "i am a file, not a dir")

	_, err := r.Register(validCfg())
	fixtures.RequireErrContains(t, err, "persist phase spec")
	// The profile DID persist (it is written first, before the spec).
	if !fixtures.FilePresent(filepath.Join(r.ProfilesDir, "minted-reviewer.json")) {
		t.Error("expected the dispatch profile to persist before the spec write failed")
	}
}

// The atomic-write OS-fault branches (rename/createtemp/write/close) and the
// marshal-error + happy-path JSON layout now live in internal/atomicwrite and
// are pinned by its own 100%-coverage tests. persist's error wraps are covered
// by TestRegister_ProfilePersistFails / _SpecPersistFails above.

// --- cycle-967 (Variant A2): Register records the mint in the shared registry ---
//
// The tree-diff guard exempts a leaked .evolve/phases/<name> path only for a
// REGISTERED mint (core.isActiveMintPhasePath), so Register must append the
// name to the mintregistry — and must do so BEFORE persisting the files: the
// reverse order recreates the cycle-967 race (files visible to a concurrent
// lane's guard before the exemption exists).

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
	cfg.Dispatch.AllowedCLIs = []string{"codex"} // claude not allowed → clamp rejects
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

// TestRegister_RegistryAppendFails_RejectsBeforePersist: a mint the guard
// cannot discover is a cross-lane abort landmine, so a registry failure must
// reject the mint loudly — and nothing may have been persisted yet
// (register-before-persist ordering).
func TestRegister_RegistryAppendFails_RejectsBeforePersist(t *testing.T) {
	r := newRegistrar(t)
	base := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(base, "blocker"), "a file, not a dir")
	r.RegistryPath = filepath.Join(base, "blocker", "active-mints.json") // MkdirAll fails
	_, err := r.Register(validCfg())
	fixtures.RequireErrContains(t, err, "mintregistry")
	if fixtures.FilePresent(filepath.Join(r.ProfilesDir, "minted-reviewer.json")) {
		t.Error("profile persisted despite registry failure")
	}
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")) {
		t.Error("phase spec persisted despite registry failure")
	}
}

// --- Mint-time catalog-contract safety (docs/chronicle/2026-08-minted-stub-class.md) ---
//
// Register persists a phase.json + profile stub built from the advisor-supplied
// config. Empty Description/WhenToUse mints a stub that fails the repo-wide
// phasespec.TestPhaseCatalog_OptionalPhasesHaveSelectMetadata once ship's
// whole-tree bind tracks it; an empty dispatch.cli mints a driverless profile
// that fails the profiles repo-contract suites and dies at dispatch preflight.
// Four instances (#399, #404, #406, #407) were hand-fixed before the class was
// closed at this seam; these are its permanent regressions.

// repoRootDir resolves the repo root from this test file's own path — never the
// process cwd, which differs between the main tree and a cycle worktree.
func repoRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	// .../go/internal/phaseregistrar/registrar_test.go → up 3 to the repo root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// metadataLessCfg is validCfg with the hazard shape: an unknown phase name and
// no advisor-supplied SELECT metadata. The name is letters-only and multi-word
// because phasespec's user-phase name floor rejects digits.
func metadataLessCfg() phaseconfig.PhaseConfig {
	cfg := validCfg()
	cfg.Name = "minted-catalog-probe"
	cfg.Description = ""
	cfg.WhenToUse = ""
	return cfg
}

// guardCondition is the EXACT predicate
// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata applies to every merged
// catalog entry. Restated here so this regression tracks the guard's real
// semantics rather than a paraphrase of them.
func guardCondition(s phasespec.PhaseSpec) bool {
	return s.Optional && s.WhenToUse == "" && s.Description == ""
}

// TestRegister_UnknownPhaseNameStaysCatalogGreen is the wiring proof: mint an
// unknown phase name through the LIVE Register (real clamp, real atomicwrite
// persistence) into a temp project root, then reload it through the production
// phasespec.MergedCatalog — the same loader the guard test uses — and apply the
// guard's own condition. Asserting on the returned spec alone would not prove
// the round-trip through disk and the real catalog loader stays green.
func TestRegister_UnknownPhaseNameStaysCatalogGreen(t *testing.T) {
	proj := t.TempDir()

	// The merge path is builtin-registry + user overlay, so seed the real
	// built-in registry: a merge over a missing registry is a different path.
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
		PhasesDir:   filepath.Join(proj, ".evolve", "phases"), // the discovery root
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

// TestRegister_MetadataLessMint_DefaultsSelectMetadata pins both halves of the
// default: the returned spec AND the persisted artifact. Defaulting only in
// memory would still write the contract-breaking file that CI trips over.
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

// TestRegister_AdvisorMetadata_NotClobbered is the boundary: the guard's
// condition is an AND over both fields, so the default must fire only when BOTH
// are empty. An advisor that described its phase keeps that text verbatim — an
// unconditional default would silently overwrite real selection criteria.
func TestRegister_AdvisorMetadata_NotClobbered(t *testing.T) {
	const want = "Reviews the diff for envelope escapes."
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Description = want // WhenToUse stays empty: the guard is already satisfied

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

// TestRegister_DriverlessCLI_Rejected is the negative regression: a mint that
// would PERSIST a profile with no CLI is rejected, and nothing is written.
func TestRegister_DriverlessCLI_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := metadataLessCfg()
	cfg.Dispatch.CLI = "" // no driver

	_, err := r.Register(cfg)
	fixtures.RequireErrContains(t, err, "driverless profile stub")
	if fixtures.FilePresent(filepath.Join(r.ProfilesDir, cfg.Name+".json")) {
		t.Error("driverless profile persisted despite rejection")
	}
	if fixtures.FilePresent(filepath.Join(r.PhasesDir, cfg.Name, "phase.json")) {
		t.Error("phase spec persisted despite a rejected driverless mint")
	}
}

// TestRegister_DriverlessCLI_AllowedWithoutPersistence pins the guard's scope:
// with ProfilesDir empty, Register is a pure factory that writes nothing, so a
// config with no dispatch block still registers. This is the shape of every
// checked-in .evolve/phases/*/phase.json and the campaign study path
// (cmd/evolve/cmd_campaign.go:403) registers exactly that way — a blanket
// rejection would break it.
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
