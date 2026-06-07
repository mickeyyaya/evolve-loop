// cmd_skills_drift_test.go is the producer-side alarm for ADR-0040's skill
// projection: it runs `evolve skills check` in-process against the live repo,
// so a hand edit inside a GENERATED:phase-facts region — or an SSOT change
// without a regenerate — fails CI instead of silently shipping drifted docs.
// Same pattern as phasecontract/contract_test.go (runtime.Caller locates the
// repo; the live tree is the fixture).
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRootForSkills locates the repo root from this file's location
// (go/cmd/evolve/ → three levels up) and skips the test if the skills/ tree
// is absent (e.g. a vendored or partial checkout).
func repoRootForSkills(t *testing.T) string {
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

// TestSkills_NoDrift asserts every phase skill's generated region matches what
// the SSOTs produce today, and every skill's frontmatter name equals its dir.
func TestSkills_NoDrift(t *testing.T) {
	root := repoRootForSkills(t)
	var out, errBuf bytes.Buffer
	if code := skillsRun(root, false, &out, &errBuf); code != 0 {
		t.Fatalf("evolve skills check: exit %d\nstderr:\n%s", code, errBuf.String())
	}
}

// TestSkills_CheckDetectsDrift mutates a generated region in a temp copy and
// asserts check exits 2 — the alarm actually fires.
func TestSkills_CheckDetectsDrift(t *testing.T) {
	root := repoRootForSkills(t)
	tmp := t.TempDir()

	// Minimal repo copy: registry + profiles + agents + skills (the four
	// inputs skillsRun reads).
	for _, rel := range []string{
		filepath.Join("docs", "architecture", "phase-registry.json"),
	} {
		copyFileForTest(t, filepath.Join(root, rel), filepath.Join(tmp, rel))
	}
	for _, dir := range []string{"skills", "agents", filepath.Join(".evolve", "profiles")} {
		copyTreeForTest(t, filepath.Join(root, dir), filepath.Join(tmp, dir))
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

	var out, errBuf bytes.Buffer
	if code := skillsRun(tmp, false, &out, &errBuf); code != 2 {
		t.Fatalf("check on drifted tree: exit %d, want 2\nstderr:\n%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "DRIFT:") {
		t.Errorf("stderr missing DRIFT report:\n%s", errBuf.String())
	}
}

// TestSpliceGeneratedRegion_Idempotent pins replace-in-place semantics: a
// second splice of the same block is a byte-level no-op, and content outside
// the markers survives verbatim.
func TestSpliceGeneratedRegion_Idempotent(t *testing.T) {
	doc := "---\nname: x\n---\n\nintro prose\n\n## Composition\n\ntail\n"
	block := skillFactsBegin + " test -->\nBODY\n" + skillFactsEnd + "\n"

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
	// Block must land before ## Composition on first insert.
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

// TestSpliceGeneratedRegion_CorruptMarkers errors on BEGIN without END rather
// than guessing.
func TestSpliceGeneratedRegion_CorruptMarkers(t *testing.T) {
	doc := "intro\n" + skillFactsBegin + " broken -->\nno end marker\n"
	if _, err := spliceGeneratedRegion(doc, "block\n"); err == nil {
		t.Fatal("want error for BEGIN without END, got nil")
	}
}

// TestSpliceGeneratedRegion_MultiplePairs errors when a second BEGIN exists
// (e.g. a botched manual merge) instead of leaving an orphaned stale region.
func TestSpliceGeneratedRegion_MultiplePairs(t *testing.T) {
	pair := skillFactsBegin + " a -->\nold\n" + skillFactsEnd + "\n"
	doc := "intro\n" + pair + "middle\n" + pair + "tail\n"
	if _, err := spliceGeneratedRegion(doc, "block\n"); err == nil {
		t.Fatal("want error for two marker pairs, got nil")
	}
}

func copyFileForTest(t *testing.T, src, dst string) {
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

func copyTreeForTest(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		copyFileForTest(t, p, filepath.Join(dst, rel))
		return nil
	})
	if err != nil {
		t.Fatalf("copy tree %s: %v", src, err)
	}
}
