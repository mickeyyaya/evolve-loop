package core

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
		// Model a compliant agent: seed the real input artifact, derive the
		// digest as the assembler will, and copy it verbatim into the disposition.
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
