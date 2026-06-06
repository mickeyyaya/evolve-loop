package phasespec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeUserPhase creates <phasesDir>/<name>/phase.json with body.
func writeUserPhase(t *testing.T, phasesDir, name, body string) {
	t.Helper()
	dir := filepath.Join(phasesDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write phase.json: %v", err)
	}
}

func TestDiscoverUserSpecs(t *testing.T) {
	phasesDir := t.TempDir()
	writeUserPhase(t, phasesDir, "security-scan", `{"name":"security-scan","optional":true,"kind":"llm"}`)
	writeUserPhase(t, phasesDir, "lint-pass", `{"optional":true}`) // name defaults to dir
	writeUserPhase(t, phasesDir, "broken", `{not json`)
	// a directory with no phase.json is silently ignored
	if err := os.MkdirAll(filepath.Join(phasesDir, "notaphase"), 0o755); err != nil {
		t.Fatal(err)
	}

	specs, warnings := DiscoverUserSpecs(phasesDir)
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2: %v", len(specs), names(specs))
	}
	// Sorted by dir name: lint-pass, security-scan
	if specs[0].Name != "lint-pass" || specs[1].Name != "security-scan" {
		t.Errorf("specs order = %v, want [lint-pass security-scan]", names(specs))
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %v, want 1 (malformed broken/phase.json)", warnings)
	}
}

func TestDiscoverUserSpecs_MissingDir(t *testing.T) {
	specs, warnings := DiscoverUserSpecs(filepath.Join(t.TempDir(), "does-not-exist"))
	if specs != nil || warnings != nil {
		t.Errorf("missing dir should yield (nil,nil), got specs=%v warnings=%v", specs, warnings)
	}
}

func TestCatalog_Merge(t *testing.T) {
	builtin, err := Load(writeRegistry(t, fullRegistry)) // scout (builtin), security-scan (builtin in fixture)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	user := []PhaseSpec{
		{Name: "security-scan", Optional: true}, // clashes with fixture builtin → dropped
		{Name: "lint-pass", Optional: true},     // new → added
		{Name: ""},                              // empty → skipped
	}
	merged, warnings := builtin.Merge(user)

	if _, ok := merged.Get("lint-pass"); !ok {
		t.Error("lint-pass should be in merged catalog")
	}
	if !merged.IsUser("lint-pass") {
		t.Error("lint-pass should be flagged as a user phase")
	}
	if merged.IsUser("scout") {
		t.Error("scout is built-in, must not be flagged user")
	}
	// clash + empty-name → 2 warnings
	if len(warnings) != 2 {
		t.Errorf("warnings = %v, want 2 (clash + empty name)", warnings)
	}
	// receiver not mutated
	if _, ok := builtin.Get("lint-pass"); ok {
		t.Error("Merge mutated the receiver catalog")
	}
}

func TestValidateUserSpec(t *testing.T) {
	cases := []struct {
		name      string
		spec      PhaseSpec
		wantClean bool
		wantSub   string // substring expected in a violation when not clean
	}{
		{"valid llm optional", PhaseSpec{Name: "security-scan", Optional: true, Kind: "llm"}, true, ""},
		{"valid default kind", PhaseSpec{Name: "lint-pass", Optional: true}, true, ""},
		{"missing name", PhaseSpec{Optional: true}, false, "name is required"},
		{"bad name", PhaseSpec{Name: "Security Scan", Optional: true}, false, "kebab-case"},
		{"not optional (floor)", PhaseSpec{Name: "x", Optional: false}, false, "must be optional"},
		{"native reserved", PhaseSpec{Name: "x", Optional: true, Kind: "native"}, false, "reserved"},
		{"traversal agent", PhaseSpec{Name: "x", Optional: true, Agent: "../../evil"}, false, "kebab-case"},
		{"valid agent", PhaseSpec{Name: "x-check", Optional: true, Agent: "evolve-x-check"}, true, ""},
		{"unknown kind", PhaseSpec{Name: "x", Optional: true, Kind: "wat"}, false, "unknown kind"},
		{"bad verdict_on_pass", PhaseSpec{Name: "x", Optional: true, Classify: &ClassifyRules{VerdictOnPass: "pass"}}, false, "PASS/FAIL/WARN/SKIPPED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := ValidateUserSpec(tc.spec)
			if tc.wantClean {
				if len(v) != 0 {
					t.Errorf("expected clean, got violations: %v", v)
				}
				return
			}
			if len(v) == 0 {
				t.Fatalf("expected a violation containing %q, got none", tc.wantSub)
			}
			found := false
			for _, s := range v {
				if strings.Contains(s, tc.wantSub) {
					found = true
				}
			}
			if !found {
				t.Errorf("violations %v missing substring %q", v, tc.wantSub)
			}
		})
	}
}
