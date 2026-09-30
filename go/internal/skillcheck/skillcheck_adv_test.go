package skillcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestParallelSubtaskCount_ArrayThreeItems(t *testing.T) {
	raw := json.RawMessage(`{"parallel_subtasks": ["a","b","c"]}`)
	if got := parallelSubtaskCount(raw); got != 3 {
		t.Errorf("3-element array: got %d, want 3", got)
	}
}

func TestParallelSubtaskCount_ArraySingleItem(t *testing.T) {
	raw := json.RawMessage(`{"parallel_subtasks": [{}]}`)
	if got := parallelSubtaskCount(raw); got != 1 {
		t.Errorf("1-element array: got %d, want 1", got)
	}
}

func TestParallelSubtaskCount_ArrayEmpty(t *testing.T) {
	raw := json.RawMessage(`{"parallel_subtasks": []}`)
	if got := parallelSubtaskCount(raw); got != 0 {
		t.Errorf("empty array: got %d, want 0", got)
	}
}

func TestParallelSubtaskCount_MissingKey(t *testing.T) {
	raw := json.RawMessage(`{"other_key": 42}`)
	if got := parallelSubtaskCount(raw); got != 0 {
		t.Errorf("missing key: got %d, want 0", got)
	}
}

func TestParallelSubtaskCount_WrongTypeIsZero(t *testing.T) {
	raw := json.RawMessage(`{"parallel_subtasks": 5}`)
	got := parallelSubtaskCount(raw)
	if got != 0 {
		t.Errorf("non-array parallel_subtasks: got %d, want 0 (unmarshal error path)", got)
	}
}

func TestParallelSubtaskCount_ArrayFiveItems(t *testing.T) {
	raw := json.RawMessage(`{"parallel_subtasks": [1,2,3,4,5]}`)
	if got := parallelSubtaskCount(raw); got != 5 {
		t.Errorf("5-element array: got %d, want 5", got)
	}
}

func TestRegistryRoles_ValidArrayFormat(t *testing.T) {
	tmp := t.TempDir()
	regDir := filepath.Join(tmp, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"phases":[{"name":"build","role":"builder"},{"name":"scout","role":"explorer"}]}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := registryRoles(tmp)
	if len(roles) == 0 {
		t.Fatal("expected non-empty map for valid phases array")
	}
	if roles["build"] != "builder" {
		t.Errorf("build role = %q, want 'builder'", roles["build"])
	}
	if roles["scout"] != "explorer" {
		t.Errorf("scout role = %q, want 'explorer'", roles["scout"])
	}
}

func TestRegistryRoles_EntryWithEmptyRole(t *testing.T) {
	tmp := t.TempDir()
	regDir := filepath.Join(tmp, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"phases":[{"name":"native-phase","role":""}]}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	roles := registryRoles(tmp)
	_ = roles
}

func TestNameMismatches_MultipleErrors(t *testing.T) {
	tmp := t.TempDir()
	for _, pair := range []struct{ dir, name string }{
		{"skill-one", "wrong-one"},
		{"skill-two", "wrong-two"},
	} {
		dir := filepath.Join(tmp, "skills", pair.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + pair.name + "\n---\n\n# Skill\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	errs := nameMismatches(tmp)
	if len(errs) < 2 {
		t.Errorf("expected ≥2 drift errors for 2 mismatched skills; got %d: %v", len(errs), errs)
	}
}

func TestNameMismatches_EmptyFrontmatterName(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: \"\"\n---\n\n# My Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	errs := nameMismatches(tmp)
	if len(errs) == 0 {
		t.Error("empty frontmatter name should not silently match dir name 'my-skill'")
	}
}

func TestNameMismatches_MatchingName(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "correct-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: correct-skill\n---\n\n# Correct Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := nameMismatches(tmp); len(errs) != 0 {
		t.Errorf("matching name should produce no errors; got %v", errs)
	}
}

func TestCollectSkillFacts_RoleMissingFromRegistry(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "orphan-phase"}
	roles := map[string]string{
		"other-phase": "builder",
	}
	facts := collectSkillFacts(t.TempDir(), spec, roles)
	if facts.Role != "orphan-phase" {
		t.Errorf("phase absent from registry: Role=%q, want 'orphan-phase' (fallback to spec.Name)", facts.Role)
	}
}

func TestCollectSkillFacts_RolePresent(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "build"}
	roles := map[string]string{"build": "builder"}
	facts := collectSkillFacts(t.TempDir(), spec, roles)
	if facts.Role != "builder" {
		t.Errorf("Role = %q, want 'builder'", facts.Role)
	}
}

func TestCollectSkillFacts_PhaseNamePropagated(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "my-phase"}
	facts := collectSkillFacts(t.TempDir(), spec, map[string]string{})
	if facts.Phase != "my-phase" {
		t.Errorf("Phase = %q, want 'my-phase'", facts.Phase)
	}
}

func TestPersonaPath_NotFound(t *testing.T) {
	root := t.TempDir()
	spec := phasespec.PhaseSpec{Name: "my-phase"}
	path := personaPath(root, spec, "builder")
	if path != "" {
		t.Errorf("expected empty path when no persona file exists; got %q", path)
	}
}

func TestPersonaPath_EmptyRole(t *testing.T) {
	root := t.TempDir()
	spec := phasespec.PhaseSpec{Name: "some-phase"}
	path := personaPath(root, spec, "")
	_ = path
}

func TestSpliceMarkedRegion_NeitherMarkerNorAnchor(t *testing.T) {
	doc := "some content"
	block := "appended block"
	result, err := SpliceMarkedRegion(doc, block, "<!-- BEGIN -->", "<!-- END -->", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "appended block") {
		t.Errorf("block not found in result: %q", result)
	}
	if !strings.Contains(result, "some content") {
		t.Errorf("original doc lost in result: %q", result)
	}
}

func TestSpliceMarkedRegion_EmptyDoc(t *testing.T) {
	result, err := SpliceMarkedRegion("", "block", "BEGIN", "END", "")
	if err != nil {
		t.Fatalf("empty doc: unexpected error: %v", err)
	}
	if !strings.Contains(result, "block") {
		t.Errorf("block missing from empty-doc result: %q", result)
	}
}

func TestSpliceMarkedRegion_WithMarkers(t *testing.T) {
	begin := "<!-- FACTS:begin -->"
	end := "<!-- FACTS:end -->"
	doc := "before\n" + begin + "\nold content\n" + end + "\nafter\n"
	result, err := SpliceMarkedRegion(doc, "new content", begin, end, "")
	if err != nil {
		t.Fatalf("marker splice: %v", err)
	}
	if strings.Contains(result, "old content") {
		t.Error("old content should be replaced")
	}
	if !strings.Contains(result, "new content") {
		t.Error("new content should be present")
	}
	if !strings.Contains(result, "before") || !strings.Contains(result, "after") {
		t.Error("surrounding content must be preserved")
	}
}

func TestSpliceMarkedRegion_IdempotentWithMarkers(t *testing.T) {
	begin := "<!-- FACTS:begin -->"
	end := "<!-- FACTS:end -->"
	doc := "header\n" + begin + "\noriginal\n" + end + "\nfooter\n"
	block := begin + "\nreplacement\n" + end + "\n"

	first, err := SpliceMarkedRegion(doc, block, begin, end, "")
	if err != nil {
		t.Fatalf("first splice: %v", err)
	}
	second, err := SpliceMarkedRegion(first, block, begin, end, "")
	if err != nil {
		t.Fatalf("second splice: %v", err)
	}
	if first != second {
		t.Errorf("not idempotent (block must include markers):\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestSpliceMarkedRegion_BeginWithoutEnd(t *testing.T) {
	begin := "<!-- BEGIN -->"
	doc := "before\n" + begin + "\nno end here\n"
	_, err := SpliceMarkedRegion(doc, "block", begin, "<!-- END -->", "")
	if err == nil {
		t.Error("expected error for BEGIN without END")
	}
}

func TestRun_CheckMode_PhaseNotInRegistry(t *testing.T) {
	tmp := prepareSkillsTree(t)
	extra := filepath.Join(tmp, "skills", "adv-test-phase-not-in-registry")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extra, "SKILL.md"),
		[]byte("---\nname: adv-test-phase-not-in-registry\n---\n\n# Adv Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	_ = Run(tmp, false, &stdout, &stderr)
}
