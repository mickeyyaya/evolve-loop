package gcpolicy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

// TestWithDefaults pins the retention defaults AND the deliberate non-default:
// the archive/delete day counts stay zero ("never"), because a retention engine
// must not invent a deletion horizon the operator never configured.
func TestWithDefaults(t *testing.T) {
	got := gcpolicy.Policy{}.WithDefaults()
	want := gcpolicy.Policy{
		Runs:           gcpolicy.RunsPolicy{KeepFull: 10},
		SalvageTTLDays: 30,
		LogsTTLDays:    30,
		TrackerTTLDays: 7,
	}
	if got != want {
		t.Errorf("zero Policy.WithDefaults() = %+v, want %+v", got, want)
	}
	if got.Runs.ArchiveAfterDays != 0 || got.Runs.DeleteAfterDays != 0 {
		t.Errorf("WithDefaults invented a retention horizon: archive=%d delete=%d, want 0/0",
			got.Runs.ArchiveAfterDays, got.Runs.DeleteAfterDays)
	}
}

// TestWithDefaultsPreservesExplicitValues proves defaults never overwrite an
// operator setting, and that WithDefaults is a value copy (the receiver is
// unchanged).
func TestWithDefaultsPreservesExplicitValues(t *testing.T) {
	in := gcpolicy.Policy{
		Mode:           "enforce",
		Runs:           gcpolicy.RunsPolicy{KeepFull: 3, ArchiveAfterDays: 14, DeleteAfterDays: 60},
		SalvageTTLDays: 1,
		LogsTTLDays:    2,
		TrackerTTLDays: 3,
		Worktrees:      gcpolicy.WorktreesPolicy{KeepRecent: 5, MinAgeMinutes: 90},
	}
	if got := in.WithDefaults(); got != in {
		t.Errorf("WithDefaults() overwrote explicit config: got %+v, want %+v", got, in)
	}
}

// TestPolicyJSONRoundTrip pins the wire contract: these tags are what operators
// write in .evolve/policy.json under "gc", and internal/policy embeds this
// struct directly, so a tag change is a silent config break.
func TestPolicyJSONRoundTrip(t *testing.T) {
	raw := `{"mode":"shadow","runs":{"keep_full":4,"archive_after_days":7,"delete_after_days":30},
	         "salvage_ttl_days":11,"logs_ttl_days":12,"tracker_ttl_days":13,
	         "worktrees":{"keep_recent":2,"min_age_minutes":45}}`
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := gcpolicy.Policy{
		Mode:           "shadow",
		Runs:           gcpolicy.RunsPolicy{KeepFull: 4, ArchiveAfterDays: 7, DeleteAfterDays: 30},
		SalvageTTLDays: 11,
		LogsTTLDays:    12,
		TrackerTTLDays: 13,
		Worktrees:      gcpolicy.WorktreesPolicy{KeepRecent: 2, MinAgeMinutes: 45},
	}
	if p != want {
		t.Fatalf("decoded %+v, want %+v", p, want)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back gcpolicy.Policy
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if back != want {
		t.Errorf("round trip lost data: %+v, want %+v", back, want)
	}
}
