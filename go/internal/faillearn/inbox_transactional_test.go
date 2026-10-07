package faillearn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
			ID:            "retro-1279-stale-active-worktree",
			Title:         "Reconcile cs.ActiveWorktree on fleet teardown",
			Weight:        0.96,
			Kind:          "bug",
			Priority:      "H",
			PriorityClass: "correctness",
			Files:         []string{"go/internal/core/fleet.go"},
			InjectedBy:    "retrofile",
		},
		{
			ID:            "retro-1279-symlink-test-suffix",
			Title:         "Resolve symlinks before the _test.go suffix check",
			Weight:        0.9,
			Kind:          "bug",
			Priority:      "H",
			PriorityClass: "correctness",
			Files:         []string{"go/internal/phases/audit/probe_quarantine.go"},
			InjectedBy:    "retrofile",
		},
	}
}

func TestWriteArtifacts_InboxItemsLandBesideRetrospective(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("WriteArtifacts with inbox items: %v", err)
	}

	if _, err := os.Stat(filepath.Join(runDir, "retrospective-report.md")); err != nil {
		t.Errorf("retrospective-report.md must still be written alongside inbox items: %v", err)
	}
	if n := countFilesWithSuffix(t, lessonsDir, ".yaml"); n != 1 {
		t.Errorf("want exactly 1 lesson YAML, got %d", n)
	}

	for _, want := range remediationItems() {
		path := filepath.Join(inboxDir, want.ID+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("remediation item %q must reach the inbox as %s: %v", want.ID, filepath.Base(path), err)
			continue
		}
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

func TestWriteArtifacts_InboxFailureLeavesNoRetrospective(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

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

func jsonEqual(got, want any) bool {
	if gf, ok := got.(float64); ok {
		if wf, ok := want.(float64); ok {
			return gf == wf
		}
	}
	return got == want
}
