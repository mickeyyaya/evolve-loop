package skillcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot locates the repo root from this file (go/internal/skillcheck/ →
// three levels up) and skips when skills/ is absent (vendored/partial checkout).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate repo root")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "skills")); err != nil {
		t.Skipf("skills/ not found at %s: %v", root, err)
	}
	return root
}

// TestCheck_CleanRepoNoDrift: the live repo's SKILL.md regions are in sync with
// the SSOTs, so the audit-facing Check returns no drift. This is the in-process
// equivalent of the CI `evolve skills check` gate.
func TestCheck_CleanRepoNoDrift(t *testing.T) {
	drift, err := Check(repoRoot(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(drift) != 0 {
		t.Fatalf("live repo should have no skill drift; got %v", drift)
	}
}

// TestCheck_DetectsMutatedRegion: a hand edit inside a generated phase-facts
// region must surface in Check's drift list (keyed by the SKILL.md rel-path),
// so the cycle audit FAILs a cycle that drifted a SKILL.md.
func TestCheck_DetectsMutatedRegion(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	for _, rel := range []string{filepath.Join("docs", "architecture", "phase-registry.json")} {
		copyFile(t, filepath.Join(root, rel), filepath.Join(tmp, rel))
	}
	for _, dir := range []string{"skills", "agents", filepath.Join(".evolve", "profiles")} {
		copyTree(t, filepath.Join(root, dir), filepath.Join(tmp, dir))
	}

	target := filepath.Join(tmp, "skills", "build", "SKILL.md")
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	mutated := strings.Replace(string(raw), "## Output contract", "## Output contracts", 1)
	if mutated == string(raw) {
		t.Fatal("fixture mutation did not apply — generated region heading not found")
	}
	if err := os.WriteFile(target, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write mutated fixture: %v", err)
	}

	drift, err := Check(tmp)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, d := range drift {
		if strings.Contains(d, filepath.Join("skills", "build", "SKILL.md")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("Check did not flag the drifted skills/build/SKILL.md; got %v", drift)
	}
}

// TestSpliceGeneratedRegion_Idempotent pins replace-in-place semantics: a second
// splice of the same block is a byte-level no-op, and content outside the
// markers survives verbatim. (Moved here with the splice logic from cmd/evolve.)
func TestSpliceGeneratedRegion_Idempotent(t *testing.T) {
	doc := "---\nname: x\n---\n\nintro prose\n\n## Composition\n\ntail\n"
	block := factsBegin + " test -->\nBODY\n" + factsEnd + "\n"

	once, err := spliceGeneratedRegion(doc, block)
	if err != nil {
		t.Fatalf("first splice: %v", err)
	}
	if !strings.Contains(once, "intro prose") || !strings.Contains(once, "tail") {
		t.Fatalf("hand prose lost:\n%s", once)
	}
	if !strings.Contains(once, "BODY") {
		t.Fatalf("block not inserted:\n%s", once)
	}
	if strings.Index(once, "BODY") > strings.Index(once, "## Composition") {
		t.Errorf("block inserted after Composition:\n%s", once)
	}

	twice, err := spliceGeneratedRegion(once, block)
	if err != nil {
		t.Fatalf("second splice: %v", err)
	}
	if twice != once {
		t.Errorf("splice not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

// TestSpliceGeneratedRegion_CorruptMarkers errors on BEGIN without END.
func TestSpliceGeneratedRegion_CorruptMarkers(t *testing.T) {
	doc := "intro\n" + factsBegin + " broken -->\nno end marker\n"
	if _, err := spliceGeneratedRegion(doc, "block\n"); err == nil {
		t.Fatal("want error for BEGIN without END, got nil")
	}
}

// TestSpliceGeneratedRegion_MultiplePairs errors when a second BEGIN exists
// (e.g. a botched manual merge) instead of leaving an orphaned stale region.
func TestSpliceGeneratedRegion_MultiplePairs(t *testing.T) {
	pair := factsBegin + " a -->\nold\n" + factsEnd + "\n"
	doc := "intro\n" + pair + "middle\n" + pair + "tail\n"
	if _, err := spliceGeneratedRegion(doc, "block\n"); err == nil {
		t.Fatal("want error for two marker pairs, got nil")
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		copyFile(t, p, filepath.Join(dst, rel))
		return nil
	})
	if err != nil {
		t.Fatalf("copy tree %s: %v", src, err)
	}
}

// prepareSkillsTree copies the minimal SSOT files needed by Run/Check into a
// new temp dir and returns the dir. All TestRun_* tests use this to share setup.
func prepareSkillsTree(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	tmp := t.TempDir()
	copyFile(t, filepath.Join(root, "docs", "architecture", "phase-registry.json"),
		filepath.Join(tmp, "docs", "architecture", "phase-registry.json"))
	for _, dir := range []string{"skills", "agents", filepath.Join(".evolve", "profiles")} {
		copyTree(t, filepath.Join(root, dir), filepath.Join(tmp, dir))
	}
	return tmp
}

// mutateBuildSkill mutates the skills/build/SKILL.md in the given root to
// create a detectable drift, returning the path. Skips when the heading is absent.
func mutateBuildSkill(t *testing.T, root string) string {
	t.Helper()
	target := filepath.Join(root, "skills", "build", "SKILL.md")
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	mutated := strings.Replace(string(raw), "## Output contract", "## Output contracts", 1)
	if mutated == string(raw) {
		t.Skip("fixture mutation anchor not found — skills/build/SKILL.md may have changed")
	}
	if err := os.WriteFile(target, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write mutated fixture: %v", err)
	}
	return target
}

func TestRun_CheckMode_NoDrift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run check no-drift: exit %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "check OK") {
		t.Errorf("want 'check OK' on stdout; got %q", stdout.String())
	}
}

func TestRun_CheckMode_Drift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("Run check drift: exit %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("want 'DRIFT:' on stderr; got %q", stderr.String())
	}
}

func TestRun_WriteMode_RewritesDrift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	target := mutateBuildSkill(t, tmp)
	original := func() []byte {
		// capture the pre-mutation content by reading from the real repo
		root := repoRoot(t)
		b, err := os.ReadFile(filepath.Join(root, "skills", "build", "SKILL.md"))
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		return b
	}()

	var stdout, stderr strings.Builder
	code := Run(tmp, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run write mode: exit %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	if string(after) != string(original) {
		t.Error("SKILL.md was not rewritten to original content by write mode")
	}
}

func TestRun_InvalidRoot(t *testing.T) {
	tmp := t.TempDir() // empty dir — no phase-registry.json
	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run invalid root: exit %d, want 1; stderr=%q", code, stderr.String())
	}
}

// TestRun_CheckMode_FrontmatterNameDrift — a skill whose SKILL.md name !=
// dir name must surface as a DRIFT: message on stderr (exit 2).
func TestRun_CheckMode_FrontmatterNameDrift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	// Add an extra skill whose frontmatter name does not match the dir name.
	extraDir := filepath.Join(tmp, "skills", "extra-skill")
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: different-name\n---\n\n# Extra Skill\n"
	if err := os.WriteFile(filepath.Join(extraDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("Run: exit %d, want 2 (name drift must fail check); stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("want 'DRIFT:' on stderr for frontmatter name mismatch; got %q", stderr.String())
	}
}
