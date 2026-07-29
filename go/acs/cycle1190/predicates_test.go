//go:build acs

// Package cycle1190 materialises the cycle-1190 acceptance criteria for the
// three triage-committed (## top_n) tasks:
//
//	inboxbatch-class-field-capture       — Item.Class captured from JSON "class"
//	inboxbatch-operator-state-detection  — IsOperatorState pure classifier
//	evalgate-monotonic-binary-target-lint — LintMonotonicBinaryTarget lint
//
// These are the scoped prerequisite slices of the three fleet-assigned inbox
// items; the full landing mechanisms are ## deferred and therefore carry ZERO
// predicates here (R9.3 floor-binding: predicates bind only to committed work).
//
// Predicate strategy — every predicate CALLS the system under test and asserts
// on its return value; none greps production source for a magic string (the
// cycle-85 degenerate-predicate ban):
//
//   - 001 runs the real inboxbatch.LoadDir over a temp inbox holding a verbatim
//     copy of a live item's JSON and asserts Class round-trips (plus the
//     absent-key zero value), so a field added without the `json:"class"` tag
//     still fails.
//   - 002/003 call IsOperatorState directly: 002 is the positive, 003 is the
//     NEGATIVE (source-touching path, empty file list, wrong class) — a
//     `return true` stub passes 002 and fails 003.
//   - 004/005 call LintMonotonicBinaryTarget: 004 asserts it fires with a
//     direction+floor-oriented message on the exact cycle-992 AC shape, 005 is
//     the NEGATIVE (compliant delta-floor phrasing, non-monotonic class, empty
//     input) — a `return []string{msg}` stub passes 004 and fails 005.
//   - 006 EXECUTES both touched packages' suites as a subprocess and asserts
//     exit 0 (regression pin: the new field/functions must not break callers).
//
// RED state at TDD time: this package does not compile — Item.Class,
// inboxbatch.IsOperatorState and evalgate.LintMonotonicBinaryTarget do not
// exist yet. That compile failure IS the RED evidence for 001-005; 006 is the
// only predicate that would pass on today's tree once compilation succeeds.
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

// liveItemJSON is a verbatim excerpt of the real fleet-scoped inbox item
// .evolve/inbox/2026-07-21T07-02-00Z-operator-state-task-archetype.json — the
// declared class is the value Item must now carry.
const liveItemJSON = `{
  "id": "operator-state-task-archetype-native-apply",
  "title": "New task archetype for live main-tree STATE mutations",
  "weight_hint": 0.93,
  "files": ["go/internal/phases/triage/", "go/internal/core/"],
  "class": "pipeline-architecture"
}`

// classlessItemJSON declares no "class" key — the tolerant-by-default contract
// (item.go doc comment) requires it to zero-value, never error.
const classlessItemJSON = `{
  "id": "no-class-declared",
  "title": "an item filed before the class vocabulary existed",
  "kind": "bug"
}`

// writeInbox materialises a temp inbox dir from name→content and returns it.
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

// loadOne loads dir and returns the single item with the given id.
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

// TestC1190_001_item_class_round_trips_from_inbox_json asserts Item captures the
// declared `class` key through the real loader.
//
// AC (task inboxbatch-class-field-capture): `Item.Class` exists, is JSON-tagged
// `class`, round-trips the exact string from a fixture inbox JSON; an absent
// key parses to "" without error or warning.
//
// Anti-gaming: this does not inspect item.go source — it runs LoadDir, so a
// field declared without the `json:"class"` tag (or named differently in the
// tag) fails on the value comparison. Edge case: the classless fixture pins the
// tolerant-by-default zero value AND that no warning is emitted for it.
func TestC1190_001_item_class_round_trips_from_inbox_json(t *testing.T) {
	dir := writeInbox(t, map[string]string{
		"2026-07-21T07-02-00Z-operator-state-task-archetype.json": liveItemJSON,
		"2026-01-01T00-00-00Z-no-class-declared.json":             classlessItemJSON,
	})

	got := loadOne(t, dir, "operator-state-task-archetype-native-apply")
	if got.Class != "pipeline-architecture" {
		t.Errorf("Item.Class = %q, want %q — the declared class is still being dropped at load", got.Class, "pipeline-architecture")
	}
	// The pre-existing fields must survive the struct change (a botched tag
	// edit that shifts other tags would otherwise go unnoticed).
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

// TestC1190_002_operator_state_detected_for_evolve_only_pipeline_item is the
// POSITIVE predicate for the classifier.
//
// AC (task inboxbatch-operator-state-detection): a pure detector
// `inboxbatch.IsOperatorState(Item) bool` returns true when the item's class is
// `pipeline-architecture` AND every declared file lives under `.evolve/`.
//
// It calls the function; there is no source inspection anywhere in this test.
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

// TestC1190_003_operator_state_rejects_source_touching_and_degenerate_items is
// the NEGATIVE predicate — the anti-no-op signal for task 2.
//
// AC (task inboxbatch-operator-state-detection): the detector returns false when
// ANY declared file is source-touching, when the file list is empty, and when
// the class is anything other than `pipeline-architecture`.
//
// The guard being encoded is the inbox item's own: "source-touching ops are
// NEVER accepted in an operator-state manifest". A `return true` stub passes 002
// and dies here; so does a class-only check (case "class matches but files are
// source"), which is exactly what the real fleet-scoped item looks like.
func TestC1190_003_operator_state_rejects_source_touching_and_degenerate_items(t *testing.T) {
	cases := []struct {
		name string
		item inboxbatch.Item
	}{
		{
			// The real fleet-scoped item: right class, source-code files.
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
			// Path-prefix trap: ".evolvex/" is NOT under ".evolve/".
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

// monotonicBinaryAC is the literal cycle-992 acceptance-criterion shape the lint
// exists to flag: a binary ABSOLUTE target on a monotonic convergence task.
const monotonicBinaryAC = "prune the inbox backlog to <=25 items"

// TestC1190_004_monotonic_binary_target_lint_fires is the POSITIVE predicate for
// the evalgate lint.
//
// AC (task evalgate-monotonic-binary-target-lint): `evalgate.LintMonotonicBinaryTarget(class string, criteria []string) []string`
// returns one finding per binary absolute-count criterion when the class is
// monotonic (`task-contract-design`, or any class carrying the monotonic tag),
// and each finding's message names the required `direction+floor` remedy.
//
// The message assertion is load-bearing: the lint's whole value is telling the
// author what to write instead (the inbox item's how_to_apply step 1).
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

	// "at most N" is the same defect in prose; the lint must not be a single
	// hard-coded "<=" match.
	prose := evalgate.LintMonotonicBinaryTarget("task-contract-design", []string{"reduce the backlog to at most 25 items"})
	if len(prose) != 1 {
		t.Errorf("LintMonotonicBinaryTarget on prose absolute target returned %d findings %v, want 1 — prose phrasing of the same binary target must flag too", len(prose), prose)
	}
}

// TestC1190_005_monotonic_binary_target_lint_does_not_false_positive is the
// NEGATIVE predicate — the anti-no-op signal for task 3.
//
// AC (task evalgate-monotonic-binary-target-lint): the lint returns no findings
// for already-compliant direction+floor phrasing, for non-monotonic classes
// (even with a binary target), and for empty input.
//
// Rationale: a false positive here blocks legitimate ACs on every non-monotonic
// task in the repo, so the false-positive cost dominates the false-negative
// cost. A `return []string{msg}` stub passes 004 and dies here.
func TestC1190_005_monotonic_binary_target_lint_does_not_false_positive(t *testing.T) {
	cases := []struct {
		name     string
		class    string
		criteria []string
	}{
		{
			// The remedy phrasing from the inbox item's how_to_apply step 1.
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

// TestC1190_006_touched_packages_stay_green EXECUTES the two touched packages'
// suites and asserts exit 0.
//
// AC (all three tasks): the struct-field addition and the two new functions must
// not regress existing inboxbatch/evalgate behaviour.
//
// This is a REGRESSION PIN, not a RED target — both suites are green in this
// worktree before any change (see test-report.md RED Run Output). `go -C` is
// used so the subprocess runs in the module dir regardless of the runner's cwd.
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
	// Guard against a vacuous pass: both packages must actually report.
	for _, pkg := range []string{"internal/inboxbatch", "internal/evalgate"} {
		if !strings.Contains(stdout, pkg) {
			t.Errorf("no result line for %s in the suite output — the package did not run, so this proves nothing\nstdout:\n%s", pkg, stdout)
		}
	}
}
