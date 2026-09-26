package core

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Legacy key sets never change. A new persisted field must be omitempty and
// appended to its additive list, so an older file keeps its bytes.
var legacyStateKeys = []string{
	"lastUpdated", "lastCycleNumber", "version", "currentBatch",
	"failedApproaches", "carryoverTodos", "setupCompletedAt", "setupVersion",
}

var additiveStateKeys = []string{
	"triageThroughput",
	"stateRevision",
	"lastAllocatedCycleNumber",
}

var legacyLedgerEntryKeys = []string{
	"ts", "cycle", "cycle_label", "role", "kind", "model", "exit_code",
	"duration_s", "artifact_path", "artifact_sha256", "challenge_token",
	"git_head", "tree_state_sha", "worktree_tree_sha", "entry_seq",
	"prev_hash", "worker_count", "workers", "action", "message", "source",
}

var additiveLedgerEntryKeys = []string{
	"run_id",
	"task_id",
}

var legacyCycleStateKeys = []string{
	"cycle_id", "phase", "started_at", "phase_started_at", "active_agent",
	"active_worktree", "completed_phases", "workspace_path", "intent_required",
}

var additiveCycleStateKeys = []string{
	"run_id",
	"worktree_base_sha",
	"audit_fail_reasons",
	"failed_at",
	"ship_fail_reasons",
	"bookkeeping_regrade_attempted",
	"audit_repair_attempts",
	"audit_repair_active",
	"audit_dispatches",
	"explanation_documentation_version",
	"goal_hash",
	"goal_text",
	"pre_cycle_head",
	"final_verdict",
	"shipped",
	"ship_recovery_code",
	"audit_decline_reason",
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
