//go:build acs

package cycle1190

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const liveItemJSON = `{
  "id": "operator-state-task-archetype-native-apply",
  "title": "New task archetype for live main-tree STATE mutations",
  "weight_hint": 0.93,
  "files": ["go/internal/phases/triage/", "go/internal/core/"],
  "class": "pipeline-architecture"
}`

const classlessItemJSON = `{
  "id": "no-class-declared",
  "title": "an item filed before the class vocabulary existed",
  "kind": "bug"
}`

func writeInbox(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("seed inbox file %s: %v", name, err)
		}
	}
	return dir
}

func loadOne(t *testing.T, dir, id string) inboxbatch.Item {
	t.Helper()
	items, warnings, err := inboxbatch.LoadDir(dir)
	if err != nil {
		t.Fatalf("inboxbatch.LoadDir(%s): %v", dir, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("inboxbatch.LoadDir(%s) warned on well-formed fixtures: %v", dir, warnings)
	}
	for _, it := range items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("item %q not loaded from %s (got %d items)", id, dir, len(items))
	return inboxbatch.Item{}
}

func TestC1190_001_item_class_round_trips_from_inbox_json(t *testing.T) {
	dir := writeInbox(t, map[string]string{
		"2026-07-21T07-02-00Z-operator-state-task-archetype.json": liveItemJSON,
		"2026-01-01T00-00-00Z-no-class-declared.json":             classlessItemJSON,
	})

	got := loadOne(t, dir, "operator-state-task-archetype-native-apply")
	if got.Class != "pipeline-architecture" {
		t.Errorf("Item.Class = %q, want %q — the declared class is still being dropped at load", got.Class, "pipeline-architecture")
	}
	if got.Weight != 0 {
		t.Errorf("Item.Weight = %v, want 0 (fixture declares only weight_hint) — field mapping disturbed", got.Weight)
	}
	if len(got.Files) != 2 || got.Files[0] != "go/internal/phases/triage/" {
		t.Errorf("Item.Files = %v, want the 2 declared paths — field mapping disturbed", got.Files)
	}

	absent := loadOne(t, dir, "no-class-declared")
	if absent.Class != "" {
		t.Errorf("Item.Class = %q for an item declaring no class, want \"\" — tolerant-by-default violated", absent.Class)
	}
	if absent.Kind != "bug" {
		t.Errorf("Item.Kind = %q, want %q — the new tag displaced an existing one", absent.Kind, "bug")
	}
}

func TestC1190_002_operator_state_detected_for_evolve_only_pipeline_item(t *testing.T) {
	it := inboxbatch.Item{
		ID:    "archive-processed-inbox-items",
		Class: "pipeline-architecture",
		Files: []string{".evolve/inbox/", ".evolve/state.json"},
	}
	if !inboxbatch.IsOperatorState(it) {
		t.Errorf("IsOperatorState(%+v) = false, want true — a pipeline-architecture item touching only .evolve/ state is the operator-state archetype", it)
	}
}

func TestC1190_003_operator_state_rejects_source_touching_and_degenerate_items(t *testing.T) {
	cases := []struct {
		name string
		item inboxbatch.Item
	}{
		{
			name: "class matches but files are source",
			item: inboxbatch.Item{
				ID:    "operator-state-task-archetype-native-apply",
				Class: "pipeline-architecture",
				Files: []string{"go/internal/phases/triage/", "go/internal/core/"},
			},
		},
		{
			name: "one source file among .evolve paths",
			item: inboxbatch.Item{
				ID:    "mixed",
				Class: "pipeline-architecture",
				Files: []string{".evolve/state.json", "go/internal/core/loop.go"},
			},
		},
		{
			name: "empty file list is not a state mutation",
			item: inboxbatch.Item{ID: "no-files", Class: "pipeline-architecture"},
		},
		{
			name: "non-pipeline class with .evolve files",
			item: inboxbatch.Item{
				ID:    "wrong-class",
				Class: "task-contract-design",
				Files: []string{".evolve/state.json"},
			},
		},
		{
			name: "no class declared",
			item: inboxbatch.Item{ID: "classless", Files: []string{".evolve/state.json"}},
		},
		{
			name: "lookalike path prefix",
			item: inboxbatch.Item{
				ID:    "lookalike",
				Class: "pipeline-architecture",
				Files: []string{".evolvex/state.json"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if inboxbatch.IsOperatorState(tc.item) {
				t.Errorf("IsOperatorState(%+v) = true, want false — %s must not route to native apply", tc.item, tc.name)
			}
		})
	}
}

const monotonicBinaryAC = "prune the inbox backlog to <=25 items"

func TestC1190_004_monotonic_binary_target_lint_fires(t *testing.T) {
	findings := evalgate.LintMonotonicBinaryTarget("task-contract-design", []string{
		monotonicBinaryAC,
		"the ship gate stays enforce",
	})
	if len(findings) != 1 {
		t.Fatalf("LintMonotonicBinaryTarget(task-contract-design, [binary-AC, unrelated-AC]) returned %d findings %v, want exactly 1 — the binary absolute target (%q) must flag and the unrelated AC must not", len(findings), findings, monotonicBinaryAC)
	}
	msg := findings[0]
	if !strings.Contains(msg, "direction+floor") {
		t.Errorf("finding = %q, want it to name the %q remedy — a lint that flags without saying what to write instead does not prevent the cycle-992 failure mode", msg, "direction+floor")
	}

	prose := evalgate.LintMonotonicBinaryTarget("task-contract-design", []string{"reduce the backlog to at most 25 items"})
	if len(prose) != 1 {
		t.Errorf("LintMonotonicBinaryTarget on prose absolute target returned %d findings %v, want 1 — prose phrasing of the same binary target must flag too", len(prose), prose)
	}
}

func TestC1190_005_monotonic_binary_target_lint_does_not_false_positive(t *testing.T) {
	cases := []struct {
		name     string
		class    string
		criteria []string
	}{
		{
			name:     "direction+floor delta phrasing on a monotonic class",
			class:    "task-contract-design",
			criteria: []string{"reduce the inbox backlog by >=50 items, landing whatever verified reduction is achieved and requeueing the remainder with the delta"},
		},
		{
			name:     "binary absolute target on a non-monotonic class",
			class:    "pipeline-architecture",
			criteria: []string{monotonicBinaryAC},
		},
		{
			name:     "binary absolute target with no class declared",
			class:    "",
			criteria: []string{monotonicBinaryAC},
		},
		{
			name:     "empty criteria on a monotonic class",
			class:    "task-contract-design",
			criteria: nil,
		},
		{
			name:     "non-count criterion on a monotonic class",
			class:    "task-contract-design",
			criteria: []string{"the ship gate stays enforce", "docs/operations/ gains a rationale section"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalgate.LintMonotonicBinaryTarget(tc.class, tc.criteria)
			if len(got) != 0 {
				t.Errorf("LintMonotonicBinaryTarget(%q, %v) = %v, want no findings — %s is a false positive that would block legitimate ACs", tc.class, tc.criteria, got, tc.name)
			}
		})
	}
}

func TestC1190_006_touched_packages_stay_green(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1",
		"./internal/inboxbatch/...", "./internal/evalgate/...",
	)
	if err != nil {
		t.Fatalf("could not run the touched-package suites: %v\nstderr:\n%s", err, stderr)
	}
	if code != 0 {
		t.Fatalf("touched-package suites exit=%d, want 0 — the new field/functions regressed existing behaviour\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, pkg := range []string{"internal/inboxbatch", "internal/evalgate"} {
		if !strings.Contains(stdout, pkg) {
			t.Errorf("no result line for %s in the suite output — the package did not run, so this proves nothing\nstdout:\n%s", pkg, stdout)
		}
	}
}
