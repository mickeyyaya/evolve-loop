// Tests for the phaseflags package — proof that all 6 phase runners
// can share one resolver instead of each duplicating the same helper.
package phaseflags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfile drops a single profile JSON into profileDir under the
// given phase name.
func writeProfile(t *testing.T, phase, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, phase+".json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return dir
}

// TestPermissionModeEnvKey_PhaseNameMapping — canonical phase names
// produce the documented env-var keys. Hyphens become underscores.
func TestPermissionModeEnvKey_PhaseNameMapping(t *testing.T) {
	cases := map[string]string{
		"build":         "EVOLVE_BUILD_PERMISSION_MODE",
		"scout":         "EVOLVE_SCOUT_PERMISSION_MODE",
		"intent":        "EVOLVE_INTENT_PERMISSION_MODE",
		"triage":        "EVOLVE_TRIAGE_PERMISSION_MODE",
		"tdd":           "EVOLVE_TDD_PERMISSION_MODE",
		"audit":         "EVOLVE_AUDIT_PERMISSION_MODE",
		"tdd-engineer":  "EVOLVE_TDD_ENGINEER_PERMISSION_MODE",
		"plan-reviewer": "EVOLVE_PLAN_REVIEWER_PERMISSION_MODE",
	}
	for phase, want := range cases {
		if got := For(phase).PermissionModeEnvKey(); got != want {
			t.Errorf("For(%q).PermissionModeEnvKey()=%q, want %q", phase, got, want)
		}
	}
}

// TestFor_PreservesName — round-trip through the factory.
func TestFor_PreservesName(t *testing.T) {
	if got := For("scout").Name(); got != "scout" {
		t.Errorf("For(scout).Name()=%q, want scout", got)
	}
}

// TestResolve_NoProfileNoEnv_ReturnsNil — fresh project, no .evolve
// dir, no env. The bridge gets nil/empty ExtraFlags.
func TestResolve_NoProfileNoEnv_ReturnsNil(t *testing.T) {
	got := For("build").Resolve(t.TempDir(), nil)
	if len(got) != 0 {
		t.Errorf("Resolve with no profile and no env should be empty; got %v", got)
	}
}

// TestResolve_ProfileMode_NoEnv — profile.permission_mode populates
// the flag when env is unset.
func TestResolve_ProfileMode_NoEnv(t *testing.T) {
	dir := writeProfile(t, "build", `{"permission_mode":"acceptEdits"}`)
	got := For("build").Resolve(dir, nil)
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--permission-mode acceptEdits") {
		t.Errorf("want --permission-mode acceptEdits; got %v", got)
	}
}

// TestResolve_EnvBeatsProfile — env override wins.
func TestResolve_EnvBeatsProfile(t *testing.T) {
	dir := writeProfile(t, "build", `{"permission_mode":"acceptEdits"}`)
	got := For("build").Resolve(dir, map[string]string{
		"EVOLVE_BUILD_PERMISSION_MODE": "plan",
	})
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--permission-mode plan") {
		t.Errorf("want --permission-mode plan from env; got %v", got)
	}
	if strings.Contains(joined, "acceptEdits") {
		t.Errorf("env should fully replace profile value; got %v", got)
	}
}

// TestResolve_EnvOnly_NoProfile — env-only path still works.
func TestResolve_EnvOnly_NoProfile(t *testing.T) {
	got := For("scout").Resolve(t.TempDir(), map[string]string{
		"EVOLVE_SCOUT_PERMISSION_MODE": "plan",
	})
	if len(got) != 2 || got[0] != "--permission-mode" || got[1] != "plan" {
		t.Errorf("env-only resolve got %v, want [--permission-mode plan]", got)
	}
}

// TestResolve_ProfileExtraFlagsAlwaysIncluded — profile.extra_flags
// pass through regardless of permission_mode state.
func TestResolve_ProfileExtraFlagsAlwaysIncluded(t *testing.T) {
	dir := writeProfile(t, "audit", `{"extra_flags":["--require-full","--print"]}`)
	got := For("audit").Resolve(dir, nil)
	want := map[string]bool{"--require-full": true, "--print": true}
	for _, f := range got {
		delete(want, f)
	}
	if len(want) != 0 {
		t.Errorf("Resolve missing %v; got %v", want, got)
	}
}

// TestResolve_ProfileExtraFlagsPlusEnvMode — extra_flags + env-driven
// permission_mode coexist.
func TestResolve_ProfileExtraFlagsPlusEnvMode(t *testing.T) {
	dir := writeProfile(t, "build", `{"extra_flags":["--require-full"]}`)
	got := For("build").Resolve(dir, map[string]string{
		"EVOLVE_BUILD_PERMISSION_MODE": "plan",
	})
	joined := strings.Join(got, " ")
	for _, want := range []string{"--require-full", "--permission-mode", "plan"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Resolve missing %q; got %v", want, got)
		}
	}
}

// TestResolve_MalformedProfile_NonFatal — broken JSON does not crash;
// env-driven flags still pass through. Guards against a corrupted
// .evolve/profiles/<phase>.json hanging the pipeline.
func TestResolve_MalformedProfile_NonFatal(t *testing.T) {
	dir := writeProfile(t, "build", "not-valid-json{{{")
	got := For("build").Resolve(dir, map[string]string{
		"EVOLVE_BUILD_PERMISSION_MODE": "plan",
	})
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--permission-mode plan") {
		t.Errorf("env mode should still propagate over malformed profile; got %v", got)
	}
}

// TestResolve_HyphenatedPhaseName — phase names with hyphens use the
// underscored env-var form.
func TestResolve_HyphenatedPhaseName(t *testing.T) {
	got := For("tdd-engineer").Resolve(t.TempDir(), map[string]string{
		"EVOLVE_TDD_ENGINEER_PERMISSION_MODE": "plan",
	})
	if len(got) != 2 || got[1] != "plan" {
		t.Errorf("hyphenated phase did not resolve env; got %v", got)
	}
}

// TestResolve_EmptyEnvValue_FallsThroughToProfile — explicit empty
// string in reqEnv is treated like an absent key.
func TestResolve_EmptyEnvValue_FallsThroughToProfile(t *testing.T) {
	dir := writeProfile(t, "build", `{"permission_mode":"acceptEdits"}`)
	got := For("build").Resolve(dir, map[string]string{
		"EVOLVE_BUILD_PERMISSION_MODE": "",
	})
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--permission-mode acceptEdits") {
		t.Errorf("empty env value should yield to profile; got %v", got)
	}
}

// TestResolve_NilReqEnv_NoCrash — defensive: nil map is safe.
func TestResolve_NilReqEnv_NoCrash(t *testing.T) {
	dir := writeProfile(t, "build", `{"permission_mode":"plan"}`)
	got := For("build").Resolve(dir, nil)
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--permission-mode plan") {
		t.Errorf("nil reqEnv should still resolve profile mode; got %v", got)
	}
}
