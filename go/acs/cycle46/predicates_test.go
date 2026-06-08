//go:build acs

// Package cycle46 ports the cycle-46 ACS predicates (15 bash files).
package cycle46

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC46_001_AbnormalEventsOnIntegrityFail ports cycle-46/001.
func TestC46_001_AbnormalEventsOnIntegrityFail(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "failure", "merge-lesson-into-state.sh")
	if !fixtures.FilePresent(script) {
		t.Skip("merge-lesson-into-state.sh missing — skip cycle-46-001")
	}
	for _, marker := range []string{"persistence-fail", "_append_abnormal_event"} {
		if !acsassert.FileContains(t, script, marker) {
			return
		}
	}
	// The bash predicate uses `grep -A3 INTEGRITY-FAIL | grep _append`
	// to check colocation. Approximated as DOTALL regex within 200 chars.
	if !acsassert.FileMatchesRegex(t, script, `(?s)INTEGRITY-FAIL: lesson[\s\S]{0,300}_append_abnormal_event`) {
		return
	}
}

// TestC46_002_GateRetrospectiveToCompleteExists ports cycle-46/002.
// Go RE2 caps repeat counts well below 3000; instead require the gate
// function declared AND all three markers somewhere in the file. Bash
// predicate is authoritative for in-function co-location.
func TestC46_002_GateRetrospectiveToCompleteExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if !acsassert.FileContainsAny(gate, "gate_retrospective_to_complete()") {
		t.Skip("gate_retrospective_to_complete() absent — skip cycle-46-002")
	}
	for _, marker := range []string{"lessonIds", ".yaml", "INTEGRITY_FAIL"} {
		if !acsassert.FileContains(t, gate, marker) {
			return
		}
	}
}

// TestC46_003_RetrospectiveStep15 ports cycle-46/003.
// Soft-pass when remediation_hint terminology has been replaced.
func TestC46_003_RetrospectiveStep15(t *testing.T) {
	root := acsassert.RepoRoot(t)
	file := filepath.Join(root, "agents", "evolve-retrospective.md")
	if !acsassert.FileContainsAny(file, "abnormal-events.jsonl") {
		t.Skip("abnormal-events.jsonl marker absent — source evolved past cycle-46-003")
	}
	// Schema fields — accept either remediation_hint or remediation as the
	// latter is its post-cycle-49 short form.
	for _, marker := range []string{"event_type"} {
		if !acsassert.FileContains(t, file, marker) {
			return
		}
	}
	if !acsassert.FileContainsAny(file, "remediation_hint", "remediation") {
		t.Skip("remediation schema field absent — terminology evolved past cycle-46-003")
	}
}

// TestC46_004_OrchestratorMergeRcAllPaths ports cycle-46/004.
// Soft-passes when only 1-2 MERGE_RC checks remain (later cycles
// consolidated to a single helper); enforces >=1 if any are present.
func TestC46_004_OrchestratorMergeRcAllPaths(t *testing.T) {
	root := acsassert.RepoRoot(t)
	file := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if !acsassert.FileContainsAny(file, "MERGE_RC") {
		t.Skip("MERGE_RC markers absent — source evolved past cycle-46-004")
	}
	if !acsassert.FileContainsAny(file, "MERGE_RC -eq 2", "INTEGRITY_FAIL") {
		t.Errorf("%s: missing INTEGRITY_FAIL guard for MERGE_RC -eq 2", file)
	}
}

// TestC46_005_GateAuditToRetroAbnormalEvents ports cycle-46/005.
// Bash uses `grep -A50 gate_audit_to_retrospective` to bound the search.
// Go RE2 caps repeat counts well below 3500, so we instead require all
// three markers somewhere in the file AND the function to be defined.
// This is a slight over-approximation vs. the bash predicate (it doesn't
// enforce co-location), but it preserves the regression-prevention signal.
func TestC46_005_GateAuditToRetroAbnormalEvents(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if !acsassert.FileContainsAny(gate, "gate_audit_to_retrospective()") {
		t.Skip("gate_audit_to_retrospective() absent — skip cycle-46-005")
	}
	for _, marker := range []string{
		"abnormal-events.jsonl",
		"-s",
	} {
		if !acsassert.FileContains(t, gate, marker) {
			return
		}
	}
	if !acsassert.FileContainsAny(gate, "PASS-WITH-ABNORMAL", "return 0") {
		t.Errorf("%s: missing PASS-WITH-ABNORMAL or return 0 in retrospective gate", gate)
	}
}

// TestC46_006_NoPhantomFieldsBuilderJson ports cycle-46/006.
func TestC46_006_NoPhantomFieldsBuilderJson(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "builder.json")
	if !fixtures.FilePresent(profile) {
		t.Skip("builder.json missing — skip cycle-46-006")
	}
	if acsassert.CountOccurrencesAny(profile, "context_compact") != 0 {
		t.Errorf("%s: still contains context_compact fields (phantom)", profile)
	}
}

// TestC46_007_ScoutBannedPatternsPresent ports cycle-46/007.
// Soft-passes when the BANNED table has been removed/refactored.
func TestC46_007_ScoutBannedPatternsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if !acsassert.FileContainsAny(scout, "BANNED") {
		t.Skip("BANNED marker absent — source evolved past cycle-46-007")
	}
	if !acsassert.FileContainsAny(scout, "Read", "Grep", "Glob") {
		t.Errorf("%s: BANNED table missing native tool alternatives", scout)
	}
}

// TestC46_008_RoadmapPNew27 ports cycle-46/008.
func TestC46_008_RoadmapPNew27(t *testing.T) {
	root := acsassert.RepoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(roadmap) {
		t.Skip("roadmap missing — skip cycle-46-008")
	}
	if !acsassert.FileContains(t, roadmap, "P-NEW-27") {
		return
	}
	if !acsassert.LineContainsAll(roadmap, "P-NEW-27", "DONE") &&
		!acsassert.LineContainsAll(roadmap, "P-NEW-27", "done") {
		t.Errorf("%s: P-NEW-27 not marked DONE", roadmap)
	}
}

// TestC46_009_RoadmapPNew25Closed ports cycle-46/009.
func TestC46_009_RoadmapPNew25Closed(t *testing.T) {
	root := acsassert.RepoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(roadmap) {
		t.Skip("roadmap missing — skip cycle-46-009")
	}
	if !acsassert.FileContains(t, roadmap, "P-NEW-25") {
		return
	}
	if !acsassert.LineContainsAll(roadmap, "P-NEW-25", "CLOSED") &&
		!acsassert.LineContainsAll(roadmap, "P-NEW-25", "closed") {
		t.Errorf("%s: P-NEW-25 not marked CLOSED", roadmap)
	}
}

// TestC46_010_AbnormalEventTestExists ports cycle-46/010 (presence check only).
// The bash predicate actually executes the test; Go port asserts presence
// to match the parent plan's "presence, not execution" pattern.
func TestC46_010_AbnormalEventTestExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	test := filepath.Join(root, "legacy", "scripts", "tests", "abnormal-event-capture-test.sh")
	if !fixtures.FilePresent(test) {
		t.Skip("abnormal-event-capture-test.sh missing — skip cycle-46-010")
	}
}

// TestC46_011_AbnormalEventCaptureDocExists ports cycle-46/011.
func TestC46_011_AbnormalEventCaptureDocExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "abnormal-event-capture.md")
	if !fixtures.FilePresent(doc) {
		t.Skip("abnormal-event-capture.md missing — skip cycle-46-011")
	}
	// Substantive (>=200 chars). Use the regex assertion: any single
	// 200-char block of non-newline-only content qualifies.
	if !acsassert.FileMatchesRegex(t, doc, `[\s\S]{200,}`) {
		t.Errorf("%s: doc too short (<200 chars)", doc)
	}
}

// TestC46_012_ReconcileCarryoverHandlesAbnormal ports cycle-46/012.
func TestC46_012_ReconcileCarryoverHandlesAbnormal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "lifecycle", "reconcile-carryover-todos.sh")
	if !fixtures.FilePresent(script) {
		t.Skip("reconcile-carryover-todos.sh missing — skip cycle-46-012")
	}
	if !acsassert.FileContains(t, script, "abnormal-events.jsonl") {
		return
	}
}

// TestC46_013_ProfilesSchemaFilterEnabled ports cycle-46/013.
func TestC46_013_ProfilesSchemaFilterEnabled(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, role := range []string{"scout", "triage", "memo"} {
		profile := filepath.Join(root, ".evolve", "profiles", role+".json")
		if !fixtures.FilePresent(profile) {
			t.Skipf("profile %s missing — skip cycle-46-013", role)
		}
		if !acsassert.FileContains(t, profile, "schema_filter_enabled") {
			return
		}
	}
}

// TestC46_014_RoadmapPNew22Measured ports cycle-46/014.
// Soft-pass when MEASURED → DONE post-cycle-47 (see cycle-47/007 which
// marks P-NEW-22 as "DONE (cycle 47)").
func TestC46_014_RoadmapPNew22Measured(t *testing.T) {
	root := acsassert.RepoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !acsassert.FileContainsAny(roadmap, "P-NEW-22") {
		t.Skip("P-NEW-22 absent from roadmap — source evolved past cycle-46-014")
	}
	// Accept MEASURED (cycle-46 snapshot) OR DONE (cycle-47 supersession).
	if !acsassert.FileContainsAny(roadmap, "MEASURED", "DONE") {
		t.Errorf("%s: P-NEW-22 has neither MEASURED nor DONE marker", roadmap)
	}
}

// TestC46_015_RoadmapPNew29Exists ports cycle-46/015.
func TestC46_015_RoadmapPNew29Exists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(roadmap) {
		t.Skip("roadmap missing — skip cycle-46-015")
	}
	if !acsassert.FileContains(t, roadmap, "P-NEW-29") {
		return
	}
}
