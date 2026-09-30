package skillcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_WriteMode_NoDrift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var stdout, stderr strings.Builder
	code := Run(tmp, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run write no-drift: exit %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("Run write no-drift: DRIFT: must not appear on stderr when tree is clean; got %q", stderr.String())
	}
}

func TestRun_CheckMode_Drift_OutputIsolation(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("Run check drift: exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("want 'DRIFT:' on stderr; got stderr=%q", stderr.String())
	}
	if strings.Contains(stdout.String(), "DRIFT:") {
		t.Errorf("DRIFT: must not appear on stdout; got stdout=%q", stdout.String())
	}
}

func TestRun_WriteMode_Idempotent(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	var out1, err1 strings.Builder
	code1 := Run(tmp, true, &out1, &err1)
	if code1 != 0 {
		t.Fatalf("write mode (first pass): exit %d; stdout=%q stderr=%q",
			code1, out1.String(), err1.String())
	}

	var out2, err2 strings.Builder
	code2 := Run(tmp, false, &out2, &err2)
	if code2 != 0 {
		t.Fatalf("check mode (after write repair): exit %d, want 0 (tree should be clean); stdout=%q stderr=%q",
			code2, out2.String(), err2.String())
	}
}

func TestNameMismatches_ReadDirFail(t *testing.T) {
	errs := nameMismatches("/nonexistent/path/that/does/not/exist/at/all")
	if len(errs) == 0 {
		t.Fatal("expected error message for missing skills dir, got none")
	}
	if !strings.Contains(errs[0], "read skills dir") {
		t.Errorf("expected 'read skills dir' in message; got %q", errs[0])
	}
}

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

func TestNameMismatches_NoSkillMD(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "skills", "my-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if errs := nameMismatches(tmp); len(errs) != 0 {
		t.Errorf("expected no errors for skill dir without SKILL.md; got %v", errs)
	}
}

func TestNameMismatches_UnparseableFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "bad-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestParallelSubtaskCount_EmptyRaw(t *testing.T) {
	if got := parallelSubtaskCount(nil); got != 0 {
		t.Errorf("nil: got %d, want 0", got)
	}
	if got := parallelSubtaskCount(json.RawMessage{}); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
}

func TestParallelSubtaskCount_InvalidJSON(t *testing.T) {
	if got := parallelSubtaskCount(json.RawMessage(`not-json`)); got != 0 {
		t.Errorf("got %d, want 0 for invalid JSON", got)
	}
}

func TestRegistryRoles_ReadFail(t *testing.T) {
	roles := registryRoles("/nonexistent/path/at/all")
	if len(roles) != 0 {
		t.Errorf("expected empty map for missing registry; got %v", roles)
	}
}

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

func TestCheck_InvalidCatalogError(t *testing.T) {
	tmp := t.TempDir()
	_, err := Check(tmp)
	if err == nil {
		t.Fatal("expected error from Check with invalid/empty catalog root")
	}
}

func TestCheck_MissingSkillMDError(t *testing.T) {
	tmp := t.TempDir()
	regDir := filepath.Join(tmp, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Check(tmp)
	if err == nil {
		t.Fatal("expected error from Check when skills are missing")
	}
}

func TestCheck_CorruptSkillMDError(t *testing.T) {
	tmp := prepareSkillsTree(t)
	target := filepath.Join(tmp, "skills", "build", "SKILL.md")
	corruptContent := factsBegin + " test -->\nno end marker\n"
	if err := os.WriteFile(target, []byte(corruptContent), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Check(tmp)
	if err == nil {
		t.Fatal("expected error from Check when SKILL.md has corrupt markers")
	}
}

func TestRun_WriteFail(t *testing.T) {
	tmp := prepareSkillsTree(t)
	target := mutateBuildSkill(t, tmp)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := Run(tmp, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected Run to return 1 on write failure, got %d. stderr: %s", code, stderr.String())
	}
}
