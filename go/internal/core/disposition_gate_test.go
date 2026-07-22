package core

// disposition_gate_test.go — RED contract for the S2 disposition-contract-gate
// (cycle-1034, item failure-disposition-router slice S2).
//
// The retro phase gains a MANDATORY disposition.json deliverable. VerifyDisposition
// is the fail-HARD counterpart to readFailureDecision's fail-SOFT boundary: a
// required deliverable, so absence/invalidity is a LOUD error (retro cannot
// complete), not a silent (nil,nil) fallback. It also cross-checks the
// disposition's fingerprint+recurrence against the S1 failure-digest.json so the
// agent cannot INVENT a failure identity in retro.
//
// disposition.json schema: {cycle, fingerprint, recurrence, legitimacy,
// root_cause:{layer,summary}, salvage:{worktree_has_value,pointer}, urgency,
// justification, routing, proposed_item}.
//
// RED today: VerifyDisposition and (*Orchestrator).finalizeRetroCompletion do
// not exist → this file fails to COMPILE (correct RED for new surface).

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validDisposition returns a schema-valid disposition whose fingerprint and
// recurrence match writeMatchingDigest below. Callers mutate one field to drive
// a specific rejection case.
func validDisposition() map[string]any {
	return map[string]any{
		"cycle":       1034,
		"fingerprint": "audit|gate-block|egps",
		"recurrence":  0,
		"legitimacy":  "legit-rejection",
		"root_cause": map[string]any{
			"layer":   "task-code",
			"summary": "acceptance predicate genuinely red",
		},
		"salvage":       map[string]any{"worktree_has_value": false, "pointer": ""},
		"urgency":       "P1",
		"justification": "the acs predicate failed for a real missing behavior",
		"routing":       "inbox",
		"proposed_item": "add-missing-behavior",
	}
}

func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// writeMatchingDigest writes a failure-digest.json whose fingerprint+recurrence
// match validDisposition() — the "identity agrees" baseline.
func writeMatchingDigest(t *testing.T, dir string) {
	t.Helper()
	writeJSON(t, dir, "failure-digest.json", map[string]any{
		"cycle":       1034,
		"fingerprint": "audit|gate-block|egps",
		"recurrence":  0,
		"pre_class":   "gate-block",
	})
}

// AC1 — retro fails LOUD without a valid disposition. Absent disposition.json →
// loud error; a syntactically broken disposition.json → loud error. Contrast
// readFailureDecision, which returns (nil,nil) on the same inputs: this gate is
// fail-HARD because the disposition is a required deliverable.
func TestDispositionGate_RetroFailsLoudWithoutValidDisposition(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir) // digest present, disposition absent
		if err := VerifyDisposition(dir); err == nil {
			t.Fatal("absent disposition.json must return a loud error (fail-HARD), got nil")
		}
	})
	t.Run("malformed", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "disposition.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := VerifyDisposition(dir); err == nil {
			t.Fatal("malformed disposition.json must return a loud error, got nil")
		}
	})
	t.Run("valid_passes", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		writeJSON(t, dir, "disposition.json", validDisposition())
		if err := VerifyDisposition(dir); err != nil {
			t.Fatalf("a fully valid disposition must pass, got: %v", err)
		}
	})
}

// AC2 — fingerprint is cross-checked against the digest (anti-invention). A
// disposition whose fingerprint disagrees with failure-digest.json is rejected;
// a matching fingerprint (and recurrence) passes. This is what stops the agent
// from inventing a failure identity that no assembler ever computed.
func TestDispositionGate_CrossChecksFingerprintAgainstDigest(t *testing.T) {
	t.Run("mismatch_rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		d := validDisposition()
		d["fingerprint"] = "build|guard-abort|statemap" // invented, != digest
		writeJSON(t, dir, "disposition.json", d)
		err := VerifyDisposition(dir)
		if err == nil {
			t.Fatal("fingerprint mismatch must be rejected (agent cannot invent identity)")
		}
		if !strings.Contains(err.Error(), "fingerprint") {
			t.Errorf("error should name the fingerprint mismatch, got: %v", err)
		}
	})
	t.Run("recurrence_mismatch_rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir) // recurrence 0
		d := validDisposition()
		d["recurrence"] = 7 // disagrees with the digest's ledger-derived count
		writeJSON(t, dir, "disposition.json", d)
		if err := VerifyDisposition(dir); err == nil {
			t.Fatal("recurrence disagreeing with the digest must be rejected")
		}
	})
	t.Run("match_passes", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		writeJSON(t, dir, "disposition.json", validDisposition())
		if err := VerifyDisposition(dir); err != nil {
			t.Fatalf("matching fingerprint+recurrence must pass, got: %v", err)
		}
	})
}

// AC3 (negative) — out-of-vocabulary enum values are rejected, with the
// offending field named. Table-driven over every enum field. The gaming fake — a
// gate that only checks the JSON parses — fails here because each of these
// documents parses cleanly yet carries an illegal enum value.
func TestDispositionGate_RejectsInvalidEnums(t *testing.T) {
	cases := []struct {
		field string
		set   func(d map[string]any)
	}{
		{"legitimacy", func(d map[string]any) { d["legitimacy"] = "totally-legit" }},
		{"layer", func(d map[string]any) { d["root_cause"].(map[string]any)["layer"] = "cosmic-rays" }},
		{"urgency", func(d map[string]any) { d["urgency"] = "P9" }},
		{"routing", func(d map[string]any) { d["routing"] = "carrier-pigeon" }},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			dir := t.TempDir()
			writeMatchingDigest(t, dir)
			d := validDisposition()
			tc.set(d)
			writeJSON(t, dir, "disposition.json", d)
			err := VerifyDisposition(dir)
			if err == nil {
				t.Fatalf("out-of-vocabulary %s must be rejected", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("rejection must name the offending field %q, got: %v", tc.field, err)
			}
		})
	}
}

// AC4 (edge) — salvage floor: worktree_has_value=true REQUIRES a non-empty
// pointer (cycles 984/1000 salvage precedent — preserved worktree value must be
// pointed at, never silently dropped). worktree_has_value=false with an empty
// pointer is accepted (nothing to salvage).
func TestDispositionGate_SalvagePointerRequiredWhenValue(t *testing.T) {
	t.Run("value_without_pointer_rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		d := validDisposition()
		d["salvage"] = map[string]any{"worktree_has_value": true, "pointer": ""}
		writeJSON(t, dir, "disposition.json", d)
		err := VerifyDisposition(dir)
		if err == nil {
			t.Fatal("worktree_has_value=true with an empty pointer must be rejected (salvage floor)")
		}
		if !strings.Contains(err.Error(), "salvage") && !strings.Contains(err.Error(), "pointer") {
			t.Errorf("error should reference the salvage/pointer floor, got: %v", err)
		}
	})
	t.Run("no_value_no_pointer_accepted", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		d := validDisposition()
		d["salvage"] = map[string]any{"worktree_has_value": false, "pointer": ""}
		writeJSON(t, dir, "disposition.json", d)
		if err := VerifyDisposition(dir); err != nil {
			t.Fatalf("no value + no pointer must be accepted, got: %v", err)
		}
	})
}

// AC5 (wiring) — the gate is invoked on the COMPOSED retro-completion path, not
// merely unit-testable in isolation (unit-green != live-green). This drives the
// real orchestrator seam recordFailureLearning uses: a live-shaped FAIL cycle
// whose retro runner returns PASS must still route through the disposition gate.
//   - missing disposition → the completion path surfaces the loud gate error
//     (recorded in Result.RetroDecision) and does NOT silently record a clean
//     retro outcome.
//   - valid disposition + matching digest → the gate passes and the normal
//     failure-learning outcome is recorded.
//
// t.TempDir() only — never mutates the live repo tree.
func TestDispositionGate_WiredIntoRetroCompletion(t *testing.T) {
	newFL := func(dir string) failureLearningRequest {
		return failureLearningRequest{
			CycleRequest: CycleRequest{ProjectRoot: dir},
			Cycle:        1034,
			Failed:       PhaseAudit,
			Err:          errors.New("audit floor red"),
			State:        &State{},
			CycleState:   &CycleState{CycleID: 1034, WorkspacePath: dir},
			Result:       &CycleResult{},
			Timings:      &[]phaseTimingEntry{},
			Context:      map[string]string{},
			Env:          map[string]string{},
		}
	}

	t.Run("missing_disposition_surfaces_loud_error", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir) // digest present, disposition absent
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
		fl := newFL(dir)
		o.recordFailureLearning(context.Background(), fl)
		if !strings.Contains(fl.Result.RetroDecision, "disposition") {
			t.Errorf("composed retro completion must surface the disposition-gate error; "+
				"RetroDecision = %q (gate not wired into recordFailureLearning?)", fl.Result.RetroDecision)
		}
	})

	t.Run("valid_disposition_completes", func(t *testing.T) {
		dir := t.TempDir()
		// The composed path now runs the S1 assembler pre-retro, which is
		// AUTHORITATIVE over any pre-seeded digest (anti-invention). Model a
		// compliant agent: seed the real input artifact, derive the canonical
		// digest exactly as the assembler will, and copy fingerprint/recurrence
		// verbatim into the disposition.
		writeJSON(t, dir, "audit-fail-reason.json", map[string]any{
			"schema_version": 1, "phase": "audit",
			"reasons": []string{"EGPS: red_count=1 (cycle ships only when red_count==0)"},
		})
		digest, err := AssembleFailureDigest(1034, dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := validDisposition()
		d["fingerprint"] = digest.Fingerprint
		d["recurrence"] = digest.Recurrence
		writeJSON(t, dir, "disposition.json", d)
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
		fl := newFL(dir)
		o.recordFailureLearning(context.Background(), fl)
		if strings.Contains(fl.Result.RetroDecision, "disposition-gate") {
			t.Errorf("a valid disposition must NOT surface a gate error; RetroDecision = %q", fl.Result.RetroDecision)
		}
	})

	t.Run("assembler_overwrites_foreign_digest_on_composed_path", func(t *testing.T) {
		// Regression pin for the I2 wiring (assembler was landed callerless by
		// cycle-1034): a pre-seeded foreign digest is REPLACED by the assembler
		// pre-retro, so a disposition copying the foreign identity fails the
		// cross-check — the digest on disk after completion is the assembler's.
		dir := t.TempDir()
		writeMatchingDigest(t, dir) // foreign fixture digest
		writeJSON(t, dir, "disposition.json", validDisposition())
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
		fl := newFL(dir)
		o.recordFailureLearning(context.Background(), fl)
		if !strings.Contains(fl.Result.RetroDecision, "disposition-gate") {
			t.Errorf("foreign digest identity must be rejected once the assembler is authoritative; RetroDecision = %q", fl.Result.RetroDecision)
		}
	})
}

// TestFinalizeRetroCompletion_SeamContract pins the direct completion seam the
// orchestrator wires (the function under the AC5 wiring). Kept distinct from the
// orchestrator-level test so a Builder implementing the gate has an isolated
// target: valid → nil, missing → loud error.
func TestFinalizeRetroCompletion_SeamContract(t *testing.T) {
	o := &Orchestrator{}

	missing := t.TempDir()
	writeMatchingDigest(t, missing)
	if err := o.finalizeRetroCompletion(missing); err == nil {
		t.Fatal("finalizeRetroCompletion must return a loud error when disposition.json is absent")
	}

	ok := t.TempDir()
	writeMatchingDigest(t, ok)
	writeJSON(t, ok, "disposition.json", validDisposition())
	if err := o.finalizeRetroCompletion(ok); err != nil {
		t.Fatalf("finalizeRetroCompletion must pass with a valid disposition, got: %v", err)
	}
}
