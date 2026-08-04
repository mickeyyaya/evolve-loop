package faillearn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// inbox_transactional_test.go — RED contract for cycle-1279 Task 3
// (`retro-inbox-transactional-write`, batch-integrity-review-2026-08-04.md F1
// solution bullet ii).
//
// The defect this pins: cycle-1255's retrospective filed two remediation items
// that "exist only inside .evolve/runs/cycle-1255/retrospective-report.md and
// never reached the inbox" — so the loop's own remediation queue never saw
// them and the defects were laundered away by later continuations. There is no
// mechanism today that writes retro-derived remediation items into
// `.evolve/inbox` in the SAME atomic call as the retrospective/lesson.
//
// API pinned by this contract (functional options — the repo idiom; the three
// existing WriteArtifacts callers stay byte-identical):
//
//	type InboxItem struct{ ID, Title, Kind, Priority, InjectedBy string; Weight float64; Files []string }
//	func WithInbox(dir string, items []InboxItem) Option
//	func WriteArtifacts(ev FailureEvent, runDir, lessonsDir string, opts ...Option) error
//
// InboxItem's JSON tags MUST match inboxbatch.Item's wire shape (id, title,
// weight, kind, priority, files, injected_by) — faillearn is a leaf package
// (stdlib + yaml.v3 only) so it cannot import inboxbatch; parity is by tag,
// asserted below on the raw JSON keys rather than by a Go type reference.

// remediationEvent is the shared fixture: a phase-scope failure carrying two
// real (non-degenerate) defects, the shape writeDeterministicLearning builds
// from a structured failure block.
func remediationEvent() FailureEvent {
	return FailureEvent{
		Cycle:          1279,
		FailedPhase:    "audit",
		Scope:          ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "audit rejected the deliverable",
		Defects: []string{
			"stale cs.ActiveWorktree survives fleet teardown",
			"symlinked test-suffix bypasses the probe quarantine",
		},
		EvidencePaths: []string{"/tmp/ws/audit-report.md"},
		Now:           time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
	}
}

func remediationItems() []InboxItem {
	return []InboxItem{
		{
			ID:         "retro-1279-stale-active-worktree",
			Title:      "Reconcile cs.ActiveWorktree on fleet teardown",
			Weight:     0.96,
			Kind:       "bug",
			Priority:   "H",
			Files:      []string{"go/internal/core/fleet.go"},
			InjectedBy: "retrofile",
		},
		{
			ID:         "retro-1279-symlink-test-suffix",
			Title:      "Resolve symlinks before the _test.go suffix check",
			Weight:     0.9,
			Kind:       "bug",
			Priority:   "H",
			Files:      []string{"go/internal/phases/audit/probe_quarantine.go"},
			InjectedBy: "retrofile",
		},
	}
}

// TestWriteArtifacts_InboxItemsLandBesideRetrospective is the primary
// behavioral criterion for F1(ii): supplying remediation items makes them
// reachable FROM THE INBOX, not only from the report body. It drives the real
// production entry point (WriteArtifacts) — the same function
// core.writeDeterministicLearning calls — and asserts on the emitted artifacts.
func TestWriteArtifacts_InboxItemsLandBesideRetrospective(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("WriteArtifacts with inbox items: %v", err)
	}

	// The pre-existing artifacts must still be written — the inbox write is an
	// ADDITION to the floor, never a replacement for it.
	if _, err := os.Stat(filepath.Join(runDir, "retrospective-report.md")); err != nil {
		t.Errorf("retrospective-report.md must still be written alongside inbox items: %v", err)
	}
	if n := countFilesWithSuffix(t, lessonsDir, ".yaml"); n != 1 {
		t.Errorf("want exactly 1 lesson YAML, got %d", n)
	}

	// Each remediation item must be an addressable inbox file, keyed by id.
	for _, want := range remediationItems() {
		path := filepath.Join(inboxDir, want.ID+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("remediation item %q must reach the inbox as %s: %v", want.ID, filepath.Base(path), err)
			continue
		}
		// Assert on the RAW JSON keys: inboxbatch.Item is the consumer and it
		// binds by wire tag, so a Go-side field rename that broke the tag would
		// silently produce items the loader drops (the cycle-1190 Class-field
		// shape of this same bug).
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Errorf("inbox item %s must be valid JSON: %v", want.ID, err)
			continue
		}
		for key, wantVal := range map[string]any{
			"id":          want.ID,
			"title":       want.Title,
			"weight":      want.Weight,
			"kind":        want.Kind,
			"priority":    want.Priority,
			"injected_by": want.InjectedBy,
		} {
			if got, ok := wire[key]; !ok {
				t.Errorf("inbox item %s: missing wire key %q (inboxbatch.Item parity)", want.ID, key)
			} else if !jsonEqual(got, wantVal) {
				t.Errorf("inbox item %s: %q = %v, want %v", want.ID, key, got, wantVal)
			}
		}
	}
}

// TestWriteArtifacts_InboxFailureLeavesNoRetrospective is the NEGATIVE /
// transactional criterion — the literal invariant F1(ii) names ("never only
// into the report"). With the inbox path unusable, WriteArtifacts must fail
// LOUDLY and must NOT have left the retrospective on disk claiming the
// remediation was recorded.
func TestWriteArtifacts_InboxFailureLeavesNoRetrospective(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	// A regular file where the inbox DIRECTORY must go: MkdirAll/create both
	// fail with ENOTDIR, the cheapest deterministic injection of an inbox-write
	// failure (no fault-injection seam needed, no permissions games that a root
	// CI runner would defeat).
	blocked := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("prepare blocked inbox path: %v", err)
	}

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blocked, remediationItems()))
	if err == nil {
		t.Fatal("WriteArtifacts must return an error when the inbox items cannot be written — a silent success is exactly the laundering this closes")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("transactional invariant violated: retrospective-report.md was written while the remediation items failed to reach the inbox")
	}
}

// TestWriteArtifacts_WithoutInboxOptionIsUnchanged is the back-compat
// criterion: the three existing production callers
// (core/failure_learning.go:442, core/reset.go:248,
// cmd/evolve/cmd_loop_outcome.go:447) pass no options and must observe exactly
// today's behavior — report + lesson, nothing else, no inbox directory minted.
func TestWriteArtifacts_WithoutInboxOptionIsUnchanged(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts without options: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "retrospective-report.md")); err != nil {
		t.Errorf("retrospective-report.md: %v", err)
	}
	if n := countFilesWithSuffix(t, lessonsDir, ".yaml"); n != 1 {
		t.Errorf("want exactly 1 lesson YAML, got %d", n)
	}
	if n := countFilesWithSuffix(t, runDir, ".json"); n != 0 {
		t.Errorf("no-option call must not mint inbox JSON into the run dir, got %d", n)
	}
}

// TestWriteArtifacts_EmptyInboxItemsMintsNoFiles is the edge criterion: a retro
// with zero remediation items supplies the option but no work — it must not
// create an empty inbox directory of noise, and must not error.
func TestWriteArtifacts_EmptyInboxItemsMintsNoFiles(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, nil)); err != nil {
		t.Fatalf("WriteArtifacts with zero inbox items must succeed: %v", err)
	}
	if n := countFilesWithSuffix(t, inboxDir, ".json"); n != 0 {
		t.Errorf("zero remediation items must mint zero inbox files, got %d", n)
	}
}

func countFilesWithSuffix(t *testing.T, dir, suffix string) int {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read %s: %v", dir, err)
	}
	n := 0
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == suffix {
			n++
		}
	}
	return n
}

// jsonEqual compares a decoded JSON value against a Go literal, normalizing the
// float64-vs-int asymmetry encoding/json introduces for numbers.
func jsonEqual(got, want any) bool {
	if gf, ok := got.(float64); ok {
		if wf, ok := want.(float64); ok {
			return gf == wf
		}
	}
	return got == want
}
