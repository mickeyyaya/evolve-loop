package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tracker must escalate ONLY on the threshold-th consecutive non-shipping
// cycle, reset on any shipping cycle, and carry the streak + distinct reasons.
func TestGoalStallTracker_Escalation(t *testing.T) {
	t.Run("threshold-th consecutive non-shipping cycle escalates", func(t *testing.T) {
		var tr goalStallTracker
		if esc := tr.observe(true, "SKIPPED_UNKNOWN", 3); esc != nil {
			t.Fatal("escalated on the 1st non-shipping cycle, want nil")
		}
		if esc := tr.observe(true, "SKIPPED_AUDIT_ADVISORY", 3); esc != nil {
			t.Fatal("escalated on the 2nd, want nil")
		}
		esc := tr.observe(true, "SKIPPED_UNKNOWN", 3)
		if esc == nil {
			t.Fatal("did NOT escalate on the 3rd consecutive non-shipping cycle")
		}
		if esc.streak != 3 {
			t.Errorf("streak = %d, want 3", esc.streak)
		}
		// SKIPPED_UNKNOWN seen twice → deduped to 2 distinct reasons.
		if len(esc.reasons) != 2 {
			t.Errorf("reasons = %v, want 2 distinct", esc.reasons)
		}
	})

	t.Run("a shipping cycle resets the streak (the counter only counts CONSECUTIVE)", func(t *testing.T) {
		var tr goalStallTracker
		tr.observe(true, "SKIPPED_UNKNOWN", 3)
		tr.observe(true, "SKIPPED_UNKNOWN", 3)
		if esc := tr.observe(false, "", 3); esc != nil { // shipped — reset
			t.Fatal("a shipping cycle escalated, want nil")
		}
		if esc := tr.observe(true, "SKIPPED_UNKNOWN", 3); esc != nil {
			t.Fatal("escalated at streak 1 after a reset")
		}
		if esc := tr.observe(true, "SKIPPED_UNKNOWN", 3); esc != nil {
			t.Fatal("escalated at streak 2 after a reset")
		}
	})

	t.Run("firing resets so the next escalation needs a fresh threshold", func(t *testing.T) {
		var tr goalStallTracker
		tr.observe(true, "x", 2)
		if tr.observe(true, "x", 2) == nil {
			t.Fatal("did not fire at threshold 2")
		}
		if tr.observe(true, "x", 2) != nil {
			t.Fatal("fired again immediately after reset — should need 2 fresh consecutive")
		}
	})
}

// The item must clamp weight up to the floor, pass validate, be idempotent by
// goal (stable id), and carry the block reasons for the scout.
func TestGoalStallItem_WriteIdempotentByGoal(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	esc := &goalStallEscalation{streak: 3, reasons: []string{"SKIPPED_UNKNOWN", "SKIPPED_AUDIT_ADVISORY"}}
	goalHash := "805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55"

	item := buildGoalStallItem(goalHash, esc, 0.5 /* below floor */, 644, "2026-07-18T00:00:00Z")
	if item.Weight < goalStallWeightFloor {
		t.Errorf("weight %v not clamped up to floor %v", item.Weight, goalStallWeightFloor)
	}
	if err := item.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	p1, err := item.writeTo(evolveDir)
	if err != nil {
		t.Fatalf("writeTo: %v", err)
	}
	p2, err := item.writeTo(evolveDir) // second fire for the SAME goal
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Errorf("re-fire wrote a different path %q vs %q — not idempotent by goal", p1, p2)
	}
	raw, _ := os.ReadFile(p1)
	var got goalStallItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != goalStallItemIDPrefix+"805f6ced" {
		t.Errorf("id = %q, want %q", got.ID, goalStallItemIDPrefix+"805f6ced")
	}
	if !strings.Contains(got.Description, "SKIPPED_AUDIT_ADVISORY") {
		t.Errorf("description missing the block reasons: %q", got.Description)
	}
}

// validate() must reject an under-weighted or field-missing item so a malformed
// self-injection fails loud (the "never a silent no-op todo" contract).
func TestGoalStallItem_ValidateRejectsMalformed(t *testing.T) {
	good := buildGoalStallItem("abcd1234ef", &goalStallEscalation{streak: 3}, 0.9, 1, "2026-07-18T00:00:00Z")
	if err := good.validate(); err != nil {
		t.Fatalf("a well-formed item must validate: %v", err)
	}
	under := good
	under.Weight = 0.5 // below the floor — must be rejected here even though buildGoalStallItem clamps
	if err := under.validate(); err == nil {
		t.Error("validate() accepted a weight below the floor — a silent under-weight would slip through")
	}
	missing := good
	missing.Source = "" // a required field
	if err := missing.validate(); err == nil {
		t.Error("validate() accepted an item missing a required field")
	}
}

// A second full threshold streak (after a reset) must re-escalate — the tracker
// re-arms, it does not fire only once per process.
func TestGoalStallTracker_ReArmsAfterFiring(t *testing.T) {
	var tr goalStallTracker
	if tr.observe(true, "x", 2) != nil || tr.observe(true, "x", 2) == nil {
		t.Fatal("first escalation did not fire at the threshold")
	}
	// after the reset, a fresh full streak must escalate AGAIN.
	if tr.observe(true, "x", 2) != nil {
		t.Fatal("re-fired at streak 1 after reset (should need a fresh threshold)")
	}
	if esc := tr.observe(true, "x", 2); esc == nil {
		t.Fatal("did NOT re-escalate on the second full streak — the tracker does not re-arm")
	} else if esc.streak != 2 {
		t.Errorf("re-escalation streak = %d, want 2", esc.streak)
	}
}

// handleGoalStall must both file the inbox todo AND emit the abnormal-event —
// the end-to-end escalation path (the feature's only integration point).
func TestHandleGoalStall_FilesInboxAndEmitsEvent(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	workspace := filepath.Join(root, "ws")
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	goalHash := "805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55"
	esc := &goalStallEscalation{streak: 3, reasons: []string{"SKIPPED_UNKNOWN"}}
	var stderr strings.Builder

	handleGoalStall(evolveDir, goalHash, workspace, 644, esc, 3, 0.9, &stderr)

	// inbox todo written at the goal-stable path
	inboxPath := filepath.Join(evolveDir, "inbox", goalStallItemIDPrefix+"805f6ced.json")
	if _, err := os.Stat(inboxPath); err != nil {
		t.Errorf("goal-stall inbox todo not written: %v", err)
	}
	// abnormal-event emitted naming the goal
	events, err := os.ReadFile(filepath.Join(workspace, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("no abnormal-events.jsonl emitted: %v", err)
	}
	if !strings.Contains(string(events), "goal-stall-escalated") || !strings.Contains(string(events), "805f6ced") {
		t.Errorf("abnormal-event missing goal-stall type or goal hash: %s", events)
	}
	if !strings.Contains(stderr.String(), "GOAL-STALL") {
		t.Errorf("no loud stderr line emitted: %q", stderr.String())
	}
}
