//go:build acs

package cycle2137

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	retrospectiveRoot = "docs/retrospectives"
	packageNote       = "docs/architecture/packages/internal-research.md"
)

var goalG3Keywords = []string{"retrospective", "recall", "wavecanary", "recovery"}

func lessonYAML(id string) string {
	return fmt.Sprintf(`- id: %s
  pattern: wavecanary-recall-root
  description: recovery retrospective wavecanary lesson that the recall must surface for the goal
  confidence: 0.9
  source: fixture
  type: failure-lesson
  category: episodic
  preventiveAction: read the retrospective lesson before the next recovery
  failureContext:
    failedStep: build
    errorCategory: wavecanary-recall-root
    auditVerdict: FAIL
`, id)
}

func writeLessonUnder(t *testing.T, projectRoot, relDir, id string) {
	t.Helper()
	dir := filepath.Join(projectRoot, relDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".yaml"), []byte(lessonYAML(id)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func absRoots(projectRoot string, cfg policy.PathsConfig) []string {
	var roots []string
	for _, p := range research.SearchPathsFromEnv(cfg) {
		if filepath.IsAbs(p) {
			roots = append(roots, p)
			continue
		}
		roots = append(roots, filepath.Join(projectRoot, p))
	}
	return roots
}

func recalledIDs(t *testing.T, roots []string) []string {
	t.Helper()
	got, err := research.NewFileKB(roots).Lookup(context.Background(), research.Query{Keywords: goalG3Keywords})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, l := range got {
		ids = append(ids, l.ID)
	}
	return ids
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestC2137_001_RetrospectiveLessonIsRecalledByDefaultRoots(t *testing.T) {
	project := t.TempDir()
	writeLessonUnder(t, project, retrospectiveRoot, "retro-c6-in-retrospectives")
	writeLessonUnder(t, project, "docs/unrelated", "retro-c6-outside-any-root")

	ids := recalledIDs(t, absRoots(project, policy.PathsConfig{}))

	if !contains(ids, "retro-c6-in-retrospectives") {
		t.Errorf("RED: lesson under %s not recalled for goal G3 with the default roots; recalled %v", retrospectiveRoot, ids)
	}
	if contains(ids, "retro-c6-outside-any-root") {
		t.Errorf("a lesson outside every default root was recalled: %v", ids)
	}
}

func TestC2137_001_DefaultRootsKeepEveryExistingRoot(t *testing.T) {
	project := t.TempDir()
	existing := []string{"knowledge-base/research", ".evolve/instincts/lessons", "docs/research"}
	for _, rel := range existing {
		writeLessonUnder(t, project, rel, "keep-"+strings.NewReplacer("/", "-", ".", "").Replace(rel))
	}
	writeLessonUnder(t, project, retrospectiveRoot, "keep-retrospectives")

	ids := recalledIDs(t, absRoots(project, policy.PathsConfig{}))

	for _, id := range []string{"keep-knowledge-base-research", "keep-evolve-instincts-lessons", "keep-docs-research", "keep-retrospectives"} {
		if !contains(ids, id) {
			t.Errorf("RED: default roots must recall %s; recalled %v", id, ids)
		}
	}
}

func TestC2137_001_RetrospectiveRootIsFlatNotRecursive(t *testing.T) {
	project := t.TempDir()
	writeLessonUnder(t, project, retrospectiveRoot+"/nested", "retro-c6-nested")
	writeLessonUnder(t, project, retrospectiveRoot, "retro-c6-flat")

	ids := recalledIDs(t, absRoots(project, policy.PathsConfig{}))

	if !contains(ids, "retro-c6-flat") {
		t.Errorf("RED: flat lesson under %s not recalled; recalled %v", retrospectiveRoot, ids)
	}
	if contains(ids, "retro-c6-nested") {
		t.Errorf("recall walked into a subdirectory of %s: %v", retrospectiveRoot, ids)
	}
}

func TestC2137_002_PolicyKBSearchPathsStillReplacesTheDefaultList(t *testing.T) {
	project := t.TempDir()
	writeLessonUnder(t, project, retrospectiveRoot, "retro-c6-default-root")
	writeLessonUnder(t, project, "custom/kb", "retro-c6-custom-root")
	cfg := policy.PathsConfig{KBSearchPaths: "custom/kb"}

	paths := research.SearchPathsFromEnv(cfg)
	ids := recalledIDs(t, absRoots(project, cfg))

	if len(paths) != 1 || paths[0] != "custom/kb" {
		t.Errorf("policy kb_search_paths must replace the default list; got %v", paths)
	}
	if !contains(ids, "retro-c6-custom-root") {
		t.Errorf("custom root not recalled: %v", ids)
	}
	if contains(ids, "retro-c6-default-root") {
		t.Errorf("a policy value must not keep %s as an extra root: %v", retrospectiveRoot, ids)
	}
}

func TestC2137_002_PolicyValueNamingRetrospectivesAloneIsHonoured(t *testing.T) {
	project := t.TempDir()
	writeLessonUnder(t, project, retrospectiveRoot, "retro-c6-only")
	writeLessonUnder(t, project, "docs/research", "retro-c6-research")
	cfg := policy.PathsConfig{KBSearchPaths: retrospectiveRoot}

	ids := recalledIDs(t, absRoots(project, cfg))

	if !contains(ids, "retro-c6-only") || contains(ids, "retro-c6-research") {
		t.Errorf("policy value %q must select exactly that root; recalled %v", retrospectiveRoot, ids)
	}
}

func TestC2137_003_PackageNoteNamesTheRetrospectiveRoot(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, packageNote)

	if !acsassert.FileContains(t, note, retrospectiveRoot) {
		t.Errorf("RED: %s does not name the recall root %s", packageNote, retrospectiveRoot)
	}
}
