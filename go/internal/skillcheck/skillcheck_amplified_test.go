package skillcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_WriteMode_NoDrift: write=true with a clean tree must exit 0 and must
// NOT emit any error output. Write mode differs from check mode here: it does
// not necessarily emit "check OK" (check mode does; write mode may omit it when
// nothing was written). The critical contract is exit 0 and no DRIFT: on stderr.
func TestRun_WriteMode_NoDrift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var stdout, stderr strings.Builder
	code := Run(tmp, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run write no-drift: exit %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	// Write mode with no drift must not produce any error signal on stderr.
	if strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("Run write no-drift: DRIFT: must not appear on stderr when tree is clean; got %q", stderr.String())
	}
}

// TestRun_CheckMode_Drift_OutputIsolation: when drift is detected, DRIFT: must
// appear ONLY on stderr and NOT bleed onto stdout. Callers that redirect stdout
// to a log must not receive error noise inline with any status output.
func TestRun_CheckMode_Drift_OutputIsolation(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("Run check drift: exit %d, want 2", code)
	}
	// DRIFT: must be on stderr.
	if !strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("want 'DRIFT:' on stderr; got stderr=%q", stderr.String())
	}
	// DRIFT: must NOT leak to stdout.
	if strings.Contains(stdout.String(), "DRIFT:") {
		t.Errorf("DRIFT: must not appear on stdout; got stdout=%q", stdout.String())
	}
}

// TestRun_WriteMode_Idempotent: after write mode repairs drift, a subsequent
// check-mode run must see a clean tree (exit 0). This tests that write mode
// leaves the tree in a state consistent with future no-drift checks.
func TestRun_WriteMode_Idempotent(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	// First pass: write=true should repair the drift.
	var out1, err1 strings.Builder
	code1 := Run(tmp, true, &out1, &err1)
	if code1 != 0 {
		t.Fatalf("write mode (first pass): exit %d; stdout=%q stderr=%q",
			code1, out1.String(), err1.String())
	}

	// Second pass: write=false should find the tree clean.
	var out2, err2 strings.Builder
	code2 := Run(tmp, false, &out2, &err2)
	if code2 != 0 {
		t.Fatalf("check mode (after write repair): exit %d, want 0 (tree should be clean); stdout=%q stderr=%q",
			code2, out2.String(), err2.String())
	}
}

// --- nameMismatches unit tests ---

// TestNameMismatches_ReadDirFail — a missing skills dir returns an error message.
func TestNameMismatches_ReadDirFail(t *testing.T) {
	errs := nameMismatches("/nonexistent/path/that/does/not/exist/at/all")
	if len(errs) == 0 {
		t.Fatal("expected error message for missing skills dir, got none")
	}
	if !strings.Contains(errs[0], "read skills dir") {
		t.Errorf("expected 'read skills dir' in message; got %q", errs[0])
	}
}

// TestNameMismatches_NonDirEntrySkipped — a plain file inside skills/ is silently skipped.
func TestNameMismatches_NonDirEntrySkipped(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("# Skills"), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := nameMismatches(tmp); len(errs) != 0 {
		t.Errorf("expected no errors for non-dir entry; got %v", errs)
	}
}

// TestNameMismatches_NoSkillMD — a skill dir without SKILL.md is silently skipped.
func TestNameMismatches_NoSkillMD(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "skills", "my-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if errs := nameMismatches(tmp); len(errs) != 0 {
		t.Errorf("expected no errors for skill dir without SKILL.md; got %v", errs)
	}
}

// TestNameMismatches_UnparseableFrontmatter — SKILL.md with an unterminated
// frontmatter block is flagged with "unparseable".
func TestNameMismatches_UnparseableFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "bad-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unterminated frontmatter — no closing "---".
	content := "---\nname: bad-skill\n# body (no closing fence)\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	errs := nameMismatches(tmp)
	if len(errs) == 0 {
		t.Fatal("expected error for unparseable frontmatter; got none")
	}
	if !strings.Contains(errs[0], "unparseable") {
		t.Errorf("expected 'unparseable' in error; got %q", errs[0])
	}
}

// TestNameMismatches_NameDrift — SKILL.md with frontmatter name != dir name
// is flagged with "DRIFT:".
func TestNameMismatches_NameDrift(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: different-name\n---\n\n# My Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	errs := nameMismatches(tmp)
	if len(errs) == 0 {
		t.Fatal("expected drift error for mismatched frontmatter name; got none")
	}
	if !strings.Contains(errs[0], "DRIFT:") {
		t.Errorf("expected 'DRIFT:' in error; got %q", errs[0])
	}
}

// --- parallelSubtaskCount unit tests ---

// TestParallelSubtaskCount_EmptyRaw — nil or empty raw message returns 0.
func TestParallelSubtaskCount_EmptyRaw(t *testing.T) {
	if got := parallelSubtaskCount(nil); got != 0 {
		t.Errorf("nil: got %d, want 0", got)
	}
	if got := parallelSubtaskCount(json.RawMessage{}); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
}

// TestParallelSubtaskCount_InvalidJSON — malformed JSON returns 0.
func TestParallelSubtaskCount_InvalidJSON(t *testing.T) {
	if got := parallelSubtaskCount(json.RawMessage(`not-json`)); got != 0 {
		t.Errorf("got %d, want 0 for invalid JSON", got)
	}
}

// --- registryRoles unit tests ---

// TestRegistryRoles_ReadFail — missing registry file returns an empty map.
func TestRegistryRoles_ReadFail(t *testing.T) {
	roles := registryRoles("/nonexistent/path/at/all")
	if len(roles) != 0 {
		t.Errorf("expected empty map for missing registry; got %v", roles)
	}
}

// TestRegistryRoles_InvalidJSON — invalid JSON in registry returns an empty map.
func TestRegistryRoles_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	regDir := filepath.Join(tmp, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte("not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := registryRoles(tmp)
	if len(roles) != 0 {
		t.Errorf("expected empty map for invalid JSON; got %v", roles)
	}
}
