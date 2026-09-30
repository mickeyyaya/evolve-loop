package setup

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestApply_WhitespaceOnlyPolicyIsEmpty(t *testing.T) {
	rep, loader := applyFixture(t)
	if _, err := Apply(rep, builtinPresets, "recommended", []byte("  \n\t"), loader); err != nil {
		t.Fatalf("whitespace-only policy must count as empty: %v", err)
	}
}

func TestApply_MalformedPinsBlockRefused(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", []byte(`{"pins":[1]}`), loader)
	if err == nil || out != nil || !strings.Contains(err.Error(), "pins block is malformed") {
		t.Fatalf("got out=%s err=%v", out, err)
	}
}

func TestApply_FloorBreachingPinRefused(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "scout.json"), `{"cli":"claude-tmux","model_tier_default":"balanced","allowed_clis":["claude"]}`)
	rep := mkReport([]CLIStatus{famBlocked("claude"), famReady("codex", codexTM)},
		profilePhase("scout", "claude-tmux", "balanced", "", "", "", []string{"all"}, ""))
	out, err := Apply(rep, builtinPresets, "recommended", nil, profiles.NewFromDir(dir))
	if err == nil || out != nil || !strings.Contains(err.Error(), "breaches floor") {
		t.Fatalf("got out=%s err=%v", out, err)
	}
}

func TestApply_EmptiedPinsBlockIsRemoved(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "scout.json"), `{"cli":"claude-tmux","model_tier_default":"balanced"}`)
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		profilePhase("scout", "claude-tmux", "balanced", "", "", "", nil, ""))
	out, err := Apply(rep, builtinPresets, "recommended", []byte(`{"version":1,"pins":{"scout":{"cli":"codex","model":"deep"}}}`), profiles.NewFromDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "pins") {
		t.Fatalf("an emptied pins block must be removed:\n%s", out)
	}
}

func TestApply_OutputIsTwoSpaceIndentedWithTrailingNewline(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", []byte(`{"version":1}`), loader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(out), "}\n") || !strings.Contains(string(out), "\n  \"version\": 1") {
		t.Fatalf("policy bytes shape:\n%q", out)
	}
}

func twoPresets(def string) PresetConfig {
	return PresetConfig{Default: def, Presets: []PresetSpec{{Name: "a", TierBias: "default"}, {Name: "b", TierBias: "default"}}}
}

func TestRecommend_ConfiguredDefaultWins(t *testing.T) {
	if got := Recommend(mkReport(nil), twoPresets("b")).Default; got != "b" {
		t.Fatalf("default = %q, want b", got)
	}
}

func TestRecommend_UnsetDefaultIsFirstPreset(t *testing.T) {
	if got := Recommend(mkReport(nil), twoPresets("")).Default; got != "a" {
		t.Fatalf("default = %q, want a", got)
	}
}

func TestRecommend_BuilderOffDefaultFamilyIsFallback(t *testing.T) {
	rep := mkReport([]CLIStatus{famBlocked("claude"), famReady("codex", codexTM)},
		profilePhase("builder", "claude-tmux", "balanced", "", "", "", []string{"all"}, "auditor"),
		profilePhase("auditor", "codex-tmux", "deep", "", "", "", []string{"all"}, "builder"))
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "builder")
	if a.CLI != "codex" || !a.CLIFallback {
		t.Fatalf("builder = %+v", a)
	}
}

func TestRecommend_AuditorSplitOffBuilderFamilyIsFallback(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM), famReady("codex", codexTM)},
		profilePhase("builder", "claude-tmux", "balanced", "", "", "", []string{"all"}, "auditor"),
		profilePhase("auditor", "claude-tmux", "deep", "", "", "", []string{"all"}, "builder"))
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "auditor")
	if a.CLI != "codex" || !a.CLIFallback {
		t.Fatalf("auditor = %+v", a)
	}
}

func TestRecommend_UnpairedAuditorUsesItsDefault(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		profilePhase("auditor", "claude-tmux", "deep", "", "", "", nil, ""))
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "auditor")
	if a.CLI != "claude" {
		t.Fatalf("auditor = %+v", a)
	}
}

func TestRecommend_DegradedRationaleIsTheWarning(t *testing.T) {
	rep := mkReport([]CLIStatus{famBlocked("claude")},
		profilePhase("scout", "claude-tmux", "balanced", "", "", "", nil, ""))
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "scout")
	if a.Warning == "" || a.Rationale != a.Warning {
		t.Fatalf("scout = %+v", a)
	}
}

func twoFamilyDoctor(ctx context.Context) bridge.DoctorReport {
	return bridge.DoctorReport{Results: []bridge.DoctorResult{
		{CLI: "codex-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/opt/bin/codex"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready", EnvWarnings: []string{"OPENAI_BASE_URL is set"}},
		{CLI: "claude-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/opt/bin/claude"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"},
	}}
}

func detectWithSeams(t *testing.T, project, evolveDir string, capTier func(string) string, now time.Time) DetectReport {
	t.Helper()
	return Detect(context.Background(), DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		AdaptersDir: t.TempDir(),
		Env:         func(string) string { return "" },
		Now:         func() time.Time { return now },
		Doctor:      twoFamilyDoctor,
		CapTier:     capTier,
	})
}

func TestDetect_CLIRowsCarryDoctorFieldsSortedByFamily(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	rep := detectWithSeams(t, project, evolveDir, func(string) string { return "full" }, time.Unix(0, 0))
	if len(rep.CLIs) != 2 || rep.CLIs[0].CLI != "claude" || rep.CLIs[1].CLI != "codex" {
		t.Fatalf("CLIs = %+v", rep.CLIs)
	}
	codex := rep.CLIs[1]
	if codex.BinaryPath != "/opt/bin/codex" || !codex.AuthConfigured || len(codex.EnvWarnings) != 1 || len(codex.TierModels) != 4 {
		t.Fatalf("codex = %+v", codex)
	}
}

func TestDetect_DefaultCapabilityProbeReadsAdaptersDir(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	rep := detectWithSeams(t, project, evolveDir, nil, time.Unix(0, 0))
	for _, c := range rep.CLIs {
		if c.CapabilityTier != "full" {
			t.Fatalf("%s capability tier = %q, want full", c.CLI, c.CapabilityTier)
		}
	}
}

func TestDetect_ScanTimeIsUTC(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("plus1", 3600))
	rep := detectWithSeams(t, project, evolveDir, func(string) string { return "full" }, now)
	if rep.ScannedAt != "2026-01-02T02:04:05Z" {
		t.Fatalf("ScannedAt = %q", rep.ScannedAt)
	}
}

func TestDetect_UnresolvablePhaseIsUnresolved(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	writeFile(t, filepath.Join(evolveDir, "profiles", "memo.json"), `{"model_tier_default":"fast"}`)
	rep := detectWithSeams(t, project, evolveDir, func(string) string { return "full" }, time.Unix(0, 0))
	if m := phaseByRole(rep, "memo"); m.Source != "unresolved" {
		t.Fatalf("memo = %+v", m)
	}
}

func TestDetect_PhaseCarriesAllowedCLIs(t *testing.T) {
	b := phaseByRole(detectWithPolicy(t, ""), "builder")
	if strings.Join(b.AllowedCLIs, ",") != "claude,agy" {
		t.Fatalf("builder = %+v", b)
	}
}

func TestDetect_PartialPinKeepsTheUnpinnedHalf(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins":{"scout":{"model":"deep"},"auditor":{"cli":"claude"}}}`)
	if s := phaseByRole(rep, "scout"); s.CurrentCLI != "claude-tmux" || s.CurrentTier != "deep" {
		t.Fatalf("scout = %+v", s)
	}
	if a := phaseByRole(rep, "auditor"); a.CurrentCLI != "claude" || a.CurrentTier != "sonnet" {
		t.Fatalf("auditor = %+v", a)
	}
}
