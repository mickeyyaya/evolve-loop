package setup

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// applyFixture builds a real DetectReport over fixtureRepo (claude+codex ready,
// gemini blocked) so the report's per-phase constraints MATCH the on-disk
// profiles that ValidatePin reads — the production invariant.
func applyFixture(t *testing.T) (DetectReport, *profiles.Loader) {
	t.Helper()
	project, evolveDir := fixtureRepo(t)
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir,
		Env:     func(string) string { return "" },
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:  fakeDoctor,
		CapTier: func(string) string { return "full" },
	})
	return rep, profiles.NewFromDir(filepath.Join(evolveDir, "profiles"))
}

func fakeDoctorAllBlocked(ctx context.Context) bridge.DoctorReport {
	return bridge.DoctorReport{
		Results: []bridge.DoctorResult{
			{CLI: "claude-tmux", Binary: bridge.BinaryInfo{Present: false}, Verdict: "blocked"},
			{CLI: "codex-tmux", Binary: bridge.BinaryInfo{Present: false}, Verdict: "blocked"},
		},
	}
}

func parsePins(t *testing.T, b []byte) map[string]policy.Pin {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	pins := map[string]policy.Pin{}
	if raw, ok := obj["pins"]; ok {
		if err := json.Unmarshal(raw, &pins); err != nil {
			t.Fatalf("output pins not valid: %v", err)
		}
	}
	return pins
}

// 1. Unknown preset → error naming the valid set; no bytes.
func TestApply_UnknownPreset_Errors(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "turbo", nil, loader)
	if err == nil {
		t.Fatal("unknown preset should error")
	}
	if out != nil {
		t.Error("no bytes should be returned on error")
	}
}

// 2. Empty policy + recommended → only the DIFFERING phases are pinned; phases
// equal to their profile default (scout) are NOT restated.
func TestApply_EmptyPolicy_WritesOnlyDifferingPins(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	pins := parsePins(t, out)
	if _, ok := pins["builder"]; !ok {
		t.Error("builder differs from default (agy→claude) → should be pinned")
	}
	if _, ok := pins["auditor"]; !ok {
		t.Error("auditor differs from default (tier→deep) → should be pinned")
	}
	if _, ok := pins["scout"]; ok {
		t.Error("scout matches its profile default → must NOT be pinned")
	}
}

// 3. Lossless merge preserves foreign top-level keys (real policy.json shape).
func TestApply_LosslessMerge_PreservesForeignKeys(t *testing.T) {
	rep, loader := applyFixture(t)
	existing := `{
	  "version": 1,
	  "floor": [{"id":"dossier-closeout"}],
	  "cli_health": {"proactive_probe": true}
	}`
	out, err := Apply(rep, builtinPresets, "recommended", []byte(existing), loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"version", "floor", "cli_health", "pins"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("lossless merge dropped %q", k)
		}
	}
}

// 4. A pre-existing pin for a NON-Role phase survives untouched.
func TestApply_PreservesForeignPins(t *testing.T) {
	rep, loader := applyFixture(t)
	existing := `{"pins":{"deploy":{"cli":"claude","model":"deep"}}}`
	out, err := Apply(rep, builtinPresets, "recommended", []byte(existing), loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	pins := parsePins(t, out)
	if d, ok := pins["deploy"]; !ok || d.CLI != "claude" || d.Model != "deep" {
		t.Errorf("foreign pin 'deploy' should survive, got %+v (present=%v)", d, ok)
	}
}

// 5. Re-applying recommended after max-quality removes the upgraded pins it no
// longer needs (idempotent convergence, no stale pins).
func TestApply_ReapplyRecommendedClearsUpgrade(t *testing.T) {
	rep, loader := applyFixture(t)
	hi, err := Apply(rep, builtinPresets, "max-quality", nil, loader)
	if err != nil {
		t.Fatalf("apply max-quality: %v", err)
	}
	if _, ok := parsePins(t, hi)["scout"]; !ok {
		t.Fatal("precondition: max-quality should pin scout (upgraded to deep)")
	}
	lo, err := Apply(rep, builtinPresets, "recommended", hi, loader)
	if err != nil {
		t.Fatalf("re-apply recommended: %v", err)
	}
	if _, ok := parsePins(t, lo)["scout"]; ok {
		t.Error("re-applying recommended should clear the stale scout upgrade pin")
	}
	if b := parsePins(t, lo)["builder"]; b.Model != "balanced" {
		t.Errorf("builder tier after re-apply = %q, want balanced", b.Model)
	}
}

// 6. Malformed existing policy → refuse (no partial/clobbering write).
func TestApply_MalformedExisting_Refuses(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", []byte(`{not json`), loader)
	if err == nil || out != nil {
		t.Errorf("malformed existing should refuse with no bytes; err=%v bytes=%v", err, out != nil)
	}
	if err != nil && !strings.Contains(err.Error(), "clobber") {
		t.Errorf("error should say it refuses to clobber, got %v", err)
	}
}

// 7. Every emitted pin passes policy.ValidatePin against its profile.
func TestApply_EmittedPinsPassValidatePin(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "max-quality", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	for role, pin := range parsePins(t, out) {
		prof, perr := loader.Get(role)
		if perr != nil {
			t.Errorf("emitted pin for %q has no profile", role)
			continue
		}
		if verr := policy.ValidatePin(role, pin, &prof); verr != nil {
			t.Errorf("emitted pin %q=%+v breaches floor: %v", role, pin, verr)
		}
	}
}

// 8. A degraded preset (no families authed) is refused, never persisted.
func TestApply_DegradedPreset_Refuses(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir,
		Env: func(string) string { return "" }, Now: func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor: fakeDoctorAllBlocked, CapTier: func(string) string { return "n/a" },
	})
	loader := profiles.NewFromDir(filepath.Join(evolveDir, "profiles"))
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err == nil || out != nil {
		t.Errorf("degraded preset should refuse; err=%v bytes=%v", err, out != nil)
	}
}

// 9. A pin stores the abstract TIER, never the native model id (else ValidatePin's
// envelope check silently bypasses the floor — see policy.ValidatePin/TierRank).
func TestApply_PinStoresTierNotNativeModel(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	a := parsePins(t, out)["auditor"]
	if a.Model != "deep" {
		t.Errorf("auditor pin model = %q, want the tier 'deep' (not a native model id)", a.Model)
	}
}

// 10. Cross-family pins are legal and split across families.
func TestApply_CrossFamilyPinsLegal(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	pins := parsePins(t, out)
	if pins["builder"].CLI == pins["auditor"].CLI {
		t.Errorf("builder/auditor should be cross-family, both %q", pins["builder"].CLI)
	}
}
