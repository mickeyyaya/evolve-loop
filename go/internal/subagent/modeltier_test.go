package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubReadProfile returns the same body for any path.
func stubReadProfile(body string) func(string) (string, error) {
	return func(string) (string, error) { return body, nil }
}

// stubReadState returns the same body for any projectRoot.
func stubReadState(body string, err error) func(string) (string, error) {
	return func(string) (string, error) { return body, err }
}

func TestResolveModelTier_HintWinsForEveryAgent(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{"hint on auditor", `{"role":"auditor","model_tier_default":"opus"}`},
		{"hint on scout", `{"role":"scout","model_tier_default":"sonnet"}`},
		{"hint on builder", `{"role":"builder","model_tier_default":"sonnet"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tier, err := ResolveModelTier(
				ResolveModelTierRequest{
					ProfilePath:   "/dev/null",
					ModelTierHint: "haiku",
				},
				ResolveModelTierOptions{
					ReadProfile: stubReadProfile(tc.role),
					ReadState:   stubReadState("", os.ErrNotExist),
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tier != "haiku" {
				t.Errorf("expected hint to win, got %q", tier)
			}
		})
	}
}

func TestResolveModelTier_NonAuditorUsesProfileDefault(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{"scout sonnet", `{"role":"scout","model_tier_default":"sonnet"}`, "sonnet"},
		{"builder opus override in profile", `{"role":"builder","model_tier_default":"opus"}`, "opus"},
		{"intent haiku", `{"role":"intent","model_tier_default":"haiku"}`, "haiku"},
		{"role from name field when role missing", `{"name":"tdd-engineer","model_tier_default":"sonnet"}`, "sonnet"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tier, err := ResolveModelTier(
				ResolveModelTierRequest{ProfilePath: "/p"},
				ResolveModelTierOptions{
					ReadProfile: stubReadProfile(tc.profile),
					ReadState:   stubReadState("", nil),
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tier != tc.want {
				t.Errorf("got %q, want %q", tier, tc.want)
			}
		})
	}
}

func TestResolveModelTier_AuditorOverrideWinsInsideAuditor(t *testing.T) {
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{
			ProfilePath:         "/p",
			AuditorTierOverride: "haiku",
		},
		ResolveModelTierOptions{
			ReadProfile: stubReadProfile(`{"role":"auditor","model_tier_default":"sonnet"}`),
			ReadState:   stubReadState(`{"consecutiveSuccesses":10}`, nil),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != "haiku" {
		t.Errorf("expected override haiku, got %q", tier)
	}
}

func TestResolveModelTier_AuditorOverrideOnlyAppliesToAuditor(t *testing.T) {
	// EVOLVE_AUDITOR_TIER_OVERRIDE has no effect on non-auditor agents.
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{
			ProfilePath:         "/p",
			AuditorTierOverride: "haiku",
		},
		ResolveModelTierOptions{
			ReadProfile: stubReadProfile(`{"role":"scout","model_tier_default":"sonnet"}`),
			ReadState:   stubReadState("", nil),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != "sonnet" {
		t.Errorf("expected scout profile-default sonnet, got %q", tier)
	}
}

func TestResolveModelTier_AuditorMasteryGateOpusOnLowStreak(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{"missing state file", "", TierOpus},
		{"missing field", `{"foo":1}`, TierOpus},
		{"streak=0", `{"consecutiveSuccesses":0}`, TierOpus},
		{"non-integer field", `{"consecutiveSuccesses":"oops"}`, TierOpus},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stateErr error
			if tc.state == "" {
				stateErr = os.ErrNotExist
			}
			tier, err := ResolveModelTier(
				ResolveModelTierRequest{ProfilePath: "/p"},
				ResolveModelTierOptions{
					ReadProfile: stubReadProfile(`{"role":"auditor","model_tier_default":"sonnet"}`),
					ReadState:   stubReadState(tc.state, stateErr),
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tier != tc.want {
				t.Errorf("got %q, want %q", tier, tc.want)
			}
		})
	}
}

func TestResolveModelTier_AuditorStreakOneFallsToDiffComplexity(t *testing.T) {
	// streak >= 1 with diff-complexity NOT disabled and trivial → sonnet.
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{
			ProfilePath:  "/p",
			WorktreePath: "/wt",
		},
		ResolveModelTierOptions{
			ReadProfile:    stubReadProfile(`{"role":"auditor","model_tier_default":"opus"}`),
			ReadState:      stubReadState(`{"consecutiveSuccesses":3}`, nil),
			DiffComplexity: func(wt string) (string, error) { return "trivial", nil },
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierSonnet {
		t.Errorf("trivial diff should downgrade to sonnet, got %q", tier)
	}
}

func TestResolveModelTier_AuditorStreakOneNonTrivialUsesProfileDefault(t *testing.T) {
	tests := []struct{ complexity string }{
		{"standard"}, {"complex"}, {""},
	}
	for _, tc := range tests {
		t.Run("complexity="+tc.complexity, func(t *testing.T) {
			tier, err := ResolveModelTier(
				ResolveModelTierRequest{ProfilePath: "/p", WorktreePath: "/wt"},
				ResolveModelTierOptions{
					ReadProfile:    stubReadProfile(`{"role":"auditor","model_tier_default":"sonnet"}`),
					ReadState:      stubReadState(`{"consecutiveSuccesses":5}`, nil),
					DiffComplexity: func(string) (string, error) { return tc.complexity, nil },
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tier != TierSonnet {
				t.Errorf("got %q, want profile default sonnet", tier)
			}
		})
	}
}

func TestResolveModelTier_AuditorDiffComplexityDisabledSkipsTrivialCheck(t *testing.T) {
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{
			ProfilePath:            "/p",
			DiffComplexityDisabled: true,
			WorktreePath:           "/wt",
		},
		ResolveModelTierOptions{
			ReadProfile:    stubReadProfile(`{"role":"auditor","model_tier_default":"opus"}`),
			ReadState:      stubReadState(`{"consecutiveSuccesses":99}`, nil),
			DiffComplexity: func(string) (string, error) { return "trivial", nil },
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierOpus {
		t.Errorf("kill-switch should force profile default opus, got %q", tier)
	}
}

func TestResolveModelTier_AuditorDiffComplexityErrorFallsThrough(t *testing.T) {
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{ProfilePath: "/p", WorktreePath: "/wt"},
		ResolveModelTierOptions{
			ReadProfile:    stubReadProfile(`{"role":"auditor","model_tier_default":"sonnet"}`),
			ReadState:      stubReadState(`{"consecutiveSuccesses":3}`, nil),
			DiffComplexity: func(string) (string, error) { return "", errors.New("boom") },
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierSonnet {
		t.Errorf("diff-complexity error should fall through to profile default, got %q", tier)
	}
}

func TestResolveModelTier_ProfileReadError(t *testing.T) {
	_, err := ResolveModelTier(
		ResolveModelTierRequest{ProfilePath: "/missing"},
		ResolveModelTierOptions{
			ReadProfile: func(string) (string, error) { return "", os.ErrNotExist },
		},
	)
	if err == nil || !strings.Contains(err.Error(), "read profile") {
		t.Fatalf("expected read profile error, got: %v", err)
	}
}

func TestResolveModelTier_MissingDefaultTier(t *testing.T) {
	_, err := ResolveModelTier(
		ResolveModelTierRequest{ProfilePath: "/p"},
		ResolveModelTierOptions{
			ReadProfile: stubReadProfile(`{"role":"scout"}`),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "model_tier_default") {
		t.Fatalf("expected missing-default error, got: %v", err)
	}
}

func TestResolveModelTier_DefaultsExerciseRealFilesystem(t *testing.T) {
	tmp := t.TempDir()
	profilePath := filepath.Join(tmp, "auditor.json")
	if err := os.WriteFile(profilePath, []byte(`{"role":"auditor","model_tier_default":"sonnet"}`), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(tmp, ".evolve", "state.json"),
		[]byte(`{"consecutiveSuccesses":7}`), 0o644,
	); err != nil {
		t.Fatalf("write state: %v", err)
	}

	// With streak >= 1, no diff-complexity helper, falls through to profile default sonnet.
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{
			ProfilePath: profilePath,
			ProjectRoot: tmp,
		},
		ResolveModelTierOptions{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierSonnet {
		t.Errorf("got %q, want %q", tier, TierSonnet)
	}
}

func TestExtractProfileString(t *testing.T) {
	tests := []struct {
		name, body, field, want string
	}{
		{"present", `{"role":"auditor"}`, "role", "auditor"},
		{"absent", `{"role":"auditor"}`, "name", ""},
		{"whitespace in body", `{"role"  :  "scout"}`, "role", "scout"},
		{"escaped quote in field name", `{"weird/key":"x"}`, "weird/key", "x"},
		{"empty value", `{"role":""}`, "role", ""},
		{"non-JSON noise around value", `prefix"role":"hidden"suffix`, "role", "hidden"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractProfileString(tc.body, tc.field)
			if got != tc.want {
				t.Errorf("extractProfileString(%q, %q) = %q, want %q", tc.body, tc.field, got, tc.want)
			}
		})
	}
}

func TestReadConsecutiveSuccesses(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
		want int
	}{
		{"missing file", "", os.ErrNotExist, 0},
		{"missing field", `{"foo":1}`, nil, 0},
		{"present zero", `{"consecutiveSuccesses":0}`, nil, 0},
		{"present positive", `{"consecutiveSuccesses":12}`, nil, 12},
		{"whitespace and noise", `something "consecutiveSuccesses"  :  4 other`, nil, 4},
		{"string value", `{"consecutiveSuccesses":"oops"}`, nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := readConsecutiveSuccesses(stubReadState(tc.body, tc.err), "/anywhere")
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDefaultStateReader_MissingFile(t *testing.T) {
	if _, err := defaultReadState(t.TempDir()); err == nil {
		t.Errorf("expected error for missing state.json")
	}
}

func TestDefaultProfileReader_MissingFile(t *testing.T) {
	if _, err := defaultReadProfile(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Errorf("expected error for missing profile")
	}
}
