package setup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var floorCeilingProfiles = []string{"auditor", "adversarial-review", "tdd-engineer", "spec-verifier", "spec-verify", "code-reviewer"}

func profilesWithoutFloorCeilings(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "..", ".evolve", "profiles")
	dst := filepath.Join(t.TempDir(), ".evolve", "profiles")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if name := strings.TrimSuffix(e.Name(), ".json"); contains(floorCeilingProfiles, name) {
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			delete(doc, "allowed_clis")
			if raw, err = json.Marshal(doc); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func codexAndAgyOnlyDoctor(context.Context) bridge.DoctorReport {
	ready := func(cli string) bridge.DoctorResult {
		return bridge.DoctorResult{CLI: cli, Binary: bridge.BinaryInfo{Present: true, Path: "/bin/" + cli}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"}
	}
	return bridge.DoctorReport{Results: []bridge.DoctorResult{ready("codex-tmux"), ready("agy-tmux")}}
}

func TestSetupApply_RefusesAFloorRolePinOutsideClaude(t *testing.T) {
	profilesDir := profilesWithoutFloorCeilings(t)
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: filepath.Dir(filepath.Dir(profilesDir)), EvolveDir: filepath.Dir(profilesDir),
		Env:     func(string) string { return "" },
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:  codexAndAgyOnlyDoctor,
		CapTier: func(string) string { return "full" },
	})
	for _, ps := range rep.Phases {
		if (ps.Role == "auditor" || ps.Role == "tdd-engineer") && !contains(ps.AllowedCLIs, "claude") {
			t.Errorf("detect reports %s with allowed_clis %v: a floor role must be clamped to claude", ps.Role, ps.AllowedCLIs)
		}
	}
	preset, err := findPreset(Recommend(rep, builtinPresets), "recommended")
	if err != nil {
		t.Fatal(err)
	}
	var degraded []string
	for _, a := range preset.Assignments {
		if a.Warning != "" {
			degraded = append(degraded, a.Role)
		}
	}
	if !contains(degraded, "auditor") || !contains(degraded, "tdd-engineer") {
		t.Errorf("degraded roles = %v, want the auditor and the tdd-engineer: no claude is authed", degraded)
	}
	out, err := Apply(rep, builtinPresets, "recommended", nil, profiles.NewFromDir(profilesDir))
	if err == nil {
		t.Fatalf("apply emitted pins for floor roles with only codex and agy authed: %s", out)
	}
}

func TestSetupApply_RefusesAFloorPinThatNoWarningFlagged(t *testing.T) {
	rep, _ := applyFixture(t)
	var phases []PhaseStatus
	for _, ps := range rep.Phases {
		switch ps.Role {
		case "auditor":
			ps.DefaultCLI, ps.AllowedCLIs = "claude-tmux", []string{"codex"}
			phases = append(phases, ps)
		case "builder":
			phases = append(phases, ps)
		}
	}
	rep.Phases = phases
	out, err := Apply(rep, builtinPresets, "recommended", nil, profiles.NewFromDir(profilesWithoutFloorCeilings(t)))
	if !errors.Is(err, ErrPinBreachesFloor) {
		t.Fatalf("apply = %s, %v: a codex pin for the auditor must breach the claude floor", out, err)
	}
}

func TestSetupDetect_ReportsAPinViolationForAFloorRoleOverACeilinglessProfile(t *testing.T) {
	profilesDir := profilesWithoutFloorCeilings(t)
	evolveDir := filepath.Dir(profilesDir)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"pins":{"auditor":{"cli":"codex-tmux"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: filepath.Dir(evolveDir), EvolveDir: evolveDir,
		Env:     func(string) string { return "" },
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:  codexAndAgyOnlyDoctor,
		CapTier: func(string) string { return "full" },
	})
	for _, ps := range rep.Phases {
		if ps.Role == "auditor" && ps.PinViolation == "" {
			t.Fatalf("auditor pinned to codex over a ceilingless profile reports no violation: %+v", ps)
		}
	}
}
