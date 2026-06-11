package core

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// bytestability_test.go — CA.6 (concurrency-factory plan, Track C-A): the
// single-mode byte-stability golden. Track C may only ADD omitempty fields
// to the shared persisted shapes — never rename, retype, or de-omit. This
// pin freezes each struct's JSON key set: a new field fails the test until
// it is consciously appended to the additive allowlist below, and a
// removed/renamed legacy key fails immediately. (The runtime half of CA.6 —
// the 10-cycle soak with byte-compared state/ledger — runs as a batch.)

// legacyStateKeys is the pre-Track-C state.json surface (frozen 2026-06-11).
var legacyStateKeys = []string{
	"lastUpdated", "lastCycleNumber", "version", "currentBatch",
	"failedApproaches", "carryoverTodos", "setupCompletedAt", "setupVersion",
}

// additiveStateKeys is every key Track C added — omitempty, additive-only.
var additiveStateKeys = []string{
	"triageThroughput",         // R9.1
	"stateRevision",            // CA.3
	"lastAllocatedCycleNumber", // CA.4
}

// legacyLedgerEntryKeys is the pre-Track-C ledger line surface.
var legacyLedgerEntryKeys = []string{
	"ts", "cycle", "cycle_label", "role", "kind", "model", "exit_code",
	"duration_s", "artifact_path", "artifact_sha256", "challenge_token",
	"git_head", "tree_state_sha", "worktree_tree_sha", "entry_seq",
	"prev_hash", "worker_count", "workers", "action", "message", "source",
}

var additiveLedgerEntryKeys = []string{
	"run_id", // CA.2
}

// legacyCycleStateKeys is the pre-Track-C cycle-state.json surface.
var legacyCycleStateKeys = []string{
	"cycle_id", "phase", "started_at", "phase_started_at", "active_agent",
	"active_worktree", "completed_phases", "workspace_path", "intent_required",
}

var additiveCycleStateKeys = []string{
	"run_id",            // CA.5
	"worktree_base_sha", // cycle-156 resume parity
}

func jsonKeysOf(t *testing.T, v any) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem() // NumField on a Ptr kind panics
	}
	keys := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s.%s has no json tag — every persisted field must be tagged", rt.Name(), rt.Field(i).Name)
		}
		name, _, _ := strings.Cut(tag, ",")
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func assertGoldenKeys(t *testing.T, name string, got, legacy, additive []string) {
	t.Helper()
	want := append(append([]string{}, legacy...), additive...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s JSON surface drifted.\n got: %v\nwant: %v\nNew fields must be appended to the additive allowlist in bytestability_test.go (omitempty, additive-only — CA.6); legacy keys must never change.", name, got, want)
	}
}

func TestByteStability_StateKeysAdditiveOnly(t *testing.T) {
	assertGoldenKeys(t, "core.State", jsonKeysOf(t, State{}), legacyStateKeys, additiveStateKeys)
}

func TestByteStability_LedgerEntryKeysAdditiveOnly(t *testing.T) {
	assertGoldenKeys(t, "core.LedgerEntry", jsonKeysOf(t, LedgerEntry{}), legacyLedgerEntryKeys, additiveLedgerEntryKeys)
}

func TestByteStability_CycleStateKeysAdditiveOnly(t *testing.T) {
	assertGoldenKeys(t, "core.CycleState", jsonKeysOf(t, CycleState{}), legacyCycleStateKeys, additiveCycleStateKeys)
}

// TestByteStability_AdditiveFieldsAreOmitempty — an additive field that is
// NOT omitempty changes every pre-Track-C file on its next write. Verify
// every allowlisted additive key carries omitempty on its struct tag.
func TestByteStability_AdditiveFieldsAreOmitempty(t *testing.T) {
	check := func(v any, additive []string) {
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			tag := rt.Field(i).Tag.Get("json")
			name, opts, _ := strings.Cut(tag, ",")
			for _, a := range additive {
				if name == a && !strings.Contains(opts, "omitempty") {
					t.Errorf("%s.%s (%s) is additive but not omitempty — breaks single-mode byte-stability", rt.Name(), rt.Field(i).Name, name)
				}
			}
		}
	}
	check(State{}, additiveStateKeys)
	check(LedgerEntry{}, additiveLedgerEntryKeys)
	check(CycleState{}, additiveCycleStateKeys)
}
